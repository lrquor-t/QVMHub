package vnc

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/digitalocean/go-libvirt"

	"qvmhub/logger"
	"qvmhub/service/libvirt_rpc"
	"qvmhub/service/vm_xml"
)

// GetVncStatus 获取 VNC 状态
func GetVncStatus(vmName string) (*VncInfo, error) {
	info := &VncInfo{}

	vmState, err := libvirt_rpc.GetDomainStateRPC(vmName)
	if err != nil {
		return nil, fmt.Errorf("虚拟机不存在: %s", vmName)
	}

	if vmState != "running" && vmState != "paused" {
		// 从 XML 配置检查是否有 VNC
		xmlStr, xmlErr := libvirt_rpc.GetDomainXMLRPC(vmName, libvirt.DomainXMLInactive)
		if xmlErr == nil {
			info.Enabled = strings.Contains(xmlStr, "graphics type='vnc'")
		}
		return info, nil
	}

	// 运行中，通过 QEMU Monitor 获取详情
	vncOutput, vncErr := libvirt_rpc.QemuMonitorCommandRPC(vmName, "info vnc", libvirt_rpc.DomainQemuMonitorCommandHmp)
	if vncErr == nil {
		info.Enabled = strings.Contains(vncOutput, "Server:")

		// 解析端口和监听地址
		for _, line := range strings.Split(vncOutput, "\n") {
			if strings.Contains(line, "Server:") {
				parts := strings.Split(line, ":")
				if len(parts) >= 3 {
					info.Port = strings.TrimSpace(parts[len(parts)-1])
				}
				// 检测是否对外暴露（监听 0.0.0.0）
				if strings.Contains(line, "0.0.0.0") {
					info.Exposed = true
				}
			}
			if strings.Contains(line, "Auth:") {
				authParts := strings.SplitN(line, ":", 2)
				if len(authParts) >= 2 {
					info.Auth = strings.TrimSpace(authParts[1])
					info.Password = info.Auth == "vnc"
				}
			}
		}
	} else {
		// 从 XML 检查暴露状态
		xmlStr, xmlErr := libvirt_rpc.GetDomainXMLRPC(vmName, libvirt.DomainXMLInactive)
		if xmlErr == nil && strings.Contains(xmlStr, "listen='0.0.0.0'") {
			info.Exposed = true
		}
	}

	return info, nil
}

// vncGraphicsBlockRe 匹配 <graphics type='vnc' ...>...</graphics> 整块
var vncGraphicsBlockRe = regexp.MustCompile(`(?s)<graphics\s+type='vnc'[^>]*>.*?</graphics>`)

// replaceVncGraphicsBlock 替换 XML 中的 VNC graphics 块
// 如果存在则替换，不存在则插入到 </devices> 前
func replaceVncGraphicsBlock(xmlStr, newBlock string) string {
	if vncGraphicsBlockRe.MatchString(xmlStr) {
		return vncGraphicsBlockRe.ReplaceAllString(xmlStr, newBlock)
	}
	// 新增 VNC 配置：插入到 </devices> 前
	return strings.Replace(xmlStr, "</devices>", newBlock+"\n    </devices>", 1)
}

// getVMXMLForVnc 获取 VM XML（运行中取活跃配置，否则取 inactive 配置）
func getVMXMLForVnc(vmName, vmState string) (string, error) {
	if vmState == "running" {
		return libvirt_rpc.GetDomainXMLRPC(vmName, 0)
	}
	return libvirt_rpc.GetDomainXMLRPC(vmName, libvirt.DomainXMLInactive)
}

// applyVncXMLAndRestart 应用修改后的 XML 配置，若 VM 原本运行则先 destroy 再 define 再启动
func applyVncXMLAndRestart(vmName, vmState, newXML string) error {
	if vmState == "running" {
		if err := libvirt_rpc.DestroyDomainRPC(vmName); err != nil {
			logger.Libvirt.Warn("VNC配置: 关闭虚拟机失败(可能已关闭)", "error", err)
		}

		if _, err := libvirt_rpc.DefineDomainXMLRPC(newXML); err != nil {
			return fmt.Errorf("VNC配置: 应用XML定义失败: %w", err)
		}
		if err := HookStartVM(vmName); err != nil {
			return err
		}
	} else {
		if _, err := libvirt_rpc.DefineDomainXMLRPC(newXML); err != nil {
			return fmt.Errorf("VNC配置: 应用XML定义失败: %w", err)
		}
	}
	return nil
}

