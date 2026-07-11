package vm

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"qvmhub/model"
	"qvmhub/utils"
)

// ParseCPUAffinity 解析 CPU 亲和性输入，支持空格或逗号分隔的核心编号
// 输入示例: "0 1 2" 或 "0,1,2" 或 "0-3" 或 "0,2,4"
// 返回去重排序后的核心编号列表，空字符串表示不设置亲和性
func ParseCPUAffinity(input string) ([]int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}

	// 规范化分隔符：逗号替换为空格
	normalized := strings.ReplaceAll(trimmed, ",", " ")
	// 合并多个连续空格
	spaceRegex := regexp.MustCompile(`\s+`)
	normalized = spaceRegex.ReplaceAllString(normalized, " ")
	parts := strings.Split(strings.TrimSpace(normalized), " ")

	seen := make(map[int]bool)
	var cores []int
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// 支持范围格式，如 "0-3"
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("无效的核心范围格式: %s", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("无效的核心编号: %s", rangeParts[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("无效的核心编号: %s", rangeParts[1])
			}
			if start > end {
				return nil, fmt.Errorf("核心范围起始值 %d 不能大于结束值 %d", start, end)
			}
			if end-start > 256 {
				return nil, fmt.Errorf("核心范围过大: %d-%d", start, end)
			}
			for i := start; i <= end; i++ {
				if !seen[i] {
					seen[i] = true
					cores = append(cores, i)
				}
			}
		} else {
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("无效的核心编号: %s，请输入纯数字", part)
			}
			if num < 0 {
				return nil, fmt.Errorf("核心编号不能为负数: %d", num)
			}
			if !seen[num] {
				seen[num] = true
				cores = append(cores, num)
			}
		}
	}

	if len(cores) == 0 {
		return nil, nil
	}

	return cores, nil
}

// GetSystemCPUCores 获取系统可用的 CPU 核心总数
func GetSystemCPUCores() (int, error) {
	return runtime.NumCPU(), nil
}

// ValidateCPUAffinity 校验 CPU 亲和性核心编号是否在系统可用范围内
func ValidateCPUAffinity(cores []int) error {
	if len(cores) == 0 {
		return nil
	}

	maxCore, err := GetSystemCPUCores()
	if err != nil {
		return err
	}

	for _, core := range cores {
		if core < 0 {
			return fmt.Errorf("CPU 核心编号不能为负数: %d", core)
		}
		if core >= maxCore {
			return fmt.Errorf("CPU 核心编号 %d 超出系统可用范围 (0-%d)", core, maxCore-1)
		}
	}

	return nil
}

// FormatCPUAffinity 将核心编号列表格式化为显示字符串
func FormatCPUAffinity(cores []int) string {
	if len(cores) == 0 {
		return ""
	}
	parts := make([]string, len(cores))
	for i, c := range cores {
		parts[i] = strconv.Itoa(c)
	}
	return strings.Join(parts, ",")
}

// ParseCPUAffinityFromDomainXML 从 domain XML 中解析 CPU 亲和性配置
// 返回格式化后的核心编号字符串，如 "0,2,4"，空字符串表示未设置
func ParseCPUAffinityFromDomainXML(xmlStr string) string {
	// 匹配 <vcpupin vcpu="N" cpuset="X"/> 或 <vcpupin vcpu='N' cpuset='X'/>
	vcpupinRegex := regexp.MustCompile(`<vcpupin\s+vcpu\s*=\s*["'](\d+)["']\s+cpuset\s*=\s*["']([^"']+)["']\s*/?>`)
	matches := vcpupinRegex.FindAllStringSubmatch(xmlStr, -1)
	if len(matches) == 0 {
		return ""
	}

	// 收集所有 cpuset 中指定的核心
	allCores := make(map[int]bool)
	for _, m := range matches {
		cpuset := m[2]
		cores, err := ParseCPUAffinity(cpuset)
		if err != nil {
			continue
		}
		for _, c := range cores {
			allCores[c] = true
		}
	}

	if len(allCores) == 0 {
		return ""
	}

	// 排序
	sorted := make([]int, 0, len(allCores))
	for c := range allCores {
		sorted = append(sorted, c)
	}
	// 简单冒泡排序
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return FormatCPUAffinity(sorted)
}

