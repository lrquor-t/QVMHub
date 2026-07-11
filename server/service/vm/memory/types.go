package memory

import (
	"encoding/xml"
	"sync"
	"time"
)

const (
	MemoryMetadataURI = "https://kvm-console.local/domain-memory"
	MemoryMetadataKey = "kvm-console-memory"

	MemoryCompatLegacyStatic = "legacy_static"
	MemoryCompatDynamic      = "dynamic"
	MemoryCompatPending      = "pending_apply"

	MemoryBackendBalloon   = "balloon"
	MemoryBackendVirtioMem = "virtio_mem"

	SchedulerGroupDynamicMemory         = "动态内存"
	SchedulerKeyDynamicMemoryBalloon    = "dynamic_memory_balloon"
	SchedulerKeyDynamicMemoryVirtioMem  = "dynamic_memory_virtio_mem"
	SchedulerNameDynamicMemoryBalloon   = "气球调度"
	SchedulerNameDynamicMemoryVirtioMem = "Windows 弹性内存调度"
)

// VMMemoryDynamicRequest 是创建/编辑时提交的动态内存配置，单位为 GB。
type VMMemoryDynamicRequest struct {
	DynamicEnabled *bool  `json:"dynamic_enabled,omitempty"`
	MemoryBackend  string `json:"memory_backend,omitempty"`
	MemoryInitial  int    `json:"memory_initial,omitempty"`
	MemoryMin      int    `json:"memory_min,omitempty"`
	MemoryMax      int    `json:"memory_max,omitempty"`
	AutoBalloon    *bool  `json:"memory_auto_balloon,omitempty"`
	MemoryCurrent  int    `json:"memory_current,omitempty"`
}

// VMMemoryDynamicInfo 是接口返回的动态内存状态，单位为 MB。
type VMMemoryDynamicInfo struct {
	DynamicEnabled   bool   `json:"memory_dynamic_enabled"`
	MemoryBackend    string `json:"memory_backend"`
	MemoryInitial    int    `json:"memory_initial"`
	MemoryMin        int    `json:"memory_min"`
	MemoryMax        int    `json:"memory_max"`
	VirtioMemCurrent int    `json:"memory_virtio_mem_current"`
	AutoBalloon      bool   `json:"memory_auto_balloon"`
	PendingApply     bool   `json:"memory_pending_apply"`
	CompatMode       string `json:"memory_compat_mode"`
	BalloonSupported bool   `json:"memory_balloon_supported"`
	BalloonStatus    string `json:"memory_balloon_status"`
	ObservationUntil int64  `json:"memory_observation_until"`
	ManualPauseUntil int64  `json:"memory_manual_pause_until"`
}

// VMMemoryMetadata 是持久化在 libvirt metadata 中的动态内存配置。
type VMMemoryMetadata struct {
	Version          int    `json:"version"`
	DynamicEnabled   bool   `json:"dynamic_enabled"`
	MemoryBackend    string `json:"memory_backend,omitempty"`
	MemoryInitialMB  int    `json:"memory_initial_mb"`
	MemoryMinMB      int    `json:"memory_min_mb"`
	MemoryMaxMB      int    `json:"memory_max_mb"`
	AutoBalloon      bool   `json:"auto_balloon"`
	PendingApply     bool   `json:"pending_apply"`
	ObservationUntil int64  `json:"observation_until"`
	ManualPauseUntil int64  `json:"manual_pause_until"`
	UpdatedAt        int64  `json:"updated_at"`
}

type vmMemoryMetadataXML struct {
	XMLName xml.Name `xml:"memoryConfig"`
	XMLNS   string   `xml:"xmlns,attr,omitempty"`
	Data    string   `xml:",chardata"`
}

type VMMemoryXMLValues struct {
	MemoryMB        int
	CurrentMemoryMB int
}

type vmMemoryStatsValues struct {
	ActualKB    int64
	UnusedKB    int64
	UsableKB    int64
	AvailableKB int64
	RSSKB       int64
}

var MemorySchedulerState = struct {
	sync.Mutex
	LastAdjust map[string]time.Time
	LowUsable  map[string]int
	HighUnused map[string]int
}{LastAdjust: make(map[string]time.Time), LowUsable: make(map[string]int), HighUnused: make(map[string]int)}

var DynamicMemorySchedulerRegisterOnce sync.Once

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SchedulerDefinition 调度器定义（与 service.SchedulerDefinition 结构一致）。
type SchedulerDefinition struct {
	Key         string
	Name        string
	Group       string
	Description string
	Enabled     func() bool
}

// SchedulerEventStartInput 调度事件开始参数（与 service.SchedulerEventStartInput 结构一致）。
type SchedulerEventStartInput struct {
	SchedulerKey   string
	SchedulerName  string
	SchedulerGroup string
	VMName         string
	VMBackend      string
	TriggerReason  string
}

// HostStats 宿主机资源信息（仅包含 memory 包需要的字段）。
type HostStats struct {
	MemTotal int64
	MemFree  int64
}

// VmStats VM 资源缓存信息（仅包含 memory 包需要的字段）。
type VmStats struct{}

// 反向依赖 Hooks - 由 service 根包注入
var (
	HookMemoryGetCachedStats           func(name string) *VmStats
	HookMemoryGetHostStats             func() (*HostStats, error)
	HookMemoryIsMaintenanceModeEnabled func() bool
	HookMemoryInjectMemballoonConfig   func(xmlStr string, enableFPR bool) string
	HookMemoryRegisterScheduler        func(def SchedulerDefinition)
	HookMemoryStartSchedulerEvent      func(input SchedulerEventStartInput) (interface{}, error)
	HookMemoryFinishSchedulerEventOk   func(event interface{}, msg string) error
	HookMemoryFinishSchedulerEventFail func(event interface{}, msg string) error
)