// EnableVnc 开启 VNC（使用 TCP 本地模式，绑定 127.0.0.1 仅本机可访问）
func EnableVnc(vmName, password string) error {
	vmState, err := libvirt_rpc.GetDomainStateRPC(vmName)
	if err != nil {
		return fmt.Errorf("虚拟机不存在: %s", vmName)
	}

	// 获取当前 XML
	xmlStr, err := getVMXMLForVnc(vmName, vmState)
	if err != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %w", err)
	}

	// 构建 graphics 属性（TCP 绑定 127.0.0.1，仅本机可访问，通过后端 WebSocket 代理）
	var graphicsXML string
	if password != "" {
		graphicsXML = fmt.Sprintf(
			"<graphics type='vnc' port='-1' autoport='yes' listen='127.0.0.1' passwd='%s'>\n      <listen type='address' address='127.0.0.1'/>\n    </graphics>",
			password)
	} else {
		graphicsXML = "<graphics type='vnc' port='-1' autoport='yes' listen='127.0.0.1'>\n      <listen type='address' address='127.0.0.1'/>\n    </graphics>"
	}

	// 替换或新增 VNC 配置
	xmlStr = replaceVncGraphicsBlock(xmlStr, graphicsXML)

	// 保留已有视频模型；如果当前没有视频设备，则按系统类型补一个默认值
	detectedOSType := HookDetectVMOSType("", xmlStr)
	xmlStr = vm_xml.ApplyVMVideoModelToDomainXML(xmlStr, vm_xml.ParseVMVideoModelFromDomainXML(xmlStr), detectedOSType)

	// 应用配置
	return applyVncXMLAndRestart(vmName, vmState, xmlStr)
}

// DisableVnc 关闭 VNC
func DisableVnc(vmName string) error {
	vmState, err := libvirt_rpc.GetDomainStateRPC(vmName)
	if err != nil {
		return fmt.Errorf("虚拟机不存在: %s", vmName)
	}

	// 获取当前 XML
	xmlStr, err := getVMXMLForVnc(vmName, vmState)
	if err != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %w", err)
	}

	// 替换为仅本地监听
	newGraphics := "<graphics type='vnc' port='-1' autoport='yes' listen='127.0.0.1'>\n      <listen type='address' address='127.0.0.1'/>\n    </graphics>"
	xmlStr = replaceVncGraphicsBlock(xmlStr, newGraphics)

	// 应用配置
	return applyVncXMLAndRestart(vmName, vmState, xmlStr)
}

// vncPasswdRe 匹配 passwd='...' 属性
var vncPasswdRe = regexp.MustCompile(`passwd='[^']*'`)

// ChangeVncPassword 修改 VNC 密码（热修改，无需重启）
func ChangeVncPassword(vmName, newPassword string) error {
	vmState, err := libvirt_rpc.GetDomainStateRPC(vmName)
	if err != nil {
		return fmt.Errorf("虚拟机不存在: %s", vmName)
	}
	if vmState != "running" {
		return fmt.Errorf("虚拟机未运行，无法热修改密码")
	}

	if len(newPassword) > 8 {
		newPassword = newPassword[:8]
	}

	// 热修改密码
	_, err = libvirt_rpc.QemuMonitorCommandRPC(vmName, fmt.Sprintf("set_password vnc %s", newPassword), libvirt_rpc.DomainQemuMonitorCommandHmp)
	if err != nil {
		return fmt.Errorf("修改密码失败: %w", err)
	}

	// 同步持久化配置
	xmlStr, xmlErr := libvirt_rpc.GetDomainXMLRPC(vmName, 0)
	if xmlErr == nil {
		updatedXML := vncPasswdRe.ReplaceAllString(xmlStr, fmt.Sprintf("passwd='%s'", newPassword))
		if _, defineErr := libvirt_rpc.DefineDomainXMLRPC(updatedXML); defineErr != nil {
			logger.Libvirt.Warn("VNC配置: 持久化密码到XML定义失败", "error", defineErr)
		}
	}

	return nil
}

