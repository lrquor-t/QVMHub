package security

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"qvmhub/utils"
)

// ============================================================
// 泄露密码检测服务
// 基于 Have I Been Pwned (HIBP) Pwned Passwords API
// 采用 k-匿名性模型：仅发送 SHA-1 哈希前 5 位，密码本身永不离开本机
// 离线时回退到内置常见弱密码列表
// ============================================================

const (
	hibpAPIURL       = "https://api.pwnedpasswords.com/range/"
	hibpTimeout      = 5 * time.Second
	hibpCacheTTL     = 30 * time.Minute // 哈希前缀 → 后缀列表的缓存有效期
	hibpCacheCleanup = 1 * time.Hour    // 缓存清理间隔
)

// hibpCacheEntry 缓存 HIBP API 返回的哈希后缀集合
type hibpCacheEntry struct {
	suffixes  map[string]struct{} // 后缀集合（大写，不含次数）
	expiresAt time.Time
}

var (
	hibpCache     = make(map[string]*hibpCacheEntry)
	hibpCacheMu   sync.RWMutex
	hibpClient    *http.Client
	hibpClientOnce sync.Once
)

// getHIBPClient 单例 HTTP 客户端
func getHIBPClient() *http.Client {
	hibpClientOnce.Do(func() {
		hibpClient = &http.Client{
			Timeout: hibpTimeout,
		}
	})
	return hibpClient
}

// init 启动缓存清理协程
func init() {
	go func() {
		defer utils.RecoverAndLog("hibp-cache-cleanup")
		ticker := time.NewTicker(hibpCacheCleanup)
		defer ticker.Stop()
		for range ticker.C {
			hibpCacheMu.Lock()
			now := time.Now()
			for prefix, entry := range hibpCache {
				if now.After(entry.expiresAt) {
					delete(hibpCache, prefix)
				}
			}
			hibpCacheMu.Unlock()
		}
	}()
}

// CheckPasswordBreached 检查密码是否在已知泄露数据库中
// 返回 (是否泄露, 是否使用了离线兜底, 错误)
func CheckPasswordBreached(password string) (breached bool, fallback bool, err error) {
	// 先检查本地常见密码列表
	if isCommonPassword(password) {
		return true, true, nil
	}

	// 通过 HIBP API 检查
	breached, err = checkHIBP(password)
	if err != nil {
		// API 不可用时不阻止操作，仅记录错误
		// 调用方可以根据 fallback 判断是否仅使用了本地兜底
		return false, false, err
	}
	return breached, false, nil
}

// checkHIBP 使用 HIBP API 的 k-匿名性模型检查密码
// 仅发送 SHA-1 哈希前 5 位，后缀在本地比对
func checkHIBP(password string) (bool, error) {
	hash := sha1.Sum([]byte(password))
	fullHash := strings.ToUpper(hex.EncodeToString(hash[:]))
	prefix := fullHash[:5]
	suffix := fullHash[5:]

	// 查缓存
	hibpCacheMu.RLock()
	entry, ok := hibpCache[prefix]
	hibpCacheMu.RUnlock()

	if ok && time.Now().Before(entry.expiresAt) {
		_, found := entry.suffixes[suffix]
		return found, nil
	}

	// 调用 HIBP API
	client := getHIBPClient()
	url := hibpAPIURL + prefix
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("HIBP 请求构建失败: %w", err)
	}
	// 添加 User-Agent（HIBP 建议但不强制）
	req.Header.Set("User-Agent", "QVMConsole-PasswordCheck")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("HIBP API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HIBP API 返回状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("HIBP 响应读取失败: %w", err)
	}

	// 解析响应：每行格式为 "HASH_SUFFIX:COUNT"
	suffixes := make(map[string]struct{})
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 1 {
			continue
		}
		s := strings.TrimSpace(parts[0])
		if s != "" {
			suffixes[s] = struct{}{}
		}
	}

	// 写入缓存
	hibpCacheMu.Lock()
	hibpCache[prefix] = &hibpCacheEntry{
		suffixes:  suffixes,
		expiresAt: time.Now().Add(hibpCacheTTL),
	}
	hibpCacheMu.Unlock()

	_, found := suffixes[suffix]
	return found, nil
}

// IsPasswordBreached 简化版：仅返回是否泄露（API 不可用时回退到本地列表）
func IsPasswordBreached(password string) bool {
	breached, _, err := CheckPasswordBreached(password)
	if err != nil {
		// API 出错时仅依赖本地列表结果
		return isCommonPassword(password)
	}
	return breached
}
