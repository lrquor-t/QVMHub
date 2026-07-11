package template

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/service/ip_resolver"
	"qvmhub/service/libvirt_rpc"
	"qvmhub/service/vm_xml"
	"qvmhub/utils"
)

// getMetaPath returns the .meta.json path for a template qcow2 file.
func getMetaPath(templatePath string) string {
	return strings.TrimSuffix(templatePath, ".qcow2") + ".meta.json"
}

// getTemplateNVRAMPath returns the .nvram.fd path for a template qcow2 file.
func getTemplateNVRAMPath(templatePath string) string {
	return strings.TrimSuffix(templatePath, ".qcow2") + ".nvram.fd"
}

func loadTemplateMeta(templatePath string) *TemplateMeta {
	metaPath := getMetaPath(templatePath)
	data, err := os.ReadFile(metaPath)
	if err != nil || len(data) == 0 {
		return nil
	}
	var meta TemplateMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

func saveTemplateMeta(templatePath string, meta *TemplateMeta) error {
	metaPath := getMetaPath(templatePath)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	// 临时移除不可变属性以允许写入
	_ = utils.RemoveFileImmutable(metaPath)
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		return fmt.Errorf("保存元数据失败: %w", err)
	}
	// 重新设置不可变属性
	_ = utils.SetFileImmutable(metaPath)
	return nil
}

// ensureTemplatePath returns the full path for a template qcow2 file or an error if it doesn't exist.
func EnsureTemplatePath(templateName string) (string, error) {
	templateDir := config.GlobalConfig.TemplateDir
	templatePath := filepath.Join(templateDir, templateName+".qcow2")
	if _, err := os.Stat(templatePath); err != nil {
		return "", fmt.Errorf("模板不存在: %s", templateName)
	}
	return templatePath, nil
}

func deleteTemplateFiles(templateName string) error {
	templatePath, err := EnsureTemplatePath(templateName)
	if err != nil {
		return err
	}
	// 移除不可变属性，允许删除
	_ = utils.RemoveFileImmutable(templatePath)
	metaPath := getMetaPath(templatePath)
	_ = utils.RemoveFileImmutable(metaPath)

	if err := os.Remove(templatePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除模板文件失败: %w", err)
	}
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除模板元数据失败: %w", err)
	}
	return nil
}

// ── Type normalization helpers ──

func isValidTemplateType(templateType string) bool {
	switch strings.ToLower(strings.TrimSpace(templateType)) {
	case "linux", "windows", "fnos", "openwrt", "other":
		return true
	default:
		return false
	}
}

func normalizeTemplateType(templateType string) string {
	templateType = strings.ToLower(strings.TrimSpace(templateType))
	if isValidTemplateType(templateType) {
		return templateType
	}
	return ""
}

func detectTemplateTypeFromName(name string) string {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	if strings.Contains(nameLower, "win") || strings.Contains(nameLower, "windows") {
		return "windows"
	}
	if strings.Contains(nameLower, "fnos") || strings.Contains(nameLower, "nas") {
		return "fnos"
	}
	if strings.Contains(nameLower, "openwrt") || strings.Contains(nameLower, "lede") || strings.Contains(nameLower, "istoreos") {
		return "openwrt"
	}
	return "linux"
}

func normalizeTemplateCategory(templateType, category string) string {
	return normalizeTemplateCategoryForName(templateType, category, "")
}

func normalizeTemplateCategoryForName(templateType, category, templateName string) string {
	normalizedType := normalizeTemplateType(templateType)
	if normalizedType != "linux" && normalizedType != "windows" && normalizedType != "openwrt" {
		return ""
	}
	category = strings.TrimSpace(category)
	if category == "" {
		if normalizedType == "windows" {
			return detectWindowsTemplateCategoryFromName(templateName)
		}
		if normalizedType == "openwrt" {
			return detectOpenWrtTemplateCategoryFromName(templateName)
		}
		return defaultLinuxTemplateCategory
	}
	var allowedCategories []string
	var defaultCategory string
	switch normalizedType {
	case "windows":
		allowedCategories = windowsTemplateCategories
		defaultCategory = detectWindowsTemplateCategoryFromName(templateName)
	case "openwrt":
		allowedCategories = openwrtTemplateCategories
		defaultCategory = detectOpenWrtTemplateCategoryFromName(templateName)
	default:
		allowedCategories = linuxTemplateCategories
		defaultCategory = defaultLinuxTemplateCategory
	}
	for _, allowed := range allowedCategories {
		if strings.EqualFold(category, allowed) {
			return allowed
		}
	}
	return defaultCategory
}

