package lxc

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/service/bandwidth"
	"qvmhub/utils"

	"gorm.io/gorm"
)

// AttachContainerToVPC 建立 VPCVMBinding（Kind=lxc）并在容器启动后把 host veth
// 接入 OVS 桥、打 VLAN tag。VLAN/ACL/带宽策略复用既有 VPC 运行时工具（见 Task 7 Step 1
// 探查）：此处采用与 VM 路径一致的 `ovs-vsctl set Port <veth> tag=<vlan>` 直接表达。
func AttachContainerToVPC(name string, switchID, sgID uint) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("容器名不能为空")
	}
	if switchID == 0 {
		return nil // 未指定交换机：用默认桥，不打 VLAN
	}
	binding := model.VPCVMBinding{
		VMName:          name,
		Username:        ownerOf(name),
		SwitchID:        switchID,
		SecurityGroupID: sgID,
		InterfaceOrder:  0,
		Kind:            "lxc",
	}
	if err := model.DB.Where("vm_name = ? AND interface_order = ?", name, 0).
		Assign(binding).FirstOrCreate(&binding).Error; err != nil {
		return fmt.Errorf("写入 VPC 绑定失败: %w", err)
	}
	var sw model.VPCSwitch
	if err := model.DB.First(&sw, switchID).Error; err != nil {
		return fmt.Errorf("交换机不存在: %w", err)
	}
	veth := waitForVeth(name)
	if veth == "" {
		return fmt.Errorf("无法解析容器 %s 的 host veth", name)
	}
	bridge := config.GlobalConfig.OVSBridge
	if bridge == "" {
		bridge = "br-ovs"
	}
	// 接入 OVS 网关桥（端口可能已存在，--may-exist 保证幂等）。
	utils.ExecCommandQuiet("ovs-vsctl", "--may-exist", "add-port", bridge, veth)
	if sw.VLANID > 0 {
		if r := utils.ExecCommand("ovs-vsctl", "set", "Port", veth, fmt.Sprintf("tag=%d", sw.VLANID)); r.Error != nil {
			return fmt.Errorf("设置 VLAN tag 失败: %s", r.Stderr)
		}
	}
	// 回填 host veth 到缓存行。
	model.DB.Model(&model.LXCCache{}).Where("name = ?", name).Update("veth_name", veth)
	return nil
}

// DetachContainerFromVPC 从 OVS 删除全部 host veth 端口（多卡逐个按 MAC 解析）并清理 Kind=lxc 的绑定。
func DetachContainerFromVPC(name string) error {
	if strings.TrimSpace(name) == "" || model.DB == nil {
		return nil
	}
	var bindings []model.VPCVMBinding
	model.DB.Where("vm_name = ? AND kind = ?", name, "lxc").Find(&bindings)
	bridge := defaultBridge()
	seen := map[string]bool{}
	for _, b := range bindings {
		veth := findContainerHostVeth(name, b.InterfaceOrder)
		if veth != "" && !seen[veth] {
			utils.ExecCommandQuiet("ovs-vsctl", "--if-exists", "del-port", bridge, veth)
			seen[veth] = true
		}
	}
	// 兼容：旧版只回填单值 VethName 的容器
	var row model.LXCCache
	if err := model.DB.Where("name = ?", name).First(&row).Error; err == nil && row.VethName != "" && !seen[row.VethName] {
		utils.ExecCommandQuiet("ovs-vsctl", "--if-exists", "del-port", bridge, row.VethName)
	}
	model.DB.Where("vm_name = ? AND kind = ?", name, "lxc").Delete(&model.VPCVMBinding{})
	return nil
}

// ResolveContainerVPCIP 取容器在 VPC 内的 IPv4（lxc-info -i）。
func ResolveContainerVPCIP(name string) string {
	res := LxcInfo(name)
	if res.ExitCode != 0 {
		return ""
	}
	d, _ := ParseLxcInfo(res.Stdout)
	fields := strings.Fields(d.IP)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSpace(fields[0])
}

// ---- helpers ----

