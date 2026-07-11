package memory

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/service/libvirt_rpc"
)

// BuildVMMemoryMetadataForCreate 生成新建 VM 的动态内存 metadata 与 XML 参数。
func BuildVMMemoryMetadataForCreate(ramGB int, req *VMMemoryDynamicRequest) (*VMMemoryMetadata, int, int, error) {
	initialGB := ramGB
	if req != nil && req.MemoryInitial > 0 {
		initialGB = req.MemoryInitial
	}
	if initialGB <= 0 {
		return nil, 0, 0, fmt.Errorf("启动内存必须大于 0")
	}

	enabled := false
	if req != nil && req.DynamicEnabled != nil {
		enabled = *req.DynamicEnabled
	}
	if !enabled {
		staticMB := initialGB * 1024
		return nil, staticMB, staticMB, nil
	}

	backend := NormalizeMemoryBackend(req.MemoryBackend)
	if backend == MemoryBackendVirtioMem && (req == nil || req.MemoryInitial <= 0) {
		initialGB = DefaultDynamicMemoryMinGB(ramGB)
	}
	maxGB := DefaultDynamicMemoryMaxGB(initialGB)
	if backend == MemoryBackendVirtioMem {
		maxGB = DefaultDynamicMemoryMaxGB(ramGB)
	}
	if req.MemoryMax > 0 {
		maxGB = req.MemoryMax
	}
	minGB := DefaultDynamicMemoryMinGB(initialGB)
	if req.MemoryMin > 0 {
		minGB = req.MemoryMin
	}
	autoBalloon := true
	if req.AutoBalloon != nil {
		autoBalloon = *req.AutoBalloon
	}
	if backend == MemoryBackendVirtioMem {
		minGB = initialGB
		autoBalloon = false
	}

	meta, err := NewVMMemoryMetadataForBackend(initialGB*1024, minGB*1024, maxGB*1024, autoBalloon, false, backend)
	if err != nil {
		return nil, 0, 0, err
	}
	return meta, meta.MemoryInitialMB, meta.MemoryMaxMB, nil
}

// NormalizeMemoryBackend 标准化内存后端类型。
func NormalizeMemoryBackend(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case MemoryBackendVirtioMem:
		return MemoryBackendVirtioMem
	default:
		return MemoryBackendBalloon
	}
}

// DefaultDynamicMemoryMinGB 返回默认最小动态内存（GB）。
func DefaultDynamicMemoryMinGB(initialGB int) int {
	minGB := initialGB / 2
	if minGB < 1 {
		minGB = 1
	}
	return minGB
}

// DefaultDynamicMemoryMaxGB 返回默认最大动态内存（GB）。
func DefaultDynamicMemoryMaxGB(initialGB int) int {
	maxGB := (initialGB*13 + 9) / 10
	if maxGB < initialGB {
		maxGB = initialGB
	}
	return maxGB
}

// NewVMMemoryMetadataForBackend 创建指定后端的内存 metadata。
func NewVMMemoryMetadataForBackend(initialMB, minMB, maxMB int, autoBalloon bool, pending bool, backend string) (*VMMemoryMetadata, error) {
	backend = NormalizeMemoryBackend(backend)
	if initialMB <= 0 {
		return nil, fmt.Errorf("启动内存必须大于 0")
	}
	if minMB <= 0 {
		minMB = 1024
	}
	if maxMB <= 0 {
		maxMB = initialMB
	}
	if minMB > initialMB {
		return nil, fmt.Errorf("最小内存不能大于启动内存")
	}
	if initialMB > maxMB {
		return nil, fmt.Errorf("启动内存不能大于最大内存")
	}
	if backend == MemoryBackendVirtioMem {
		if initialMB == maxMB {
			return nil, fmt.Errorf("Windows 弹性内存最大内存必须大于基础内存")
		}
		minMB = initialMB
		autoBalloon = false
		pending = false
	}
	now := time.Now()
	observationHours := 24
	if config.GlobalConfig != nil && config.GlobalConfig.DynamicMemoryObservationHours > 0 {
		observationHours = config.GlobalConfig.DynamicMemoryObservationHours
	}
	return &VMMemoryMetadata{
		Version:          1,
		DynamicEnabled:   true,
		MemoryBackend:    backend,
		MemoryInitialMB:  initialMB,
		MemoryMinMB:      minMB,
		MemoryMaxMB:      maxMB,
		AutoBalloon:      autoBalloon,
		PendingApply:     pending,
		ObservationUntil: now.Add(time.Duration(observationHours) * time.Hour).Unix(),
		UpdatedAt:        now.Unix(),
	}, nil
}

