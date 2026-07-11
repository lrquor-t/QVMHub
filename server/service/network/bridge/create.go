package bridge

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"qvmhub/model"
	ovspkg "qvmhub/service/ovs"
	"qvmhub/utils"
)

func CreateNetworkBridge(req NetworkBridgeRequest) (*model.NetworkBridge, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Mode = NormalizeBridgeMode(req.Mode)
	req.UplinkIF = strings.TrimSpace(req.UplinkIF)
	if req.Mode != BridgeModeDirect {
		return nil, fmt.Errorf("当前仅允许创建桥接直通网桥")
	}
	if err := validateBridgeName(req.Name); err != nil {
		return nil, err
	}
	if req.Name == ovspkg.OvsBridgeName() {
		return nil, fmt.Errorf("默认 OVS 内网网桥已存在，不能重复创建")
	}
	if req.UplinkIF == "" {
		return nil, fmt.Errorf("请选择物理网卡")
	}
	if err := validateBridgeUplink(req.UplinkIF, req.Name); err != nil {
		return nil, err
	}
	if model.DB != nil {
		var count int64
		model.DB.Model(&model.NetworkBridge{}).Where("name = ?", req.Name).Count(&count)
		if count > 0 {
			return nil, fmt.Errorf("网桥名称已存在")
		}
	}
	// 创建前捕获物理网卡当前 IP 配置（必须在加入 OVS 之前）
	var ipCfg HostIPConfig
	if req.MigrateHostIP {
		ipCfg = CaptureInterfaceIPv4(req.UplinkIF)
	}
	if err := EnsureOVSBridgeDirect(req.Name, req.UplinkIF, req.MigrateHostIP, ipCfg); err != nil {
		return nil, err
	}
	row := &model.NetworkBridge{
		Name: req.Name, Mode: BridgeModeDirect, UplinkIF: req.UplinkIF,
		MigrateHostIP: req.MigrateHostIP,
		HostAddrs:     ipCfg.Addrs, HostGateway: ipCfg.Gateway, HostMetric: ipCfg.Metric, HostDNS: ipCfg.DNS,
	}
	if model.DB != nil {
		if err := model.DB.Create(row).Error; err != nil {
			return nil, fmt.Errorf("保存网桥配置失败: %w", err)
		}
		if HookEnsureOVSNetworkReady != nil {
			if err := HookEnsureOVSNetworkReady(); err != nil {
				return nil, fmt.Errorf("网桥已创建，但恢复默认 OVS 网络失败: %w", err)
			}
		}
		if HookEnsureAllVPCSwitchRuntime != nil {
			if err := HookEnsureAllVPCSwitchRuntime(); err != nil {
				return nil, fmt.Errorf("网桥已创建，但恢复 VPC 交换机网络失败: %w", err)
			}
		}
	}
	return row, nil
}

