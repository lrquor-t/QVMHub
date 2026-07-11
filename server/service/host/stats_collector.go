package host

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/digitalocean/go-libvirt"

	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/service/libvirt_rpc"
	"qvmhub/service/lxc"
	vmpkg "qvmhub/service/vm"
	"qvmhub/utils"
)

// ==================== 资源采集缓存 ====================
// 后台协程定时采集运行中VM的资源数据，缓存在内存中供列表接口快速读取，
// 同时定时持久化到数据库供历史查询。

// statsCache 内存缓存：VM名称 -> 最新资源数据
var statsCache = struct {
	sync.RWMutex
	data map[string]*vmpkg.VmStats
}{data: make(map[string]*vmpkg.VmStats)}

// hostStatsCache 宿主机最新资源数据缓存
var hostStatsCache = struct {
	sync.RWMutex
	data *vmpkg.HostStats
}{}

// lxcStatsCache 内存缓存：LXC 容器名 -> 最新资源数据。
// 与 VM 的 statsCache 隔离，避免 VM 内存子系统（HookMemoryGetCachedStats 按名查 statsCache）
// 误读 LXC 条目。
var lxcStatsCache = struct {
	sync.RWMutex
	data map[string]*vmpkg.VmStats
}{data: make(map[string]*vmpkg.VmStats)}

// GetCachedLXCStats 取 LXC 容器最新资源数据（缓存未命中返回零值）。
func GetCachedLXCStats(name string) vmpkg.VmStats {
	lxcStatsCache.RLock()
	defer lxcStatsCache.RUnlock()
	if s := lxcStatsCache.data[name]; s != nil {
		return *s
	}
	return vmpkg.VmStats{}
}

// StartStatsCollector 启动后台资源采集协程
// 每 10 秒采集一次运行中VM的资源数据（更新缓存）
// 每 60 秒将缓存快照持久化到数据库
func StartStatsCollector() {
	HookInitializeVMRuntimeTracker()
	HookInitializeUserRuntimeQuotaTracker()
	HookInitializeLightweightRuntimeQuotaTracker()

	go func() {
		defer utils.RecoverAndLog("host-stats-collector")
		collectTicker := time.NewTicker(10 * time.Second)
		persistTicker := time.NewTicker(60 * time.Second)
		defer collectTicker.Stop()
		defer persistTicker.Stop()

		logger.App.Info("资源采集器已启动", "collectInterval", "10s", "persistInterval", "60s")

		for {
			select {
			case <-collectTicker.C:
				collectHostStats()
				observedAt := time.Now()
				activeVMs, err := HookGetRuntimeActiveVMSetFromHost()
				if err != nil {
					logger.App.Warn("获取宿主机运行中虚拟机列表失败", "error", err)
				} else {
					HookSyncAllUserRuntimeQuotaStatesWithActiveVMs(activeVMs, observedAt)
					HookSyncAllLightweightVMRuntimeQuotaStatesWithActiveVMs(activeVMs, observedAt)
				}
				if !IsMaintenanceModeEnabled() {
					collectAllVMStats()
					// LXC 容器资源采集（CPU/内存/网络/磁盘），写独立缓存 + VmStatsRecord。
					collectLXCStats()
				}
			case <-persistTicker.C:
				persistStatsToDB()
				persistHostStatsToDB()
			}
		}
	}()

	// 启动流量配额检查定时器（每 60 秒检查 + 凌晨重置）
	HookStartTrafficQuotaChecker()
}

// collectHostStats 采集宿主机资源数据
func collectHostStats() {
	stats, err := HookGetHostStats()
	if err == nil {
		hostStatsCache.Lock()
		hostStatsCache.data = stats
		hostStatsCache.Unlock()
	}
}

