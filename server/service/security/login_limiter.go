package security

import (
	"sync"
	"time"
)

// LoginLimiter 双维度登录限制器
type LoginLimiter struct {
	mu          sync.Mutex
	ipEntries   map[string]*loginEntry
	userEntries map[string]*loginEntry

	ipMaxFail   int
	ipLockDur   time.Duration
	userMaxFail int
	userLockDur time.Duration
}

type loginEntry struct {
	failCount int
	lockedAt  time.Time
	lastFail  time.Time
}

var globalLoginLimiter *LoginLimiter

func init() {
	globalLoginLimiter = NewLoginLimiter(5, 5*time.Minute, 10, 15*time.Minute)
	go globalLoginLimiter.cleanupLoop()
}

// NewLoginLimiter 创建登录限制器
func NewLoginLimiter(ipMax int, ipDur time.Duration, userMax int, userDur time.Duration) *LoginLimiter {
	return &LoginLimiter{
		ipEntries:   make(map[string]*loginEntry),
		userEntries: make(map[string]*loginEntry),
		ipMaxFail:   ipMax,
		ipLockDur:   ipDur,
		userMaxFail: userMax,
		userLockDur: userDur,
	}
}

// CheckLoginAllowed 检查是否允许登录尝试，返回 (allowed, reason)
func CheckLoginAllowed(ip, username string) (bool, string) {
	return globalLoginLimiter.check(ip, username)
}

func (l *LoginLimiter) check(ip, username string) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// 检查 IP 维度
	if entry, ok := l.ipEntries[ip]; ok {
		if entry.failCount >= l.ipMaxFail && now.Sub(entry.lockedAt) < l.ipLockDur {
			return false, "登录尝试过于频繁，请稍后重试"
		}
		// 锁定期已过，重置
		if entry.failCount >= l.ipMaxFail && now.Sub(entry.lockedAt) >= l.ipLockDur {
			delete(l.ipEntries, ip)
		}
	}

	// 检查用户名维度
	if username != "" {
		if entry, ok := l.userEntries[username]; ok {
			if entry.failCount >= l.userMaxFail && now.Sub(entry.lockedAt) < l.userLockDur {
				return false, "登录尝试过于频繁，请稍后重试"
			}
			if entry.failCount >= l.userMaxFail && now.Sub(entry.lockedAt) >= l.userLockDur {
				delete(l.userEntries, username)
			}
		}
	}

	return true, ""
}

// RecordLoginFailure 记录登录失败
func RecordLoginFailure(ip, username string) {
	globalLoginLimiter.recordFailure(ip, username)
}

func (l *LoginLimiter) recordFailure(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// IP 维度记录
	entry, ok := l.ipEntries[ip]
	if !ok {
		entry = &loginEntry{}
		l.ipEntries[ip] = entry
	}
	entry.failCount++
	entry.lastFail = now
	if entry.failCount >= l.ipMaxFail && entry.lockedAt.IsZero() {
		entry.lockedAt = now
	}

	// 用户名维度记录
	if username != "" {
		uEntry, ok := l.userEntries[username]
		if !ok {
			uEntry = &loginEntry{}
			l.userEntries[username] = uEntry
		}
		uEntry.failCount++
		uEntry.lastFail = now
		if uEntry.failCount >= l.userMaxFail && uEntry.lockedAt.IsZero() {
			uEntry.lockedAt = now
		}
	}
}

// ClearLoginFailures 登录成功后清除失败计数
func ClearLoginFailures(ip, username string) {
	globalLoginLimiter.clearFailures(ip, username)
}

func (l *LoginLimiter) clearFailures(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.ipEntries, ip)
	if username != "" {
		delete(l.userEntries, username)
	}
}

// cleanupLoop 定期清理过期条目
func (l *LoginLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.cleanup()
	}
}

func (l *LoginLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	maxAge := 30 * time.Minute

	for k, v := range l.ipEntries {
		if now.Sub(v.lastFail) > maxAge {
			delete(l.ipEntries, k)
		}
	}
	for k, v := range l.userEntries {
		if now.Sub(v.lastFail) > maxAge {
			delete(l.userEntries, k)
		}
	}
}