func ownerOf(name string) string {
	if model.DB == nil {
		return "admin"
	}
	var row model.LXCCache
	if err := model.DB.Where("name = ?", name).First(&row).Error; err == nil {
		return row.OwnerUsername
	}
	return "admin"
}

// waitForVeth 解析容器 order0 网卡在 host 侧的 veth 名（按网络命名空间，非 MAC）。
func waitForVeth(name string) string {
	return findContainerHostVeth(name, 0)
}

// ReadVethCounters 读取 host veth 的累计 rx/tx 字节数（来自 sysfs）。
// 用于流量采集的 lxc 分支：取代 VM 的 libvirt 接口统计。
func ReadVethCounters(veth string) (int64, int64) {
	if strings.TrimSpace(veth) == "" {
		return 0, 0
	}
	return readSysCounter(veth, "rx_bytes"), readSysCounter(veth, "tx_bytes")
}

func readSysCounter(veth, name string) int64 {
	b, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/statistics/%s", veth, name))
	if err != nil {
		return 0
	}
	var v int64
	fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &v)
	return v
}

// configPath 返回容器 config 文件路径（lxc.lxcpath/<name>/config）。
func configPath(name string) string {
	return filepath.Join(config.GlobalConfig.LXCLxcPath, name, "config")
}

// applyNicRuntime 对单个 host veth 幂等施加 OVS 端口 + VLAN tag + 下行限速。
// veth 为空（容器未运行 / 暂无 veth）时跳过，不报错；order 0 总会回填 LXCCache.VethName
// 以兼容现有流量采集/Detach 路径。
func applyNicRuntime(name string, order int, sw model.VPCSwitch, binding model.VPCVMBinding) error {
	veth := findContainerHostVeth(name, order)
	if order == 0 {
		// 兼容现有流量采集/Detach 读 LXCCache.VethName；即使容器未运行也清空旧值。
		model.DB.Model(&model.LXCCache{}).Where("name = ?", name).Update("veth_name", veth)
	}
	if veth == "" {
		return nil // 容器未运行，无 veth 可施加
	}
	bridge := strings.TrimSpace(sw.BridgeName)
	if bridge == "" {
		bridge = config.GlobalConfig.OVSBridge
		if bridge == "" {
			bridge = "br-ovs"
		}
	}
	utils.ExecCommandQuiet("ovs-vsctl", "--may-exist", "add-port", bridge, veth)
	if sw.VLANID > 0 {
		if r := utils.ExecCommand("ovs-vsctl", "set", "Port", veth, fmt.Sprintf("tag=%d", sw.VLANID)); r.Error != nil {
			return fmt.Errorf("设置 VLAN tag 失败: %s", r.Stderr)
		}
	}
	// 下行限速（按端口名，libvirt 无关）；0 = 不限
	if binding.BandwidthInboundAvg > 0 {
		applyNicRateLimit(veth, binding.BandwidthInboundAvg)
	}
	return nil
}

// applyNicRateLimit 对 host veth 打 tc 下行限速（Mbps）。best-effort，失败仅告警不中断。
func applyNicRateLimit(veth string, downMbps int) {
	if veth == "" || downMbps <= 0 {
		return
	}
	bandwidth.ApplyTCVPCSwitchDownlinkLimit(veth, downMbps)
}