func EnsureOVSBridgeDirect(bridge, uplink string, migrateHostIP bool, cfg HostIPConfig) error {
	if result := utils.ExecCommand("bash", "-c", "command -v ovs-vsctl"); result.Error != nil {
		return fmt.Errorf("OVS 未安装，请先安装 openvswitch-switch")
	}
	bridge = strings.TrimSpace(bridge)
	uplink = strings.TrimSpace(uplink)
	if err := os.MkdirAll(bridgeConfigDir, 0755); err != nil {
		return fmt.Errorf("创建网桥配置目录失败: %w", err)
	}
	if result := utils.ExecCommand("ovs-vsctl", "--may-exist", "add-br", bridge); result.Error != nil {
		return fmt.Errorf("创建桥接网桥失败: %s", result.Stderr)
	}
	utils.ExecCommand("ip", "link", "set", bridge, "up")
	if result := utils.ExecCommand("ovs-vsctl", "--may-exist", "add-port", bridge, uplink); result.Error != nil {
		return fmt.Errorf("添加物理网卡到桥接网桥失败: %s", result.Stderr)
	}
	utils.ExecCommand("ip", "link", "set", uplink, "up")
	// IP 迁移逻辑
	if migrateHostIP {
		// 检查网桥是否已有 IP（重启恢复场景：systemd 服务已应用了静态 IP）
		bridgeCfg := CaptureInterfaceIPv4(bridge)
		bridgeHasIP := strings.TrimSpace(bridgeCfg.Addrs) != ""
		if !bridgeHasIP {
			// 网桥没有 IP，尝试从物理口迁移或使用存储值
			uplinkCfg := CaptureInterfaceIPv4(uplink)
			if strings.TrimSpace(uplinkCfg.Addrs) != "" {
				// 物理口有 IP，执行动态迁移
				migrateInterfaceIPv4ToBridge(uplink, bridge)
			} else if strings.TrimSpace(cfg.Addrs) != "" {
				// 物理口也没 IP，使用存储的静态配置恢复
				applyStaticIPv4ToBridge(bridge, cfg)
			}
		}
		// DNS 总是需要确保配置正确（重启恢复场景下即使网桥已有 IP，DNS 也可能丢失）
		ensureBridgeResolvedDNSWithStatic(uplink, bridge, cfg.DNS)
		// 如果 cfg 为空但网桥已有 IP，更新 cfg 用于写入脚本
		if strings.TrimSpace(cfg.Addrs) == "" {
			cfg = CaptureInterfaceIPv4(bridge)
			// 同时保留已有的 DNS 信息
			if cfg.DNS == "" {
				cfg.DNS = captureInterfaceDNSServers(bridge)
			}
		}
		// 兼容旧记录：IP 已存储但 DNS 未存储，从网桥当前状态捕获 DNS
		if strings.TrimSpace(cfg.DNS) == "" {
			cfg.DNS = captureInterfaceDNSServers(bridge)
			// 网桥也没有则回退到 uplink
			if cfg.DNS == "" {
				cfg.DNS = captureInterfaceDNSServers(uplink)
			}
		}
	}
	// IP 已迁移完成后再禁用 networkd DHCP，避免周期性 DHCP Discover 干扰 OVS 数据通道
	disableNetworkdDHCPForPort(uplink)
	if err := writeBridgeRestoreScript(bridge, uplink, migrateHostIP, cfg); err != nil {
		return err
	}
	if err := writeBridgeRestoreUnit(); err != nil {
		return err
	}
	return nil
}

func validateBridgeName(name string) error {
	if name == "" {
		return fmt.Errorf("网桥名称不能为空")
	}
	if len(name) > 15 {
		return fmt.Errorf("网桥名称不能超过 15 个字符")
	}
	if ok, _ := regexp.MatchString(`^[A-Za-z0-9_.-]+$`, name); !ok {
		return fmt.Errorf("网桥名称只能包含字母、数字、点、下划线和短横线")
	}
	return nil
}

func validateBridgeUplink(uplink, targetBridge string) error {
	if !isPhysicalInterface(uplink) {
		return fmt.Errorf("请选择真实物理网卡")
	}
	ports := readOVSPortBridgeMap()
	if bridge := ports[uplink]; bridge != "" && bridge != targetBridge {
		return fmt.Errorf("物理网卡 %s 已接入 OVS 网桥 %s", uplink, bridge)
	}
	if model.DB != nil {
		var count int64
		model.DB.Model(&model.NetworkBridge{}).Where("uplink_if = ?", uplink).Count(&count)
		if count > 0 {
			return fmt.Errorf("物理网卡 %s 已被其它桥接网桥使用", uplink)
		}
	}
	return nil
}

func ovsBridgeExists(name string) bool {
	return utils.ExecCommand("ovs-vsctl", "br-exists", strings.TrimSpace(name)).Error == nil
}

func linkIsUp(name string) bool {
	result := utils.ExecCommand("ip", "-j", "link", "show", "dev", strings.TrimSpace(name))
	return result.Error == nil && strings.Contains(strings.ToUpper(result.Stdout), "UP")
}
