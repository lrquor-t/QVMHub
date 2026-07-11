package vm

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"qvmhub/logger"
	"qvmhub/service/libvirt_rpc"
	"qvmhub/utils"
)

// ==================== 虚拟机配置操作 ====================

// EditVMConfig 编辑虚拟机配置（CPU/内存）
func EditVMConfig(name string, vcpu, maxVCPU, memoryMB int) error {
	if err := D.HookEnsureVMNotMigrating(name, "编辑配置"); err != nil {
		return err
	}
	// 检查虚拟机状态
	stateResult := utils.ExecCommand("virsh", "domstate", name)
	if stateResult.Error != nil {
		return fmt.Errorf("获取虚拟机状态失败: %s", stateResult.Stderr)
	}
	state := strings.TrimSpace(stateResult.Stdout)

	// 修改 CPU
	if vcpu > 0 {
		if state == "running" {
			// 先获取在线最大 vCPU：libvirt 不支持在线修改最大值，新 vCPU 超过在线最大值时热添加必然失败
			liveMaxResult := utils.ExecCommand("virsh", "vcpucount", name, "--maximum", "--live")
			liveMax, _ := strconv.Atoi(strings.TrimSpace(liveMaxResult.Stdout))
			if liveMax <= 0 {
				liveMaxResult2 := utils.ExecCommand("virsh", "vcpucount", name, "--maximum")
				liveMax, _ = strconv.Atoi(strings.TrimSpace(liveMaxResult2.Stdout))
			}
			if vcpu <= liveMax {
				if libvirt_rpc.IsLibvirtRPCAvailable() {
					if err := libvirt_rpc.SetDomainVcpusFlagsRPC(name, uint32(vcpu), libvirt_rpc.DomainVcpuLive); err != nil {
						logger.Libvirt.Warn("设置 live vCPU 失败，降级为 virsh", "domain", name, "error", err)
						utils.ExecCommand("virsh", "setvcpus", name, strconv.Itoa(vcpu), "--live")
					}
				} else {
					utils.ExecCommand("virsh", "setvcpus", name, strconv.Itoa(vcpu), "--live")
				}
			}
			// vcpu > liveMax 时仅更新持久化配置，需重启后生效
		}

		// 当 domain XML 中存在 CPU topology 时，virsh setvcpus --config --maximum
		// 会校验 sockets × dies × cores × threads == 目标 vcpu，不匹配则报错。
		// virsh define 同样会校验。因此当存在 topology 时，必须同时修改 vcpu 和 topology
		// 后 define 回去；不存在 topology 时仍用 virsh setvcpus 命令。
		if err := D.SetVMCPUWithTopologySync(name, vcpu, maxVCPU); err != nil {
			return err
		}
	}

	// 修改内存
	if memoryMB > 0 {
		memKB := strconv.Itoa(memoryMB * 1024)
		if state == "running" {
			if libvirt_rpc.IsLibvirtRPCAvailable() {
				if err := libvirt_rpc.SetDomainMemoryFlagsRPC(name, uint64(memoryMB*1024), libvirt_rpc.DomainMemLive); err != nil {
					logger.Libvirt.Warn("设置 live mem 失败，降级为 virsh", "domain", name, "error", err)
					utils.ExecCommand("virsh", "setmem", name, memKB, "--live")
				}
			} else {
				utils.ExecCommand("virsh", "setmem", name, memKB, "--live")
			}
		}
		var result *utils.CmdResult
		if libvirt_rpc.IsLibvirtRPCAvailable() {
			err := libvirt_rpc.SetDomainMemoryFlagsRPC(name, uint64(memoryMB*1024), libvirt_rpc.DomainMemConfig|libvirt_rpc.DomainMemMaximum)
			if err != nil {
				logger.Libvirt.Warn("设置 maxmem 失败，降级为 virsh", "domain", name, "error", err)
				result = utils.ExecCommand("virsh", "setmaxmem", name, memKB, "--config")
				if result.Error != nil {
					return fmt.Errorf("设置最大内存失败: %s", result.Stderr)
				}
			}
		} else {
			result = utils.ExecCommand("virsh", "setmaxmem", name, memKB, "--config")
			if result.Error != nil {
				return fmt.Errorf("设置最大内存失败: %s", result.Stderr)
			}
		}
		if libvirt_rpc.IsLibvirtRPCAvailable() {
			err := libvirt_rpc.SetDomainMemoryFlagsRPC(name, uint64(memoryMB*1024), libvirt_rpc.DomainMemConfig)
			if err != nil {
				logger.Libvirt.Warn("设置 config mem 失败，降级为 virsh", "domain", name, "error", err)
				result = utils.ExecCommand("virsh", "setmem", name, memKB, "--config")
				if result.Error != nil {
					return fmt.Errorf("设置内存失败: %s", result.Stderr)
				}
			}
		} else {
			result = utils.ExecCommand("virsh", "setmem", name, memKB, "--config")
			if result.Error != nil {
				return fmt.Errorf("设置内存失败: %s", result.Stderr)
			}
		}
	}

	return nil
}

