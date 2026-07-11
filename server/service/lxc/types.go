package lxc

import "errors"

// ErrInvalidName 表示传入的容器名称为空或非法。
var ErrInvalidName = errors.New("无效的容器名称")

// ContainerListItem 列表项（解析自 lxc-ls --fancy）。
type ContainerListItem struct {
	Name      string
	Status    string // RUNNING/STOPPED/FROZEN/...
	IPv4      string
	Autostart string // YES/NO
	Running   bool
}

// ContainerDetail 详情（解析自 lxc-info + config）。
type ContainerDetail struct {
	Name      string
	Status    string
	IP        string
	PID       string
	Arch      string
	Backing   string
	CPUShares int
	MemoryMB  int
	Autostart bool
}

// AddLXCInterfaceRequest 增/改 LXC 网卡入参。
type AddLXCInterfaceRequest struct {
	SwitchID             uint   `json:"switch_id" binding:"required"`
	SecurityGroupID      uint   `json:"security_group_id"`
	BandwidthInboundAvg  int    `json:"bandwidth_inbound_avg"` // Mbps，0=不限
	BandwidthOutboundAvg int    `json:"bandwidth_outbound_avg"`
}

// LXCInterfaceInfo 单张网卡视图（config + 绑定 + 运行态）。
type LXCInterfaceInfo struct {
	Order                int    `json:"order"`
	IsPrimary            bool   `json:"is_primary"`
	MAC                  string `json:"mac"`
	Link                 string `json:"link"`
	SwitchID             uint   `json:"switch_id"`
	SwitchName           string `json:"switch_name"`
	BridgeMode           string `json:"bridge_mode"`
	CIDR                 string `json:"cidr"`
	VLANID               int    `json:"vlan_id"`
	SecurityGroupID      uint   `json:"security_group_id"`
	SecurityGroupName    string `json:"security_group_name"`
	BandwidthInboundAvg  int    `json:"bandwidth_inbound_avg"`
	BandwidthOutboundAvg int    `json:"bandwidth_outbound_avg"`
	Veth                 string `json:"veth"`
	IP                   string `json:"ip"`
	RXBytes              int64  `json:"rx_bytes"`
	TXBytes              int64  `json:"tx_bytes"`
}