// ReconcileContainerNICs 在容器启动后对其全部 VPCVMBinding(kind=lxc) 施加 OVS/VLAN/限速。
// 修复「重启丢 OVS」缺口（host veth 每次启动换名 → 旧 Stop 路径删过端口、Start 路径不重接），
// 并使停机态新增的卡在下次启动生效。幂等：缺失 veth / 无绑定 / 已存在的 OVS 端口都不报错。
func ReconcileContainerNICs(name string) error {
	var bindings []model.VPCVMBinding
	if err := model.DB.Where("vm_name = ? AND kind = ?", name, "lxc").
		Order("interface_order ASC").Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		return nil
	}
	// 等 order 0 的 veth 出现（最长 ~5s）：lxc-start 返回后内核创建 veth 有延迟。
	deadline := 5 * time.Second
	start := time.Now()
	for time.Since(start) < deadline {
		if veth := findContainerHostVeth(name, bindings[0].InterfaceOrder); veth != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	var lastErr error
	for _, b := range bindings {
		var sw model.VPCSwitch
		if err := model.DB.First(&sw, b.SwitchID).Error; err != nil {
			lastErr = err
			continue
		}
		if err := applyNicRuntime(name, b.InterfaceOrder, sw, b); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

const maxLXCInterfaces = 8

// ListContainerInterfaces 列出容器全部网卡（config + VPCVMBinding + 运行态）。
func ListContainerInterfaces(name string) ([]LXCInterfaceInfo, error) {
	data, err := os.ReadFile(configPath(name))
	if err != nil {
		return nil, fmt.Errorf("读取容器 config 失败: %w", err)
	}
	_, blocks := SplitNICBlocks(string(data))
	var bindings []model.VPCVMBinding
	model.DB.Where("vm_name = ? AND kind = ?", name, "lxc").Order("interface_order ASC").Find(&bindings)
	bindingByOrder := map[int]model.VPCVMBinding{}
	for _, b := range bindings {
		bindingByOrder[b.InterfaceOrder] = b
	}
	// order 集合 = config 块 ∪ binding
	orderSet := map[int]bool{}
	for o := range blocks {
		orderSet[o] = true
	}
	for o := range bindingByOrder {
		orderSet[o] = true
	}
	orders := make([]int, 0, len(orderSet))
	for o := range orderSet {
		orders = append(orders, o)
	}
	sort.Ints(orders)
	ip := ResolveContainerVPCIP(name)
	out := make([]LXCInterfaceInfo, 0, len(orders))
	for _, o := range orders {
		blk := blocks[o]
		mac := blk["hwaddr"]
		if mac == "" {
			mac = NICMAC(name, o)
		}
		b := bindingByOrder[o]
		info := LXCInterfaceInfo{
			Order: o, IsPrimary: o == 0, MAC: mac, Link: blk["link"],
			SwitchID: b.SwitchID, SecurityGroupID: b.SecurityGroupID,
			BandwidthInboundAvg: b.BandwidthInboundAvg, BandwidthOutboundAvg: b.BandwidthOutboundAvg,
		}
		if varSw, err := lookupSwitch(b.SwitchID); err == nil {
			info.SwitchName = varSw.Name
			info.BridgeMode = varSw.BridgeMode
			info.CIDR = varSw.CIDR
			info.VLANID = varSw.VLANID
		}
		info.SecurityGroupName = lookupSGName(b.SecurityGroupID)
		if veth := findContainerHostVeth(name, o); veth != "" {
			info.Veth = veth
			rx, tx := ReadVethCounters(veth)
			info.RXBytes, info.TXBytes = rx, tx
		}
		if o == 0 {
			info.IP = ip
		}
		out = append(out, info)
	}
	return out, nil
}

// AddContainerInterface 给容器追加一块网卡（已停：仅写 config+绑定；运行中：热插拔）。
func AddContainerInterface(name string, req AddLXCInterfaceRequest) error {
	if req.SwitchID == 0 {
		return fmt.Errorf("必须选择 VPC 交换机")
	}
	sw, err := lookupSwitch(req.SwitchID)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(configPath(name))
	if err != nil {
		return fmt.Errorf("读取容器 config 失败: %w", err)
	}
	other, blocks := SplitNICBlocks(string(data))
	if len(blocks) >= maxLXCInterfaces {
		return fmt.Errorf("网卡数量已达上限 %d", maxLXCInterfaces)
	}
	next := nextOrder(blocks)
	bridge := sw.BridgeName
	if strings.TrimSpace(bridge) == "" {
		bridge = defaultBridge()
	}
	blocks[next] = map[string]string{
		"type":   "veth",
		"link":   bridge,
		"hwaddr": NICMAC(name, next),
		"flags":  "up",
	}
	binding := model.VPCVMBinding{
		VMName: name, Username: ownerOf(name), SwitchID: req.SwitchID,
		SecurityGroupID: req.SecurityGroupID, InterfaceOrder: next, Kind: "lxc",
		BandwidthInboundAvg: req.BandwidthInboundAvg, BandwidthOutboundAvg: req.BandwidthOutboundAvg,
	}
	if err := model.DB.Where("vm_name = ? AND interface_order = ? AND kind = ?", name, next, "lxc").
		Assign(binding).FirstOrCreate(&binding).Error; err != nil {
		return fmt.Errorf("写入 VPC 绑定失败: %w", err)
	}
	if err := writeConfig(name, other, blocks); err != nil {
		// 回滚刚写入的绑定，避免悬挂的 config 缺失
		model.DB.Where("vm_name = ? AND interface_order = ? AND kind = ?", name, next, "lxc").
			Delete(&model.VPCVMBinding{})
		return err
	}
	// 运行中：热插拔；已停：下次启动由 ReconcileContainerNICs 施加
	if containerRunning(name) {
		return hotplugNic(name, next, sw)
	}
	return nil
}

// UpdateContainerInterface 编辑某网卡（换交换机/限速/安全组）。MAC 不变。
func UpdateContainerInterface(name string, order int, req AddLXCInterfaceRequest) error {
	sw, err := lookupSwitch(req.SwitchID)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(configPath(name))
	if err != nil {
		return err
	}
	other, blocks := SplitNICBlocks(string(data))
	if blocks[order] == nil {
		return fmt.Errorf("网卡 order=%d 不存在", order)
	}
	// Fix A: order 0 主网卡换交换机时，若旧交换机已按 MAC 绑定静态 IP，阻止切换 ——
	// 旧 dhcp-hosts-<oldSwitchID> 条目不会随切换迁移/清理，会留下孤立条目导致容器丢静态 IP。
	if order == 0 {
		var cur model.VPCVMBinding
		model.DB.Where("vm_name = ? AND interface_order = ? AND kind = ?", name, 0, "lxc").First(&cur)
		if cur.SwitchID != req.SwitchID {
			mac := blocks[order]["hwaddr"]
			if mac == "" {
				mac = NICMAC(name, order)
			}
			if GetVPCStaticIPByMACExported(cur.SwitchID, mac) != "" {
				return fmt.Errorf("主网卡已绑定静态 IP，请先在网络 tab 解绑后再更换交换机")
			}
		}
	}
	bridge := sw.BridgeName
	if strings.TrimSpace(bridge) == "" {
		bridge = defaultBridge()
	}
	blocks[order]["link"] = bridge // 仅换 link，hwaddr/type/flags 保留
	// Fix B: 先更新 DB，再写 config；config 写失败时按 prior 回滚 binding 字段，
	// 避免出现「config 已写新桥 + binding 仍是旧交换机」的错位（下次 Reconcile 会用旧 VLAN/限速施加到新桥 veth）。
	var prior model.VPCVMBinding
	model.DB.Where("vm_name = ? AND interface_order = ? AND kind = ?", name, order, "lxc").First(&prior)
	updates := map[string]interface{}{
		"switch_id": req.SwitchID, "security_group_id": req.SecurityGroupID,
		"bandwidth_inbound_avg": req.BandwidthInboundAvg, "bandwidth_outbound_avg": req.BandwidthOutboundAvg,
	}
	if err := model.DB.Model(&model.VPCVMBinding{}).
		Where("vm_name = ? AND interface_order = ? AND kind = ?", name, order, "lxc").Updates(updates).Error; err != nil {
		return err
	}
	if err := writeConfig(name, other, blocks); err != nil {
		// 配置写失败：best-effort 把 binding 字段回滚到 prior，与未变更的 config 保持一致
		model.DB.Model(&model.VPCVMBinding{}).
			Where("vm_name = ? AND interface_order = ? AND kind = ?", name, order, "lxc").
			Updates(map[string]interface{}{
				"switch_id": prior.SwitchID, "security_group_id": prior.SecurityGroupID,
				"bandwidth_inbound_avg": prior.BandwidthInboundAvg, "bandwidth_outbound_avg": prior.BandwidthOutboundAvg,
			})
		return err
	}
	// 重新施加运行态（VLAN/限速随交换机变化）
	var b model.VPCVMBinding
	model.DB.Where("vm_name = ? AND interface_order = ? AND kind = ?", name, order, "lxc").First(&b)
	if containerRunning(name) {
		return applyNicRuntime(name, order, sw, b)
	}
	return nil
}

// RemoveContainerInterface 删除某网卡并重排索引。order==0 主卡需 force=true。
func RemoveContainerInterface(name string, order int, force bool) error {
	data, err := os.ReadFile(configPath(name))
	if err != nil {
		return err
	}
	other, blocks := SplitNICBlocks(string(data))
	if blocks[order] == nil {
		return fmt.Errorf("网卡 order=%d 不存在", order)
	}
	mac := blocks[order]["hwaddr"]
	if mac == "" {
		mac = NICMAC(name, order)
	}
	// Fix C: 在 delete/compact 前深拷贝 blocks，作为事务失败时 best-effort 回滚 config 的 pre-compaction 快照。
	// （other 不会被改动，复用同一变量即可。）
	origBlocks := make(map[int]map[string]string, len(blocks))
	for o, blk := range blocks {
		cp := make(map[string]string, len(blk))
		for k, v := range blk {
			cp[k] = v
		}
		origBlocks[o] = cp
	}
	// 已绑静态 IP 的卡：必须先解绑（直接按 binding.SwitchID 查，避免 switch 行缺失时绕过校验）
	var b model.VPCVMBinding
	model.DB.Where("vm_name = ? AND interface_order = ? AND kind = ?", name, order, "lxc").First(&b)
	if b.SwitchID != 0 && mac != "" {
		if GetVPCStaticIPByMACExported(b.SwitchID, mac) != "" {
			return fmt.Errorf("该网卡已绑定静态 IP，请先在网络 tab 解绑后再删除")
		}
	}
	if order == 0 && !force {
		return fmt.Errorf("主网卡需二次确认（force=true）方可删除")
	}
	// 运行中：热拔
	if containerRunning(name) {
		hotunplugNic(name, order)
	}
	delete(blocks, order)
	blocks = CompactNICBlocks(blocks)
	if err := writeConfig(name, other, blocks); err != nil {
		return err
	}
	// 删旧绑定 + 重排其余绑定 interface_order（事务内原子完成）
	var all []model.VPCVMBinding
	model.DB.Where("vm_name = ? AND kind = ?", name, "lxc").Order("interface_order ASC").Find(&all)
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("vm_name = ? AND interface_order = ? AND kind = ?", name, order, "lxc").
			Delete(&model.VPCVMBinding{}).Error; err != nil {
			return err
		}
		newIdx := 0
		for _, bb := range all {
			if bb.InterfaceOrder == order {
				continue
			}
			if bb.InterfaceOrder != newIdx {
				if err := tx.Model(&model.VPCVMBinding{}).Where("id = ?", bb.ID).
					Update("interface_order", newIdx).Error; err != nil {
					return err
				}
			}
			newIdx++
		}
		return nil
	}); err != nil {
		// 事务失败：binding 未变更，best-effort 把 config 回滚到 pre-compaction 状态以保持一致
		writeConfig(name, other, origBlocks)
		return fmt.Errorf("重排网卡绑定失败: %w", err)
	}
	return nil
}