// SetVMAutostart 设置虚拟机自动启动
func SetVMAutostart(name string, autostart bool) error {
	if err := D.HookEnsureVMNotMigrating(name, "设置开机自启"); err != nil {
		return err
	}
	if err := libvirt_rpc.SetDomainAutostartRPC(name, autostart); err != nil {
		return fmt.Errorf("设置自动启动失败: %w", err)
	}
	RefreshVMCacheByNameAsync(name)
	return nil
}

// SetVMBootOrder 设置虚拟机启动顺序
func SetVMBootOrder(name string, bootOrder []string) error {
	if err := D.HookEnsureVMNotMigrating(name, "设置启动顺序"); err != nil {
		return err
	}
	// 先尝试 virt-xml 方式
	bootStr := strings.Join(bootOrder, ",")
	result := utils.ExecCommand("virt-xml", name, "--edit", "--boot", bootStr)
	if result.Error == nil {
		return nil
	}

	// 回退到直接编辑 XML 方式
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	xmlContent := xmlResult.Stdout
	lines := strings.Split(xmlContent, "\n")
	var newLines []string
	inOS := false
	osEndInserted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "<os>") || strings.Contains(trimmed, "<os ") {
			inOS = true
		}
		if inOS && strings.Contains(trimmed, "<boot dev=") {
			continue // 跳过旧的 boot 行
		}
		if inOS && strings.Contains(trimmed, "</os>") {
			if !osEndInserted {
				for _, dev := range bootOrder {
					newLines = append(newLines, fmt.Sprintf("    <boot dev='%s'/>", dev))
				}
				osEndInserted = true
			}
			inOS = false
		}
		newLines = append(newLines, line)
	}

	newXML := strings.Join(newLines, "\n")
	xmlPath := fmt.Sprintf("/tmp/_boot-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入启动顺序 XML 失败: %v", err)
	}
	defer os.Remove(xmlPath)
	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	_ = os.Remove(xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("设置启动顺序失败: %s", defineResult.Stderr)
	}
	return nil
}