// ApplyCPUAffinityToDomainXML 将 CPU 亲和性配置写入 domain XML 的 cputune 块
// cores 为空表示清除 CPU 亲和性配置
func ApplyCPUAffinityToDomainXML(xmlStr string, vcpu int, cores []int) string {
	if vcpu <= 0 {
		return xmlStr
	}

	// 先移除现有的 vcpupin 配置
	xmlStr = removeVCPUPinFromXML(xmlStr)

	if len(cores) == 0 {
		return xmlStr
	}

	// 按轮询方式为每个 vCPU 分配核心
	vcpupinLines := buildVCPUPinXML(vcpu, cores)

	// 插入到 cputune 块中
	return insertVCPUPinToCPUTune(xmlStr, vcpupinLines)
}

var (
	cpuAffinityVCPUPinRegex = regexp.MustCompile(`(?s)\n?\s*<vcpupin\s+vcpu\s*=\s*["'][^"']+["']\s+cpuset\s*=\s*["'][^"']+["']\s*(?:/>|></vcpupin>)`)
)

func removeVCPUPinFromXML(xmlStr string) string {
	return cpuAffinityVCPUPinRegex.ReplaceAllString(xmlStr, "")
}

func buildVCPUPinXML(vcpu int, cores []int) string {
	if len(cores) == 0 {
		return ""
	}

	var lines []string
	coreCount := len(cores)
	for i := 0; i < vcpu; i++ {
		coreIdx := i % coreCount
		lines = append(lines, fmt.Sprintf("    <vcpupin vcpu='%d' cpuset='%d'/>", i, cores[coreIdx]))
	}
	return strings.Join(lines, "\n")
}

func insertVCPUPinToCPUTune(xmlStr string, vcpupinLines string) string {
	if vcpupinLines == "" {
		return xmlStr
	}

	cputuneBlockRegex := regexp.MustCompile(`(?s)<cputune\b[^>]*(?:/>|>.*?</cputune>)`)
	block := cputuneBlockRegex.FindString(xmlStr)

	if block == "" {
		// 创建新的 cputune 块
		newBlock := fmt.Sprintf("  <cputune>\n%s\n  </cputune>", vcpupinLines)
		if strings.Contains(xmlStr, "<devices>") {
			return strings.Replace(xmlStr, "<devices>", newBlock+"\n  <devices>", 1)
		}
		return xmlStr
	}

	// 已存在 cputune 块，追加 vcpupin 到末尾
	if strings.Contains(block, "</cputune>") {
		updated := strings.Replace(block, "</cputune>", "\n"+vcpupinLines+"\n  </cputune>", 1)
		return cputuneBlockRegex.ReplaceAllString(xmlStr, updated)
	}

	// 自闭合 cputune 块，替换为完整块
	newBlock := fmt.Sprintf("  <cputune>\n%s\n  </cputune>", vcpupinLines)
	return cputuneBlockRegex.ReplaceAllString(xmlStr, newBlock)
}

// SetVMCPUAffinity 设置虚拟机的 CPU 亲和性
// coresStr 为空字符串表示清除 CPU 亲和性
func SetVMCPUAffinity(name string, coresStr string) error {
	cores, err := ParseCPUAffinity(coresStr)
	if err != nil {
		return fmt.Errorf("CPU 亲和性格式错误: %w", err)
	}

	if len(cores) > 0 {
		if err := ValidateCPUAffinity(cores); err != nil {
			return err
		}
	}

	xmlStr, err := GetVMInactiveDomainXML(name)
	if err != nil {
		return err
	}

	vcpu := ParseVCPUCountFromDomainXML(xmlStr)
	if vcpu <= 0 {
		return fmt.Errorf("无法识别虚拟机 CPU 核心数")
	}

	updatedXML := ApplyCPUAffinityToDomainXML(xmlStr, vcpu, cores)
	if err := SetVMInactiveDomainXML(name, updatedXML); err != nil {
		return err
	}

	// 同步运行态：使用在线域的 vCPU 数量，因为持久化配置的 vCPU 可能已被修改但尚未生效
	stateResult := utils.ExecCommand("virsh", "domstate", name)
	if stateResult.Error == nil && strings.TrimSpace(stateResult.Stdout) == "running" {
		// 获取在线域的 vCPU 数量，避免因持久化与在线 vCPU 不一致导致索引越界
		liveVCPU := vcpu
		liveResult := utils.ExecCommand("virsh", "dumpxml", name)
		if liveResult.Error == nil {
			if parsed := ParseVCPUCountFromDomainXML(liveResult.Stdout); parsed > 0 {
				liveVCPU = parsed
			}
		}
		if err := applyVMLiveCPUAffinity(name, liveVCPU, cores); err != nil {
			return fmt.Errorf("已保存持久化 CPU 亲和性，但同步运行态失败: %w", err)
		}
	}

	RefreshVMCacheByNameAsync(name)
	return nil
}

