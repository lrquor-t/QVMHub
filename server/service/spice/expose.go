package spice

import (
	"fmt"
	"strings"

	"qvmhub/logger"
	"qvmhub/service/libvirt_rpc"
)

// expose.go — SPICE 公网暴露与防火墙联动、连接信息解析。
// 与 VNC 的关键差异：SPICE 不走面板 WS 代理、由客户端直连宿主端口，
// 因此对外暴露必须放行宿主防火墙对应端口；收回时删除规则。

// ExposeSpice 切换 SPICE 对外暴露状态（需重启虚拟机生效）。
//   - expose=true:  监听 0.0.0.0 并 ufw 放行 spice 端口
//   - expose=false: 监听切回 127.0.0.1 并收回防火墙规则
func ExposeSpice(vmName string, expose bool) error {
	vmState, err := libvirt_rpc.GetDomainStateRPC(vmName)
	if err != nil {
		return fmt.Errorf("虚拟机不存在: %s", vmName)
	}

	xmlStr, err := getVMXMLForSpice(vmName, vmState)
	if err != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %w", err)
	}

	if !HasSPICEGraphics(xmlStr) {
		return fmt.Errorf("SPICE 未开启，请先开启 SPICE")
	}

	// VM 当前若在运行，先读出旧端口以便稍后收回防火墙规则
	var oldPort string
	if vmState == "running" || vmState == "paused" {
		if p, pErr := detectSpicePort(vmName); pErr == nil {
			oldPort = p
		}
	}

	listenAddr := "127.0.0.1"
	if expose {
		listenAddr = "0.0.0.0"
	}
	xmlStr = SetSPICEListenAddress(xmlStr, listenAddr)

	// 应用配置（关机直接 define；运行中 destroy→define→start）
	if err := applySpiceXMLAndRestart(vmName, vmState, xmlStr); err != nil {
		return err
	}

	if expose {
		// 重启后端口可能变化，按新端口补防火墙
		if port, pErr := detectSpicePort(vmName); pErr == nil && port != "" {
			if err := allowSpiceFirewall(port); err != nil {
				logger.Libvirt.Warn("SPICE暴露: 放行防火墙端口失败", "vm", vmName, "port", port, "error", err)
			}
		}
	} else if oldPort != "" {
		// 收回旧端口防火墙
		if err := revokeSpiceFirewall(oldPort); err != nil {
			logger.Libvirt.Warn("SPICE收回: 删除防火墙端口失败", "vm", vmName, "port", oldPort, "error", err)
		}
	}
	return nil
}

// detectSpicePort 通过 QEMU monitor "info spice" 读回实际端口。
func detectSpicePort(vmName string) (string, error) {
	out, err := libvirt_rpc.QemuMonitorCommandRPC(vmName, "info spice", libvirt_rpc.DomainQemuMonitorCommandHmp)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "address:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if idx := strings.LastIndex(fields[1], ":"); idx >= 0 {
					return fields[1][idx+1:], nil
				}
			}
		}
	}
	return "", fmt.Errorf("未从 info spice 解析到端口")
}

// allowSpiceFirewall / revokeSpiceFirewall 通过 HookManageUFWRule 操作 ufw。
func allowSpiceFirewall(port string) error {
	if HookManageUFWRule == nil {
		return nil
	}
	return HookManageUFWRule("allow", port+"/tcp")
}

func revokeSpiceFirewall(port string) error {
	if HookManageUFWRule == nil || port == "" {
		return nil
	}
	return HookManageUFWRule("delete", port+"/tcp")
}

// GetSpiceConnInfo 解析对外连接信息（host + port + 密码），供 .vv 文件使用。
func GetSpiceConnInfo(vmName string) (*SpiceConnInfo, error) {
	xmlStr, err := libvirt_rpc.GetDomainXMLRPC(vmName, 0)
	if err != nil {
		return nil, fmt.Errorf("获取 XML 失败: %w", err)
	}
	if !HasSPICEGraphics(xmlStr) {
		return nil, fmt.Errorf("SPICE 未开启，请先开启 SPICE")
	}

	info := &SpiceConnInfo{}
	block := spiceGraphicsBlockRe.FindString(xmlStr)
	if block != "" {
		info.Password = extractSPICEPasswd(block)
		if strings.Contains(block, "0.0.0.0") {
			info.Exposed = true
		}
	}

	if port, pErr := detectSpicePort(vmName); pErr == nil {
		info.Port = port
	}

	// 对外地址：优先配置的公网 IP
	if HookGetHostIP != nil {
		info.Host = HookGetHostIP()
	}
	if info.Host == "" {
		info.Host = "127.0.0.1"
	}
	if !info.Exposed {
		info.Host = "127.0.0.1"
	}
	return info, nil
}