// ReorderVMDevices 按传入的设备标识符顺序重排 XML 中的 <disk> 元素。
// deviceOrder 是 dev 名称列表（如 ["sdb", "sda", "vda"]），
// 排在列表前面的设备将在 XML 的 <devices> 段中更靠前。
// 由于 libvirt 按 <address> 中的 unit 编号排序设备，
// 交换时同步交换对应磁盘的 unit 编号以真正改变设备顺序。
func ReorderVMDevices(name string, deviceOrder []string) error {
	if len(deviceOrder) == 0 {
		return nil
	}
	if err := D.HookEnsureVMNotMigrating(name, "重排设备顺序"); err != nil {
		return err
	}
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}
	xmlContent := xmlResult.Stdout

	// 构建设备标识符 -> 目标位置的映射
	orderMap := make(map[string]int, len(deviceOrder))
	for i, dev := range deviceOrder {
		orderMap[dev] = i
	}

	// 匹配所有 <disk> 元素（含 cdrom）
	diskRe := regexp.MustCompile(`(?s)<disk\b[^>]*>.*?</disk>`)
	diskMatches := diskRe.FindAllString(xmlContent, -1)
	if len(diskMatches) == 0 {
		return nil
	}

	// 获取每个 disk 的 dev 名称和 address unit
	targetRe := regexp.MustCompile(`<target\s+dev=['"]([^'"]+)['"]\s+bus=['"]([^'"]+)['"]`)
	addrRe := regexp.MustCompile(`<address\s+[^>]*\bunit=['"](\d+)['"]`)
	type diskMeta struct {
		xml     string
		dev     string
		bus     string
		unit    int
		hasUnit bool
	}
	var disks []diskMeta
	for _, diskXML := range diskMatches {
		d := diskMeta{xml: diskXML, unit: -1}
		if m := targetRe.FindStringSubmatch(diskXML); len(m) > 2 {
			d.dev = m[1]
			d.bus = m[2]
		}
		if m := addrRe.FindStringSubmatch(diskXML); len(m) > 1 {
			fmt.Sscanf(m[1], "%d", &d.unit)
			d.hasUnit = true
		}
		disks = append(disks, d)
	}

	// 收集所有在 deviceOrder 中的 SATA/IDE 磁盘的 unit 号
	// 并按目标顺序重分配 unit 号
	type orderedDisk struct {
		idx  int
		unit int
	}
	var ordered []orderedDisk
	for i, d := range disks {
		if _, ok := orderMap[d.dev]; ok && d.hasUnit && (d.bus == "sata" || d.bus == "ide") {
			ordered = append(ordered, orderedDisk{idx: i, unit: d.unit})
		}
	}
	// 按 deviceOrder 中的顺序对 ordered 排序
	sort.SliceStable(ordered, func(i, j int) bool {
		return orderMap[disks[ordered[i].idx].dev] < orderMap[disks[ordered[j].idx].dev]
	})
	// 构建 unit 映射：原 unit -> 新 unit
	unitMap := make(map[int]int, len(ordered))
	for newIdx, od := range ordered {
		unitMap[od.unit] = newIdx
	}

	// 重新分配 unit 号：使用占位符避免交换时的字符串冲突
	newXML := xmlContent
	// 首先把所有受影响的 unit 替换为占位符
	type unitReplace struct {
		oldAddr    string
		newUnit    int
		newAddrStr string // 替换后的 address 字符串（带新 unit）
	}
	var replacements []unitReplace
	for _, d := range disks {
		if !d.hasUnit {
			continue
		}
		if newUnit, ok := unitMap[d.unit]; ok {
			oldAddr := addrRe.FindString(d.xml)
			if oldAddr == "" {
				continue
			}
			newAddr := oldAddr
			newAddr = strings.Replace(newAddr, fmt.Sprintf("unit='%d'", d.unit), fmt.Sprintf("unit='%d'", newUnit), 1)
			newAddr = strings.Replace(newAddr, fmt.Sprintf(`unit="%d"`, d.unit), fmt.Sprintf(`unit="%d"`, newUnit), 1)
			replacements = append(replacements, unitReplace{oldAddr: oldAddr, newUnit: newUnit, newAddrStr: newAddr})
		}
	}
	// Phase 1: 替换为唯一占位符
	for i, r := range replacements {
		ph := fmt.Sprintf("__QVM_UNIT_PH_%d__", i)
		newXML = strings.Replace(newXML, r.oldAddr, ph, 1)
	}
	// Phase 2: 从占位符还原为新 unit
	for i, r := range replacements {
		ph := fmt.Sprintf("__QVM_UNIT_PH_%d__", i)
		newXML = strings.Replace(newXML, ph, r.newAddrStr, 1)
	}

	// 写入临时文件并 define
	xmlPath := fmt.Sprintf("/tmp/_reorder-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入设备顺序 XML 失败: %v", err)
	}
	defer os.Remove(xmlPath)
	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	_ = os.Remove(xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("重排设备顺序失败: %s", defineResult.Stderr)
	}
	return nil
}

// SetVMNicModel 修改虚拟机网卡模型（通过编辑 XML）
func SetVMNicModel(name, nicModel string) error {
	state := strings.TrimSpace(utils.ExecCommand("virsh", "domstate", name).Stdout)
	if state == "running" {
		return fmt.Errorf("修改网卡类型需要先关机")
	}

	// 获取当前 XML
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	// 替换 model type
	xmlStr := xmlResult.Stdout
	lines := strings.Split(xmlStr, "\n")
	var newLines []string
	inInterface := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "<interface ") {
			inInterface = true
		}
		if inInterface && strings.Contains(trimmed, "<model type='") {
			// 替换为新的网卡模型
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			line = fmt.Sprintf("%s<model type='%s'/>", indent, nicModel)
			inInterface = false // 只修改第一个网卡
		}
		if inInterface && strings.Contains(trimmed, "</interface>") {
			inInterface = false
		}
		newLines = append(newLines, line)
	}

	newXML := strings.Join(newLines, "\n")
	xmlPath := fmt.Sprintf("/tmp/_nic-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入网卡 XML 失败: %v", err)
	}
	defer os.Remove(xmlPath)

	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	_ = os.Remove(xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("修改网卡类型失败: %s", defineResult.Stderr)
	}

	return nil
}