// applyVMLiveCPUAffinity 将 CPU 亲和性应用到运行中的虚拟机
func applyVMLiveCPUAffinity(name string, vcpu int, cores []int) error {
	if len(cores) == 0 {
		// 清除亲和性：将所有 vCPU 绑定到所有可用核心
		maxCore, err := GetSystemCPUCores()
		if err != nil {
			return err
		}
		allCores := "0"
		if maxCore > 1 {
			allCores = fmt.Sprintf("0-%d", maxCore-1)
		}
		for i := 0; i < vcpu; i++ {
			result := utils.ExecCommand("virsh", "vcpupin", name, strconv.Itoa(i), allCores, "--live")
			if result.Error != nil {
				message := strings.TrimSpace(result.Stderr)
				if message == "" && result.Error != nil {
					message = result.Error.Error()
				}
				return fmt.Errorf("清除 vCPU %d 亲和性失败: %s", i, message)
			}
		}
		return nil
	}

	coreCount := len(cores)
	for i := 0; i < vcpu; i++ {
		coreIdx := i % coreCount
		coreStr := strconv.Itoa(cores[coreIdx])
		result := utils.ExecCommand("virsh", "vcpupin", name, strconv.Itoa(i), coreStr, "--live")
		if result.Error != nil {
			message := strings.TrimSpace(result.Stderr)
			if message == "" && result.Error != nil {
				message = result.Error.Error()
			}
			return fmt.Errorf("设置 vCPU %d 亲和性到核心 %d 失败: %s", i, cores[coreIdx], message)
		}
	}
	return nil
}

// buildCompactCPUSet 将核心列表压缩为最短字符串表示
// 例如 [0,1,2,3,5,7] -> "0-3,5,7"
func buildCompactCPUSet(cores []int) string {
	if len(cores) == 0 {
		return ""
	}

	sorted := make([]int, len(cores))
	copy(sorted, cores)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var parts []string
	start := sorted[0]
	end := sorted[0]

	for i := 1; i < len(sorted); i++ {
		if sorted[i] == end+1 {
			end = sorted[i]
		} else {
			if start == end {
				parts = append(parts, strconv.Itoa(start))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", start, end))
			}
			start = sorted[i]
			end = sorted[i]
		}
	}
	if start == end {
		parts = append(parts, strconv.Itoa(start))
	} else {
		parts = append(parts, fmt.Sprintf("%d-%d", start, end))
	}

	return strings.Join(parts, ",")
}

// ensureIntInRange ensures an int value is within [minVal, maxVal]
func ensureIntInRange(val, minVal, maxVal int) int {
	return int(math.Max(float64(minVal), math.Min(float64(maxVal), float64(val))))
}

// ApplyCPUAffinityIfSet 校验并应用 CPU 亲和性到 domain XML（一键调用）
// 如果 cpuAffinity 为空则直接返回原 XML，不做修改
func ApplyCPUAffinityIfSet(xmlStr string, vcpu int, cpuAffinity string) (string, error) {
	if cpuAffinity == "" {
		return xmlStr, nil
	}
	cores, err := ParseCPUAffinity(cpuAffinity)
	if err != nil {
		return xmlStr, fmt.Errorf("CPU 亲和性格式错误: %w", err)
	}
	if len(cores) > 0 {
		if err := ValidateCPUAffinity(cores); err != nil {
			return xmlStr, err
		}
	}
	return ApplyCPUAffinityToDomainXML(xmlStr, vcpu, cores), nil
}

// CPUAffinityPreset CPU 亲和性预设
type CPUAffinityPreset struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

const cpuAffinityPresetsKey = "cpu_affinity_presets"

// GetCPUAffinityPresets 获取 CPU 亲和性预设列表（默认空，由管理员自行定义）
func GetCPUAffinityPresets() []CPUAffinityPreset {
	val, ok := model.GetSetting(cpuAffinityPresetsKey)
	if !ok || val == "" {
		return nil
	}
	var presets []CPUAffinityPreset
	if err := json.Unmarshal([]byte(val), &presets); err != nil {
		return nil
	}
	return presets
}

// SaveCPUAffinityPresets 保存 CPU 亲和性预设列表
func SaveCPUAffinityPresets(presets []CPUAffinityPreset) error {
	if presets == nil {
		presets = []CPUAffinityPreset{}
	}
	data, err := json.Marshal(presets)
	if err != nil {
		return fmt.Errorf("序列化预设失败: %w", err)
	}
	return model.SetSetting(cpuAffinityPresetsKey, string(data))
}
