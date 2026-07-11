package firewall

import (
	"fmt"
	"strings"
)

// SetPortForwardFirewallExemption 设置端口转发是否豁免入站区域限制。
func SetPortForwardFirewallExemption(key string, exempt bool) (*FirewallPolicy, error) {
	policy, err := GetFirewallPolicy()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("端口转发规则标识不能为空")
	}
	policy.PortForwardExemptions[key] = exempt
	if !exempt {
		delete(policy.PortForwardExemptions, key)
	}
	if err := SaveFirewallPolicy(policy); err != nil {
		return nil, err
	}
	return policy, nil
}

// ClearPortForwardFirewallExemption 清理已删除端口转发的区域限制豁免记录。
func ClearPortForwardFirewallExemption(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	policy, err := GetFirewallPolicy()
	if err != nil {
		return err
	}
	if policy.PortForwardExemptions == nil || !policy.PortForwardExemptions[key] {
		return nil
	}
	delete(policy.PortForwardExemptions, key)
	return SaveFirewallPolicy(policy)
}