func lookupSwitch(id uint) (model.VPCSwitch, error) {
	var sw model.VPCSwitch
	if err := model.DB.First(&sw, id).Error; err != nil {
		return sw, fmt.Errorf("交换机不存在: %w", err)
	}
	return sw, nil
}
func lookupSGName(id uint) string {
	if id == 0 {
		return ""
	}
	var sg model.VPCSecurityGroup
	if err := model.DB.First(&sg, id).Error; err != nil {
		return ""
	}
	return sg.Name
}
func defaultBridge() string {
	if b := config.GlobalConfig.OVSBridge; b != "" {
		return b
	}
	return "br-ovs"
}
func nextOrder(blocks map[int]map[string]string) int {
	max := -1
	for o := range blocks {
		if o > max {
			max = o
		}
	}
	return max + 1
}
func writeConfig(name, other string, blocks map[int]map[string]string) error {
	return os.WriteFile(configPath(name), []byte(other+RenderNICBlocks(blocks)), 0644)
}
func containerRunning(name string) bool {
	r := LxcInfo(name)
	if r.ExitCode != 0 {
		return false
	}
	d, _ := ParseLxcInfo(r.Stdout)
	return strings.Contains(strings.ToUpper(d.Status), "RUNNING")
}

// hotplugNic / hotunplugNic：运行中容器的 veth 热加/热拔（best-effort，失败返回 needs_restart 语义见 handler）。
func hotplugNic(name string, order int, sw model.VPCSwitch) error {
	pid := containerPID(name)
	if pid == "" {
		return fmt.Errorf("无法获取容器 PID，请重启容器使配置生效")
	}
	a := fmt.Sprintf("vxc%d_%s_a", order, name)
	c := fmt.Sprintf("vxc%d_%s_b", order, name)
	if r := utils.ExecCommand("ip", "link", "add", a, "type", "veth", "peer", "name", c); r.Error != nil {
		return fmt.Errorf("热插拔建 veth 失败，请重启容器: %s", r.Stderr)
	}
	utils.ExecCommandQuiet("ip", "link", "set", c, "netns", pid)
	// a/c derive from validated container name → injection-safe by construction
	utils.ExecShell(fmt.Sprintf("lxc-attach -n %s -- sh -c 'ip link set %s name eth%d; ip link set eth%d up' 2>/dev/null",
		utils.ShellSingleQuote(name), c, order+1, order+1))
	bridge := sw.BridgeName
	if strings.TrimSpace(bridge) == "" {
		bridge = defaultBridge()
	}
	utils.ExecCommandQuiet("ovs-vsctl", "--may-exist", "add-port", bridge, a)
	if sw.VLANID > 0 {
		utils.ExecCommandQuiet("ovs-vsctl", "set", "Port", a, fmt.Sprintf("tag=%d", sw.VLANID))
	}
	// 注：热插拔的 veth host 侧 MAC 为随机值，与 lxc.net.N.hwaddr 不一致；
	//     后续 ReconcileContainerNICs 仍按 config MAC 找不到该 veth，故热插拔为「临时生效」，
	//     持久态依赖下次重启由 lxc-start 按 config 重建。引导用户重启以获一致状态。
	return nil
}
func hotunplugNic(name string, order int) {
	veth := findContainerHostVeth(name, order)
	if veth == "" {
		return
	}
	utils.ExecCommandQuiet("ovs-vsctl", "--if-exists", "del-port", defaultBridge(), veth)
	utils.ExecCommandQuiet("ip", "link", "del", veth)
}
func containerPID(name string) string {
	r := utils.ExecCommand("lxc-info", "-n", name, "-p")
	if r.Error != nil {
		return ""
	}
	for _, line := range strings.Split(r.Stdout, "\n") {
		if strings.Contains(line, "PID:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				return f[len(f)-1]
			}
		}
	}
	return ""
}

// GetVPCStaticIPByMACExported 读交换机 dhcp-hosts 文件，返回 MAC 对应已绑 IP（空=未绑）。
func GetVPCStaticIPByMACExported(switchID uint, mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	if mac == "" {
		return ""
	}
	b, err := os.ReadFile(fmt.Sprintf("/etc/kvm-console/vpc/dhcp-hosts-%d", switchID))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) >= 2 && strings.EqualFold(strings.TrimSpace(fields[0]), mac) {
			return strings.TrimSpace(fields[1])
		}
	}
	return ""
}