// collectAllVMStats 批量采集所有运行中VM的资源
func collectAllVMStats() {
	HookSyncVMRuntimeStatesFromHost(time.Now())

	// 获取运行中的VM列表
	names, err := getRunningVMNamesRPC()
	if err != nil {
		logger.Libvirt.Error("获取运行中 VM 列表失败", "error", err)
		return
	}

	runningSet := make(map[string]bool)

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		runningSet[name] = true

		stats, err := collectVMStatsRPC(name)
		if err != nil {
			logger.Libvirt.Warn("采集 VM 统计失败", "vm", name, "error", err)
			continue
		}

		statsCache.Lock()
		statsCache.data[name] = stats
		statsCache.Unlock()
	}

	// 清理已关机的VM缓存
	statsCache.Lock()
	for name := range statsCache.data {
		if !runningSet[name] {
			delete(statsCache.data, name)
		}
	}
	statsCache.Unlock()
}

// getRunningVMNamesRPC 通过 go-libvirt RPC 获取运行中的 VM 名称列表
func getRunningVMNamesRPC() ([]string, error) {
	domains, err := libvirt_rpc.ListAllDomainsRPC()
	if err != nil {
		return nil, err
	}
	l, err := libvirt_rpc.GetLibvirt()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, dom := range domains {
		state, _, _, _, _, infoErr := l.DomainGetInfo(dom)
		if infoErr != nil {
			continue
		}
		if libvirt.DomainState(state) == libvirt.DomainRunning {
			names = append(names, dom.Name)
		}
	}
	return names, nil
}

// collectVMStatsRPC 通过 go-libvirt RPC 采集单台 VM 的实时资源统计
func collectVMStatsRPC(name string) (*vmpkg.VmStats, error) {
	// 获取 vCPU 数量
	vcpuCount, _, _, _, err := libvirt_rpc.GetDomainInfoRPC(name)
	if err != nil {
		return nil, err
	}
	if vcpuCount <= 0 {
		vcpuCount = 1
	}

	// CPU 第一次采样（DomainGetInfo 返回的 cpu_time 为纳秒）
	cpuTime1, err := libvirt_rpc.GetDomainCPUStatsRPC(name)
	if err != nil {
		return nil, err
	}

	// 等待 1 秒再采样
	time.Sleep(time.Second)

	cpuTime2, err := libvirt_rpc.GetDomainCPUStatsRPC(name)
	if err != nil {
		return nil, err
	}

	stats := &vmpkg.VmStats{}

	// 计算 CPU 使用率 = (差值秒数 / 采样间隔 / vCPU数) * 100
	delta := float64(cpuTime2-cpuTime1) / 1e9
	if delta >= 0 {
		stats.CPUPercent = (delta / 1.0 / float64(vcpuCount)) * 100
		if stats.CPUPercent > 100 {
			stats.CPUPercent = 100
		}
	}

	// 内存统计（替代 virsh dommemstat）
	memStats, err := libvirt_rpc.GetDomainMemoryStatsRPC(name)
	if err == nil {
		stats.MemTotal = int64(memStats["actual"])
		stats.MemUsed = stats.MemTotal - int64(memStats["unused"])
		if memStats["available"] > 0 {
			stats.MemUsed = stats.MemTotal - int64(memStats["usable"])
		}
	}

	// 获取当前 XML 以提取网络接口和磁盘设备名
	xmlStr, err := libvirt_rpc.GetDomainXMLRPC(name, 0)
	if err == nil {
		// 网络统计（替代 virsh domifstat）
		ifNames := extractInterfaceTargetDevsFromXML(xmlStr)
		for _, ifName := range ifNames {
			if ifName == "" || ifName == "-" {
				continue
			}
			rxBytes, txBytes, ifErr := libvirt_rpc.GetDomainInterfaceStatsRPC(name, ifName)
			if ifErr == nil {
				stats.NetRxBytes += rxBytes
				stats.NetTxBytes += txBytes
			}
		}

		// 磁盘 I/O 统计（替代 virsh domblkstat）——只取第一个非 cdrom 磁盘
		dev := extractFirstDiskTargetDevFromXML(xmlStr)
		if dev != "" {
			rdReq, rdBytes, wrReq, wrBytes, blkErr := libvirt_rpc.GetDomainBlockStatsRPC(name, dev)
			if blkErr == nil {
				stats.DiskRdBytes = rdBytes
				stats.DiskWrBytes = wrBytes
				stats.DiskRdOps = rdReq
				stats.DiskWrOps = wrReq
			}
		}
	}

	return stats, nil
}

