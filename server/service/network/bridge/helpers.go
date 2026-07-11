package bridge

import (
	"strings"

	"qvmhub/model"
	ovspkg "qvmhub/service/ovs"
)

func BridgeModeForSwitch(sw model.VPCSwitch) string {
	mode := NormalizeBridgeMode(sw.BridgeMode)
	if mode == "" {
		mode = BridgeModeNAT
	}
	return mode
}

func BridgeNameForSwitch(sw model.VPCSwitch) string {
	if strings.TrimSpace(sw.BridgeName) != "" {
		return strings.TrimSpace(sw.BridgeName)
	}
	return ovspkg.OvsBridgeName()
}

func SwitchUsesDirectBridge(sw model.VPCSwitch) bool {
	return BridgeModeForSwitch(sw) == BridgeModeDirect
}

func NormalizeBridgeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return BridgeModeNAT
	}
	return mode
}
