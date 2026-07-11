package public_ip

import "qvmhub/model"

const (
	PublicIPModeNAT           = "nat"
	PublicIPModeClassicRoute  = "classic_route"
	PublicIPModeClassicBridge = "classic_bridge"

	PublicIPStatusFree  = "free"
	PublicIPStatusBound = "bound"

	publicIPConfigDir   = "/etc/kvm-console/public-ip"
	publicIPRulesPath   = "/etc/kvm-console/public-ip/rules.sh"
	publicIPRuleComment = "kvm-console:public-ip"
	publicIPFlowPrefix  = "0x9a"
	publicIPFlowMask    = "0xff00000000000000"
)

type PublicIPRequest struct {
	IP             string `json:"ip"`
	CIDR           string `json:"cidr"`
	Gateway        string `json:"gateway"`
	UplinkIF       string `json:"uplink_if"`
	SupportedModes string `json:"supported_modes"`
	Status         string `json:"status"`
	Remark         string `json:"remark"`
}

type PublicIPBindRequest struct {
	Username    string `json:"username"`
	VMName      string `json:"vm_name"`
	VMPrivateIP string `json:"vm_private_ip"`
	Mode        string `json:"mode"`
}

type PublicIPOperationParams struct {
	Action      string              `json:"action"`
	PublicIPID  uint                `json:"public_ip_id"`
	TargetVM    string              `json:"target_vm"`
	TargetUser  string              `json:"target_user"`
	BindRequest PublicIPBindRequest `json:"bind_request"`
}

type PublicIPInfo struct {
	model.PublicIP
	Modes        []string               `json:"modes"`
	ModeLabels   []string               `json:"mode_labels"`
	Binding      *model.PublicIPBinding `json:"binding,omitempty"`
	RuntimeRules []string               `json:"runtime_rules,omitempty"`
	Issues       []string               `json:"issues,omitempty"`
}

type PublicIPPreview struct {
	PublicIP   model.PublicIP      `json:"public_ip"`
	Binding    PublicIPBindRequest `json:"binding"`
	Commands   []string            `json:"commands"`
	ConfigHint string              `json:"config_hint"`
	Warnings   []string            `json:"warnings"`
}

type PublicIPAttachment struct {
	PublicIP      string `json:"public_ip"`
	Mode          string `json:"mode"`
	ModeLabel     string `json:"mode_label"`
	VMPrivateIP   string `json:"vm_private_ip"`
	RuntimeStatus string `json:"runtime_status"`
}