// extractInterfaceTargetDevsFromXML 从 domain XML 中提取所有网络接口的 target dev 名称
func extractInterfaceTargetDevsFromXML(xmlStr string) []string {
	var result []string
	ifaceRe := regexp.MustCompile(`(?s)<interface\s[^>]*>(.*?)</interface>`)
	targetRe := regexp.MustCompile(`<target\s+dev=['"]([^'"]+)['"]`)
	matches := ifaceRe.FindAllStringSubmatch(xmlStr, -1)
	for _, m := range matches {
		if tm := targetRe.FindStringSubmatch(m[1]); len(tm) > 1 {
			result = append(result, tm[1])
		}
	}
	return result
}

// extractFirstDiskTargetDevFromXML 从 domain XML 中提取第一个非 cdrom 磁盘的 target dev 名称
func extractFirstDiskTargetDevFromXML(xmlStr string) string {
	diskRe := regexp.MustCompile(`(?s)<disk\s[^>]*device=['"]disk['"][^>]*>(.*?)</disk>`)
	targetRe := regexp.MustCompile(`<target\s+dev=['"]([^'"]+)['"]`)
	sourceRe := regexp.MustCompile(`<source\s+file=['"]([^'"]+)['"]`)
	matches := diskRe.FindAllStringSubmatch(xmlStr, -1)
	for _, m := range matches {
		// 跳过 ISO 镜像
		if sm := sourceRe.FindStringSubmatch(m[1]); len(sm) > 1 {
			if strings.HasSuffix(sm[1], ".iso") {
				continue
			}
		}
		if tm := targetRe.FindStringSubmatch(m[1]); len(tm) > 1 {
			return tm[1]
		}
	}
	return ""
}

// persistStatsToDB 将当前缓存数据批量写入数据库
func persistStatsToDB() {
	statsCache.RLock()
	defer statsCache.RUnlock()

	now := time.Now()
	for vmName, stats := range statsCache.data {
		record := model.VmStatsRecord{
			VMName:      vmName,
			CPUPercent:  stats.CPUPercent,
			MemUsed:     stats.MemUsed,
			MemTotal:    stats.MemTotal,
			NetRxBytes:  stats.NetRxBytes,
			NetTxBytes:  stats.NetTxBytes,
			DiskRdBytes: stats.DiskRdBytes,
			DiskWrBytes: stats.DiskWrBytes,
			DiskRdOps:   stats.DiskRdOps,
			DiskWrOps:   stats.DiskWrOps,
			RecordedAt:  now,
		}
		if err := model.DB.Create(&record).Error; err != nil {
			logger.App.Error("持久化资源记录失败", "vm", vmName, "error", err)
		}
	}
}

// persistHostStatsToDB 将当前宿主机缓存数据持久化到数据库
func persistHostStatsToDB() {
	hostStatsCache.RLock()
	stats := hostStatsCache.data
	hostStatsCache.RUnlock()

	if stats == nil {
		return
	}

	record := model.HostStatsRecord{
		CPUPercent:  stats.CPUPercent,
		MemUsed:     stats.MemUsed,
		MemTotal:    stats.MemTotal,
		NetRxBytes:  stats.NetRxBytes,
		NetTxBytes:  stats.NetTxBytes,
		DiskRdBytes: stats.DiskRdBytes,
		DiskWrBytes: stats.DiskWrBytes,
		RecordedAt:  time.Now(),
	}
	if err := model.DB.Create(&record).Error; err != nil {
		logger.App.Error("持久化宿主机资源记录失败", "error", err)
	}
}

// GetCachedStats 从缓存获取指定VM的最新资源数据（列表展示用）
func GetCachedStats(name string) *vmpkg.VmStats {
	statsCache.RLock()
	defer statsCache.RUnlock()
	return statsCache.data[name]
}

