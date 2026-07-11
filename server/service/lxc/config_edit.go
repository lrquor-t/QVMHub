package lxc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"qvmhub/config"
	"qvmhub/model"
)

// ContainerConfigUpdate 配置编辑入参。
type ContainerConfigUpdate struct {
	CPUShares int    `json:"cpu_shares"`
	MemoryMB  int    `json:"memory_mb"`
	Autostart *bool  `json:"autostart"`
	Remark    string `json:"remark"`
	GroupName string `json:"group_name"`
	MAC       string `json:"mac"` // 通常不改
}

func renderConfigOverrides(u ContainerConfigUpdate) string {
	var b strings.Builder
	if u.CPUShares > 0 {
		fmt.Fprintf(&b, "lxc.cgroup2.cpu.weight = %d\n", u.CPUShares)
	}
	if u.MemoryMB > 0 {
		fmt.Fprintf(&b, "lxc.cgroup2.memory.max = %dM\n", u.MemoryMB)
	}
	if u.Autostart != nil {
		fmt.Fprintf(&b, "lxc.start.auto = %d\n", auto01(*u.Autostart))
	}
	if u.MAC != "" {
		fmt.Fprintf(&b, "lxc.net.0.hwaddr = %s\n", u.MAC)
	}
	return b.String()
}

func auto01(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpdateContainerConfig 更新容器 config 文件 + DB 缓存（remark/group/autostart/cpu/mem）。
func UpdateContainerConfig(name string, u ContainerConfigUpdate) error {
	var row model.LXCCache
	if err := model.DB.Where("name = ?", name).First(&row).Error; err != nil {
		return errors.New("容器不存在")
	}
	// 重写 config 相关行：简单策略——删除旧行再追加新行
	cfg := filepath.Join(config.GlobalConfig.LXCLxcPath, name, "config")
	if err := rewriteConfigKeys(cfg, []string{
		"lxc.cgroup2.cpu.weight", "lxc.cgroup2.memory.max", "lxc.start.auto", "lxc.net.0.hwaddr",
	}, renderConfigOverrides(u)); err != nil {
		return fmt.Errorf("更新配置文件失败: %w", err)
	}
	// 热应用 cgroup（运行中）
	if row.Status == "RUNNING" {
		if u.CPUShares > 0 {
			applyCgroup(row.Name, "cpu.weight", fmt.Sprintf("%d", u.CPUShares))
		}
		if u.MemoryMB > 0 {
			applyCgroup(row.Name, "memory.max", fmt.Sprintf("%dM", u.MemoryMB))
		}
	}
	// 回写 DB
	updates := map[string]interface{}{}
	if u.CPUShares > 0 {
		updates["cpu_shares"] = u.CPUShares
	}
	if u.MemoryMB > 0 {
		updates["memory_mb"] = u.MemoryMB
	}
	if u.Autostart != nil {
		updates["autostart"] = *u.Autostart
	}
	if u.Remark != "" {
		updates["remark"] = u.Remark
	}
	if u.GroupName != "" {
		updates["group_name"] = u.GroupName
	}
	if len(updates) > 0 {
		model.DB.Model(&row).Updates(updates)
	}
	return nil
}

func rewriteConfigKeys(path string, keys []string, appendText string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		drop := false
		for _, k := range keys {
			if strings.HasPrefix(strings.TrimSpace(line), k+" ") || strings.HasPrefix(strings.TrimSpace(line), k+"=") {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, line)
		}
	}
	content := strings.Join(out, "\n") + "\n" + appendText
	return os.WriteFile(path, []byte(content), 0644)
}

func applyCgroup(name, key, val string) {
	// lxc-cgroup -n name key val（cgroup2 路径）
	runCmd("lxc-cgroup", "-n", name, key, val)
}

// CheckLXCQuota 校验用户 LXC 配额（admin 不限）。
func CheckLXCQuota(username string, cpu, ramMB int) error {
	var u model.User
	if err := model.DB.Where("username = ?", username).First(&u).Error; err != nil {
		return errors.New("用户不存在")
	}
	if u.Role == "admin" {
		return nil
	}
	// 现有计数
	var count int64
	var sumCPU, sumRAM int
	{
		var rows []model.LXCCache
		model.DB.Where("owner_username = ? AND present = ?", username, true).Find(&rows)
		count = int64(len(rows))
		for _, r := range rows {
			sumCPU += r.CPUShares
			sumRAM += r.MemoryMB
		}
	}
	if u.MaxLXCCount > 0 && count+1 > int64(u.MaxLXCCount) {
		return fmt.Errorf("超过容器数量配额 %d", u.MaxLXCCount)
	}
	if u.MaxLXCCPU > 0 && sumCPU+cpu > u.MaxLXCCPU {
		return fmt.Errorf("超过 CPU 配额 %d", u.MaxLXCCPU)
	}
	if u.MaxLXCRAMMB > 0 && sumRAM+ramMB > u.MaxLXCRAMMB {
		return fmt.Errorf("超过内存配额 %d MB", u.MaxLXCRAMMB)
	}
	return nil
}

// CheckLXCQuotaForBatch 批量创建配额校验：数量按 n、CPU/内存按 ×n（admin 不限）。
func CheckLXCQuotaForBatch(username string, cpu, ramMB, n int) error {
	var u model.User
	if err := model.DB.Where("username = ?", username).First(&u).Error; err != nil {
		return errors.New("用户不存在")
	}
	if u.Role == "admin" {
		return nil
	}
	var count int64
	var sumCPU, sumRAM int
	{
		var rows []model.LXCCache
		model.DB.Where("owner_username = ? AND present = ?", username, true).Find(&rows)
		count = int64(len(rows))
		for _, r := range rows {
			sumCPU += r.CPUShares
			sumRAM += r.MemoryMB
		}
	}
	if u.MaxLXCCount > 0 && count+int64(n) > int64(u.MaxLXCCount) {
		return fmt.Errorf("超过容器数量配额 %d", u.MaxLXCCount)
	}
	if u.MaxLXCCPU > 0 && sumCPU+cpu*n > u.MaxLXCCPU {
		return fmt.Errorf("超过 CPU 配额 %d", u.MaxLXCCPU)
	}
	if u.MaxLXCRAMMB > 0 && sumRAM+ramMB*n > u.MaxLXCRAMMB {
		return fmt.Errorf("超过内存配额 %d MB", u.MaxLXCRAMMB)
	}
	return nil
}