// GetVMMemoryDynamicInfo 返回 VM 的动态内存配置；旧 VM 会被推断为静态兼容。
func GetVMMemoryDynamicInfo(name, xmlStr, state string) *VMMemoryDynamicInfo {
	values := ParseDomainMemoryXML(xmlStr)
	if values.MemoryMB <= 0 {
		values.MemoryMB = 1024
	}
	if values.CurrentMemoryMB <= 0 {
		values.CurrentMemoryMB = values.MemoryMB
	}

	balloonSupported := HasUsableMemballoon(xmlStr)
	info := &VMMemoryDynamicInfo{
		DynamicEnabled:   false,
		MemoryBackend:    MemoryBackendBalloon,
		MemoryInitial:    values.CurrentMemoryMB,
		MemoryMin:        DefaultMinMemoryMB(values.CurrentMemoryMB),
		MemoryMax:        values.MemoryMB,
		VirtioMemCurrent: ParseVirtioMemCurrentMB(xmlStr),
		AutoBalloon:      false,
		PendingApply:     false,
		CompatMode:       MemoryCompatLegacyStatic,
		BalloonSupported: balloonSupported,
		BalloonStatus:    ResolveBalloonStatus(name, state, balloonSupported, false),
	}

	meta, err := ReadVMMemoryMetadata(name)
	if err != nil || meta == nil {
		return info
	}
	backend := NormalizeMemoryBackend(meta.MemoryBackend)
	info.DynamicEnabled = meta.DynamicEnabled
	info.MemoryBackend = backend
	info.MemoryInitial = FallbackPositive(meta.MemoryInitialMB, values.CurrentMemoryMB)
	info.MemoryMin = FallbackPositive(meta.MemoryMinMB, DefaultMinMemoryMB(info.MemoryInitial))
	info.MemoryMax = FallbackPositive(meta.MemoryMaxMB, values.MemoryMB)
	info.AutoBalloon = meta.AutoBalloon
	info.PendingApply = meta.PendingApply
	info.ObservationUntil = meta.ObservationUntil
	info.ManualPauseUntil = meta.ManualPauseUntil
	if meta.DynamicEnabled {
		info.CompatMode = MemoryCompatDynamic
	}
	if meta.PendingApply {
		info.CompatMode = MemoryCompatPending
	}
	info.BalloonStatus = ResolveBalloonStatus(name, state, balloonSupported, meta.PendingApply)
	return info
}

// FallbackPositive 如果 value > 0 返回 value，否则返回 fallback。
func FallbackPositive(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// DefaultMinMemoryMB 返回默认最小内存（MB）。
func DefaultMinMemoryMB(initialMB int) int {
	minMB := initialMB / 2
	if minMB < 1024 {
		minMB = 1024
	}
	return minMB
}

// ResolveBalloonStatus 判断 balloon 状态。
func ResolveBalloonStatus(name, state string, supported, pending bool) string {
	if pending {
		return "pending_apply"
	}
	if !supported {
		return "missing_balloon"
	}
	if state != "running" {
		return "not_running"
	}
	if HookMemoryGetCachedStats != nil {
		if cached := HookMemoryGetCachedStats(name); cached != nil {
			return "ok"
		}
	}
	if name == "" {
		return "no_stats"
	}
	return "no_stats"
}

// ReadVMMemoryMetadata 读取 VM 的动态内存 metadata。
func ReadVMMemoryMetadata(name string) (*VMMemoryMetadata, error) {
	result, err := libvirt_rpc.GetDomainMetadataRPC(name, 2, MemoryMetadataURI, 2) // metadataType=2(VIR_DOMAIN_METADATA_ELEMENT), flags=2(VIR_DOMAIN_AFFECT_CONFIG)
	if err != nil {
		text := strings.ToLower(strings.TrimSpace(err.Error()))
		if strings.Contains(text, "metadata not found") || strings.Contains(text, "no metadata") {
			return nil, nil
		}
		return nil, fmt.Errorf("读取动态内存配置失败: %w", err)
	}
	return parseVMMemoryMetadataOutput(result)
}

func parseVMMemoryMetadataOutput(output string) (*VMMemoryMetadata, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		// 部分 libvirt 版本在 metadata 不存在时返回成功但 stdout 为空。
		return nil, nil
	}
	var wrapper vmMemoryMetadataXML
	if err := xml.Unmarshal([]byte(output), &wrapper); err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(wrapper.Data))
	if err != nil {
		return nil, err
	}
	var meta VMMemoryMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// WriteVMMemoryMetadata 写入 VM 的动态内存 metadata。
func WriteVMMemoryMetadata(name string, meta *VMMemoryMetadata) error {
	meta.UpdatedAt = time.Now().Unix()
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	wrapper := vmMemoryMetadataXML{
		XMLNS: MemoryMetadataURI,
		Data:  base64.StdEncoding.EncodeToString(raw),
	}
	xmlBytes, err := xml.Marshal(wrapper)
	if err != nil {
		return err
	}
	if err := libvirt_rpc.SetDomainMetadataRPC(name, 2, string(xmlBytes), MemoryMetadataKey, MemoryMetadataURI, 2); err != nil {
		return fmt.Errorf("写入动态内存配置失败: %w", err)
	}
	return nil
}