func detectWindowsTemplateCategoryFromName(templateName string) string {
	nameLower := strings.ToLower(strings.TrimSpace(templateName))
	compact := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "").Replace(nameLower)
	switch {
	case strings.Contains(compact, "windows11") ||
		strings.Contains(compact, "win11"):
		return "Windows11"
	case strings.Contains(compact, "windowsserver2012r2") ||
		strings.Contains(compact, "server2012r2") ||
		strings.Contains(compact, "win2012r2") ||
		strings.Contains(compact, "win2k12r2"):
		return "WindowsServer2012R2"
	case strings.Contains(compact, "windows10") ||
		strings.Contains(compact, "win10"):
		return "Windows10"
	default:
		return defaultWindowsTemplateCategory
	}
}

func detectOpenWrtTemplateCategoryFromName(templateName string) string {
	nameLower := strings.ToLower(strings.TrimSpace(templateName))
	if strings.Contains(nameLower, "istoreos") || strings.Contains(nameLower, "istore") {
		return "iStoreOS"
	}
	return defaultOpenWrtTemplateCategory
}

func normalizeTemplateDefaultConfig(config *TemplateDefaultConfig) *TemplateDefaultConfig {
	if config == nil {
		return nil
	}
	normalized := *config
	if normalized.VCPU < 0 {
		normalized.VCPU = 0
	}
	if normalized.RAM < 0 {
		normalized.RAM = 0
	}
	if normalized.DiskSize < 0 {
		normalized.DiskSize = 0
	}
	if strings.TrimSpace(normalized.DiskBus) != "" {
		normalized.DiskBus = HookNormalizeVMDiskBus(normalized.DiskBus)
	}
	if strings.TrimSpace(normalized.NicModel) != "" {
		normalized.NicModel = HookNormalizeVMNicModel(normalized.NicModel)
	}
	if strings.TrimSpace(normalized.VideoModel) != "" {
		switch strings.ToLower(strings.TrimSpace(normalized.VideoModel)) {
		case vm_xml.VMVideoModelVirtio, vm_xml.VMVideoModelVGA, vm_xml.VMVideoModelVMVGA, vm_xml.VMVideoModelCirrus:
			normalized.VideoModel = strings.ToLower(strings.TrimSpace(normalized.VideoModel))
		default:
			normalized.VideoModel = ""
		}
	}
	if strings.TrimSpace(normalized.CPUTopologyMode) != "" {
		normalized.CPUTopologyMode = HookNormalizeVMCPUTopologyMode(normalized.CPUTopologyMode)
	}
	if strings.TrimSpace(normalized.FirstBootRebootMode) != "" {
		normalized.FirstBootRebootMode = HookNormalizeVMFirstBootRebootMode(normalized.FirstBootRebootMode)
	}
	if normalized.VCPU <= 0 && normalized.RAM <= 0 && normalized.DiskSize <= 0 &&
		strings.TrimSpace(normalized.DiskBus) == "" && strings.TrimSpace(normalized.NicModel) == "" &&
		strings.TrimSpace(normalized.VideoModel) == "" && strings.TrimSpace(normalized.CPUTopologyMode) == "" &&
		strings.TrimSpace(normalized.FirstBootRebootMode) == "" {
		return nil
	}
	return &normalized
}

// ── Video model / CPU topology helpers ──

func getVMVideoModel(vmName string) string {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return ""
	}
	if xmlStr, err := libvirt_rpc.GetDomainXMLRPC(vmName, 2); err == nil {
		if videoModel := vm_xml.ParseVMVideoModelFromDomainXML(xmlStr); videoModel != "" {
			return videoModel
		}
	}
	if xmlStr, err := libvirt_rpc.GetDomainXMLRPC(vmName, 0); err == nil {
		return vm_xml.ParseVMVideoModelFromDomainXML(xmlStr)
	}
	return ""
}