// GetVncConnInfo 获取 VNC 连接信息（自动检测 Unix Socket / TCP 模式）
func GetVncConnInfo(vmName string) (*VncConnInfo, error) {
	// 1. 先从 XML 尝试获取 socket 路径
	xmlStr, err := libvirt_rpc.GetDomainXMLRPC(vmName, 0)
	if err != nil {
		return nil, fmt.Errorf("获取 XML 失败: %w", err)
	}

	// 尝试解析 socket 路径
	for _, line := range strings.Split(xmlStr, "\n") {
		if strings.Contains(line, "socket=") && strings.Contains(line, "vnc") {
			start := strings.Index(line, "socket='")
			if start >= 0 {
				start += len("socket='")
				end := strings.Index(line[start:], "'")
				if end >= 0 {
					socketPath := line[start : start+end]
					// 检查 socket 文件是否存在
					if info, statErr := os.Stat(socketPath); statErr == nil && info.Mode()&os.ModeSocket != 0 {
						return &VncConnInfo{Network: "unix", Address: socketPath}, nil
					}
				}
			}
		}
	}

	// 2. Socket 不可用，尝试从 QEMU Monitor 获取 TCP 地址
	vncOutput, vncErr := libvirt_rpc.QemuMonitorCommandRPC(vmName, "info vnc", libvirt_rpc.DomainQemuMonitorCommandHmp)
	if vncErr == nil {
		for _, line := range strings.Split(vncOutput, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Server:") {
				// 格式形如 "Server: 127.0.0.1:5900 (ipv4)" 或 "Server: [::]:5900 (ipv6)"
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					addr := parts[1]
					// 去掉可能的 (ipv4)/(ipv6) 后缀
					addr = strings.TrimSpace(addr)
					if addr != "" && addr != "none" {
						return &VncConnInfo{Network: "tcp", Address: addr}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("未找到可用的 VNC 连接（请先开启 VNC）")
}

// vncListenAttrRe 匹配 graphics type='vnc' 标签上的 listen='...' 属性
var vncListenAttrRe = regexp.MustCompile(`(<graphics\s+type='vnc'[^>]*?)listen='[^']*'`)

// vncListenAddressRe 匹配 <listen type='address' address='...'/> 标签
var vncListenAddressRe = regexp.MustCompile(`(<listen\s+type='address'[^>]*?)address='[^']*'`)

// ExposeVnc 切换 VNC 对外暴露状态（需重启虚拟机生效）
// expose=true: 监听 0.0.0.0（对外暴露）; expose=false: 监听 127.0.0.1（仅本地）
func ExposeVnc(vmName string, expose bool) error {
	vmState, err := libvirt_rpc.GetDomainStateRPC(vmName)
	if err != nil {
		return fmt.Errorf("虚拟机不存在: %s", vmName)
	}

	// 获取当前 XML
	xmlStr, err := getVMXMLForVnc(vmName, vmState)
	if err != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %w", err)
	}

	// 检查是否有 VNC 配置
	if !strings.Contains(xmlStr, "graphics type='vnc'") {
		return fmt.Errorf("VNC 未开启，请先开启 VNC")
	}

	var listenAddr string
	if expose {
		listenAddr = "0.0.0.0"
	} else {
		listenAddr = "127.0.0.1"
	}

	// 替换 listen 地址（仅修改 graphics 和 listen 行，避免误改 mac address 等）
	xmlStr = vncListenAttrRe.ReplaceAllString(xmlStr, fmt.Sprintf("${1}listen='%s'", listenAddr))
	xmlStr = vncListenAddressRe.ReplaceAllString(xmlStr, fmt.Sprintf("${1}address='%s'", listenAddr))

	// 应用配置
	return applyVncXMLAndRestart(vmName, vmState, xmlStr)
}