// GetAllCachedStats 获取全部缓存的资源数据
func GetAllCachedStats() map[string]*vmpkg.VmStats {
	statsCache.RLock()
	defer statsCache.RUnlock()

	copy := make(map[string]*vmpkg.VmStats, len(statsCache.data))
	for k, v := range statsCache.data {
		copy[k] = v
	}
	return copy
}

// DeleteVMStatsRecords 删除指定VM的所有历史资源记录
func DeleteVMStatsRecords(name string) {
	result := model.DB.Where("vm_name = ?", name).Delete(&model.VmStatsRecord{})
	if result.Error != nil {
		logger.App.Warn("清理资源历史记录失败", "vm", name, "error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.App.Info("已清理资源历史记录", "vm", name, "rows", result.RowsAffected)
	}

	// 同时清理缓存
	statsCache.Lock()
	delete(statsCache.data, name)
	statsCache.Unlock()
}

// QueryVMStatsHistory 按日期范围查询VM的资源历史记录
func QueryVMStatsHistory(name string, start, end time.Time) ([]model.VmStatsRecord, error) {
	var records []model.VmStatsRecord
	err := model.DB.Where("vm_name = ? AND recorded_at >= ? AND recorded_at <= ?", name, start, end).
		Order("recorded_at ASC").
		Find(&records).Error
	return records, err
}

// QueryHostStatsHistory 按日期范围查询宿主机的资源历史记录
func QueryHostStatsHistory(start, end time.Time) ([]model.HostStatsRecord, error) {
	var records []model.HostStatsRecord
	err := model.DB.Where("recorded_at >= ? AND recorded_at <= ?", start, end).
		Order("recorded_at ASC").
		Find(&records).Error
	return records, err
}

// ClearRuntimeCachesForMaintenance 清理运行时缓存（维护模式调用，同包可直接访问 statsCache）
func ClearRuntimeCachesForMaintenance() {
	statsCache.Lock()
	statsCache.data = make(map[string]*vmpkg.VmStats)
	statsCache.Unlock()
}

// collectLXCStats 采集所有运行中 LXC 容器的 CPU/内存/网络/磁盘用量：
//   - 更新 lxcStatsCache（供 /lxc/:name/stats 读取，实时值 ≤10s）；
//   - 每 tick 写一行 VmStatsRecord（沿用旧 collectLXCVethStats 的 10s 粒度，
//     网络=order-0 veth，保持 VPC 流量计费 aggregateSwitchMonthlyTrafficRaw 零回归；
//     仅补 CPU/内存/磁盘列，并覆盖所有运行容器而非仅 VPC 绑定）。
func collectLXCStats() {
	if model.DB == nil {
		return
	}
	var rows []model.LXCCache
	if err := model.DB.Where("present = ?", true).Find(&rows).Error; err != nil {
		return
	}
	now := time.Now()
	cache := make(map[string]*vmpkg.VmStats, len(rows))
	for _, row := range rows {
		if !strings.EqualFold(row.Status, "running") {
			continue
		}
		s := lxc.GetContainerStats(row.Name, row.VethName)
		ss := s
		cache[row.Name] = &ss
		record := model.VmStatsRecord{
			VMName:         row.Name,
			CPUPercent:     s.CPUPercent,
			MemUsed:        s.MemUsed,
			MemTotal:       s.MemTotal,
			NetRxBytes:     s.NetRxBytes,
			NetTxBytes:     s.NetTxBytes,
			DiskUsedBytes:  s.DiskUsedBytes,
			DiskTotalBytes: s.DiskTotalBytes,
			RecordedAt:     now,
		}
		if err := model.DB.Create(&record).Error; err != nil {
			logger.App.Warn("持久化 LXC 资源记录失败", "name", row.Name, "error", err)
		}
	}
	lxcStatsCache.Lock()
	lxcStatsCache.data = cache
	lxcStatsCache.Unlock()
}