func getVMCPUTopologyMode(vmName string) string {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return ""
	}
	if xmlStr, err := libvirt_rpc.GetDomainXMLRPC(vmName, 2); err == nil {
		if mode := HookParseVMCPUTopologyModeFromDomainXML(xmlStr); mode != "" {
			return mode
		}
	}
	if xmlStr, err := libvirt_rpc.GetDomainXMLRPC(vmName, 0); err == nil {
		return HookParseVMCPUTopologyModeFromDomainXML(xmlStr)
	}
	return ""
}

func inheritMissingTemplateVideoModelFromSource(meta *TemplateMeta) {
	if meta == nil || strings.TrimSpace(meta.CreatedFromVM) == "" {
		return
	}
	if meta.DefaultConfig != nil && strings.TrimSpace(meta.DefaultConfig.VideoModel) != "" {
		return
	}
	videoModel := getVMVideoModel(meta.CreatedFromVM)
	if videoModel == "" {
		return
	}
	if meta.DefaultConfig == nil {
		meta.DefaultConfig = &TemplateDefaultConfig{}
	}
	meta.DefaultConfig.VideoModel = videoModel
	meta.DefaultConfig = normalizeTemplateDefaultConfig(meta.DefaultConfig)
}

func inheritMissingTemplateCPUTopologyFromSource(meta *TemplateMeta) {
	if meta == nil || strings.TrimSpace(meta.CreatedFromVM) == "" {
		return
	}
	if meta.DefaultConfig != nil && strings.TrimSpace(meta.DefaultConfig.CPUTopologyMode) != "" {
		return
	}
	mode := getVMCPUTopologyMode(meta.CreatedFromVM)
	if mode == "" || mode == VMCPUTopologyAuto {
		return
	}
	if meta.DefaultConfig == nil {
		meta.DefaultConfig = &TemplateDefaultConfig{}
	}
	meta.DefaultConfig.CPUTopologyMode = mode
	meta.DefaultConfig = normalizeTemplateDefaultConfig(meta.DefaultConfig)
}

// ── normalizeLoadedTemplateMeta ──

func normalizeLoadedTemplateMetaWithDetector(name, path string, meta *TemplateMeta, hasMeta bool, detector func(string) string) TemplateMeta {
	normalized := TemplateMeta{}
	if meta != nil {
		normalized = *meta
	}
	if strings.TrimSpace(normalized.Type) == "" {
		normalized.Type = detectTemplateTypeFromName(name)
	}
	normalized.Category = normalizeTemplateCategoryForName(normalized.Type, normalized.Category, name)
	normalized.BootType, normalized.BootVerified = resolveTemplateBootType(path, normalized.Type, normalized.BootType, normalized.BootVerified, detector)
	if strings.TrimSpace(normalized.NVRAMPath) == "" {
		nvramPath := getTemplateNVRAMPath(path)
		if _, err := os.Stat(nvramPath); err == nil {
			normalized.NVRAMPath = nvramPath
		}
	} else if !filepath.IsAbs(normalized.NVRAMPath) {
		normalized.NVRAMPath = filepath.Join(filepath.Dir(path), normalized.NVRAMPath)
	}
	normalized.DefaultConfig = normalizeTemplateDefaultConfig(normalized.DefaultConfig)
	inheritMissingTemplateVideoModelFromSource(&normalized)
	inheritMissingTemplateCPUTopologyFromSource(&normalized)
	if strings.TrimSpace(normalized.AdminName) == "" {
		normalized.AdminName = name
	}
	if strings.TrimSpace(normalized.DisplayName) == "" {
		normalized.DisplayName = normalized.AdminName
	}
	if strings.TrimSpace(normalized.NodeID) == "" {
		normalized.NodeID = "legacy_" + name
	}
	if strings.TrimSpace(normalized.TemplateUID) == "" {
		normalized.TemplateUID = "legacy_" + normalized.NodeID
	}
	if strings.TrimSpace(normalized.RootNodeID) == "" {
		if strings.TrimSpace(normalized.ParentNodeID) == "" {
			normalized.RootNodeID = normalized.NodeID
		} else {
			normalized.RootNodeID = normalized.ParentNodeID
		}
	}
	if strings.TrimSpace(normalized.CreatedAt) == "" {
		if stat, err := os.Stat(path); err == nil {
			normalized.CreatedAt = stat.ModTime().Format(time.RFC3339)
		}
	}
	if !hasMeta {
		normalized.CloneVisible = false
	}
	return normalized
}

func normalizeLoadedTemplateMeta(name, path string, meta *TemplateMeta, hasMeta bool) TemplateMeta {
	return normalizeLoadedTemplateMetaWithDetector(name, path, meta, hasMeta, DetectTemplateBootType)
}

// ── VM default config collection ──

func collectVMTemplateDefaultConfig(vmName string) *TemplateDefaultConfig {
	cfg := &TemplateDefaultConfig{}

	vcpu, maxMemKB, usedMemKB, _, err := libvirt_rpc.GetDomainInfoRPC(vmName)
	if err == nil {
		cfg.VCPU = vcpu
		memoryMB := int(maxMemKB) / 1024
		if memoryMB <= 0 {
			memoryMB = int(usedMemKB) / 1024
		}
		if memoryMB > 0 {
			cfg.RAM = int(math.Round(float64(memoryMB) / 1024.0))
			if cfg.RAM <= 0 {
				cfg.RAM = 1
			}
		}
	}

	if disks, err := HookListDisks(vmName); err == nil {
		for _, disk := range disks {
			if disk.DeviceType != "" && disk.DeviceType != "disk" {
				continue
			}
			if cfg.DiskSize <= 0 {
				cfg.DiskSize = parseSizeValueToGB(disk.CapacityGB)
			}
			if strings.TrimSpace(cfg.DiskBus) == "" {
				cfg.DiskBus = strings.TrimSpace(disk.Bus)
			}
			if cfg.DiskSize > 0 && strings.TrimSpace(cfg.DiskBus) != "" {
				break
			}
		}
	}

	if cfg.DiskSize <= 0 {
		diskInfo := HookGetVMDiskInfo(vmName)
		if strings.TrimSpace(diskInfo.Path) != "" {
			result := qemuImgInfoBrief(diskInfo.Path)
			if result.VirtualSize > 0 {
				cfg.DiskSize = int(math.Ceil(float64(result.VirtualSize) / float64(1<<30)))
			}
		}
	}

	netInfo := HookGetVMNetworkInfo(vmName)
	if strings.TrimSpace(netInfo.NICModel) != "" {
		cfg.NicModel = netInfo.NICModel
	}
	cfg.VideoModel = getVMVideoModel(vmName)
	cfg.CPUTopologyMode = getVMCPUTopologyMode(vmName)

	return normalizeTemplateDefaultConfig(cfg)
}

// qemuImgInfoBriefResult holds a subset of qemu-img info output.
type qemuImgInfoBriefResult struct {
	VirtualSize int64
}

func qemuImgInfoBrief(diskPath string) qemuImgInfoBriefResult {
	// Use the templateDiskInfoCommand to get info
	result := templateDiskInfoCommand(diskPath)
	if result == nil || result.Error != nil {
		return qemuImgInfoBriefResult{}
	}
	var info struct {
		VirtualSize int64 `json:"virtual-size"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &info); err != nil {
		return qemuImgInfoBriefResult{}
	}
	return qemuImgInfoBriefResult{VirtualSize: info.VirtualSize}
}

func parseSizeValueToGB(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	matched := regexp.MustCompile(`[\d.]+`).FindString(value)
	if matched == "" {
		return 0
	}
	size, err := strconv.ParseFloat(matched, 64)
	if err != nil || size <= 0 {
		return 0
	}
	return int(math.Ceil(size))
}

// hydrateTemplateRelatedVMs fills in Status and IP for related VMs.
func hydrateTemplateRelatedVMs(vms []TemplateRelatedVM) []TemplateRelatedVM {
	for i := range vms {
		state := strings.TrimSpace(HookGetDomainState(vms[i].Name))
		if state == "" {
			state = "unknown"
		}
		vms[i].Status = state
		vms[i].IP = ip_resolver.GetVMIP(vms[i].Name, state == "running")
	}
	return vms
}
