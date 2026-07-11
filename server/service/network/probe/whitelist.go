package probe

import (
	"fmt"
	"strings"

	"qvmhub/model"
	netpkg "qvmhub/service/network"
	fwpkg "qvmhub/service/firewall"
	vpcpkg "qvmhub/service/network/vpc"
)

func ListPortForwardWhitelists() (*PortForwardWhitelistList, error) {
	var rows []model.PortForwardWhitelist
	if err := model.DB.Order("scope_type ASC, scope_value ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := &PortForwardWhitelistList{
		Users: make([]model.PortForwardWhitelist, 0),
		VMs:   make([]model.PortForwardWhitelist, 0),
	}
	for _, row := range rows {
		switch strings.TrimSpace(row.ScopeType) {
		case model.PortForwardWhitelistScopeUser:
			result.Users = append(result.Users, row)
		case model.PortForwardWhitelistScopeVM:
			result.VMs = append(result.VMs, row)
		}
	}
	return result, nil
}

func AddPortForwardWhitelist(scopeType, scopeValue, createdBy string) (*model.PortForwardWhitelist, []string, error) {
	scopeType = strings.TrimSpace(scopeType)
	scopeValue = strings.TrimSpace(scopeValue)
	createdBy = strings.TrimSpace(createdBy)
	if createdBy == "" {
		createdBy = "admin"
	}
	if scopeType != model.PortForwardWhitelistScopeUser && scopeType != model.PortForwardWhitelistScopeVM {
		return nil, nil, fmt.Errorf("白名单类型无效")
	}
	if scopeValue == "" {
		return nil, nil, fmt.Errorf("白名单值不能为空")
	}
	if scopeType == model.PortForwardWhitelistScopeUser {
		var user model.User
		if err := model.DB.Where("username = ?", scopeValue).First(&user).Error; err != nil {
			return nil, nil, fmt.Errorf("用户不存在")
		}
	} else {
		if err := HookVMExists(scopeValue); err != nil {
			return nil, nil, fmt.Errorf("虚拟机不存在")
		}
	}

	row := &model.PortForwardWhitelist{
		ScopeType:  scopeType,
		ScopeValue: scopeValue,
		CreatedBy:  createdBy,
	}
	if err := model.DB.Where("scope_type = ? AND scope_value = ?", scopeType, scopeValue).
		Assign(model.PortForwardWhitelist{CreatedBy: createdBy}).
		FirstOrCreate(row).Error; err != nil {
		return nil, nil, err
	}

	warnings, err := restoreBannedPortForwardsByWhitelist(scopeType, scopeValue)
	if err != nil {
		return nil, warnings, err
	}
	warnings = append(warnings, healWhitelistedPortForwardsByWhitelist(scopeType, scopeValue)...)
	return row, warnings, nil
}

func DeletePortForwardWhitelist(scopeType, scopeValue string) error {
	scopeType = strings.TrimSpace(scopeType)
	scopeValue = strings.TrimSpace(scopeValue)
	if scopeType == "" || scopeValue == "" {
		return fmt.Errorf("白名单参数不能为空")
	}
	return model.DB.Where("scope_type = ? AND scope_value = ?", scopeType, scopeValue).Delete(&model.PortForwardWhitelist{}).Error
}

func GetPortForwardWhitelistSummary(vmName, username, role string) (*PortForwardWhitelistSummary, error) {
	vmName = strings.TrimSpace(vmName)
	username = strings.TrimSpace(username)
	role = strings.TrimSpace(role)
	if vmName == "" {
		return nil, fmt.Errorf("虚拟机名称不能为空")
	}
	if username == "" {
		username = strings.TrimSpace(HookFindVMOwner(vmName))
	}
	summary := &PortForwardWhitelistSummary{
		VMName:   vmName,
		Username: username,
	}
	if role == "admin" || username == "admin" {
		summary.UserWhitelisted = true
		summary.EffectiveWhitelisted = true
		summary.EffectiveScope = PortForwardWhitelistScopeAdmin
		return summary, nil
	}
	sets, err := loadPortForwardWhitelistSet()
	if err != nil {
		return nil, err
	}
	summary.UserWhitelisted = sets.user[username]
	summary.VMWhitelisted = sets.vm[vmName]
	summary.EffectiveWhitelisted = summary.UserWhitelisted || summary.VMWhitelisted
	switch {
	case summary.UserWhitelisted:
		summary.EffectiveScope = model.PortForwardWhitelistScopeUser
	case summary.VMWhitelisted:
		summary.EffectiveScope = model.PortForwardWhitelistScopeVM
	default:
		summary.EffectiveScope = PortForwardWhitelistScopeNone
	}
	return summary, nil
}

func loadPortForwardWhitelistSet() (*portForwardWhitelistSet, error) {
	var rows []model.PortForwardWhitelist
	if err := model.DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := &portForwardWhitelistSet{
		user: make(map[string]bool),
		vm:   make(map[string]bool),
	}
	for _, row := range rows {
		switch strings.TrimSpace(row.ScopeType) {
		case model.PortForwardWhitelistScopeUser:
			result.user[strings.TrimSpace(row.ScopeValue)] = true
		case model.PortForwardWhitelistScopeVM:
			result.vm[strings.TrimSpace(row.ScopeValue)] = true
		}
	}
	return result, nil
}

func (s *portForwardWhitelistSet) Match(ownerUsername, vmName string, createdByAdmin bool) string {
	ownerUsername = strings.TrimSpace(ownerUsername)
	vmName = strings.TrimSpace(vmName)
	if ownerUsername == "admin" || createdByAdmin {
		return PortForwardWhitelistScopeAdmin
	}
	if s == nil {
		return PortForwardWhitelistScopeNone
	}
	if s.user[ownerUsername] {
		return model.PortForwardWhitelistScopeUser
	}
	if s.vm[vmName] {
		return model.PortForwardWhitelistScopeVM
	}
	return PortForwardWhitelistScopeNone
}

// ── Whitelist restore / heal helpers ──

func restoreBannedPortForwardsByWhitelist(scopeType, scopeValue string) ([]string, error) {
	var states []model.PortForwardProbeState
	query := model.DB.Where("banned = ?", true)
	switch strings.TrimSpace(scopeType) {
	case model.PortForwardWhitelistScopeUser:
		query = query.Where("owner_username = ?", strings.TrimSpace(scopeValue))
	case model.PortForwardWhitelistScopeVM:
		query = query.Where("vm_name = ?", strings.TrimSpace(scopeValue))
	default:
		return nil, nil
	}
	if err := query.Find(&states).Error; err != nil {
		return nil, err
	}
	warnings := make([]string, 0)
	for i := range states {
		if err := restorePortForwardProbeState(&states[i], scopeType); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s 恢复失败: %v", states[i].RuleKey, err))
		}
	}
	return warnings, nil
}

func healWhitelistedPortForwardsByWhitelist(scopeType, scopeValue string) []string {
	var states []model.PortForwardProbeState
	query := model.DB.Where("live = ?", true)
	switch strings.TrimSpace(scopeType) {
	case model.PortForwardWhitelistScopeUser:
		query = query.Where("owner_username = ?", strings.TrimSpace(scopeValue))
	case model.PortForwardWhitelistScopeVM:
		query = query.Where("vm_name = ?", strings.TrimSpace(scopeValue))
	default:
		return nil
	}
	if err := query.Find(&states).Error; err != nil {
		return []string{fmt.Sprintf("%s 白名单修复失败: %v", scopeValue, err)}
	}
	warnings := make([]string, 0)
	for i := range states {
		state := states[i]
		liveRule, err := netpkg.FindLivePortForwardByStableKey(state.RuleKey)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s 校验失败: %v", state.RuleKey, err))
			continue
		}
		if liveRule == nil {
			continue
		}
		vmName := firstNonEmptyString(state.VMName, liveRule.VMName)
		protocol := firstNonEmptyString(strings.ToLower(strings.TrimSpace(state.Protocol)), strings.ToLower(strings.TrimSpace(liveRule.Protocol)), "tcp")
		portText := firstNonEmptyString(state.DestPort, liveRule.DestPort)
		if err := vpcpkg.EnsureSecurityGroupAllowsPortForward(vmName, protocol, portText); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s 修复安全组失败: %v", state.RuleKey, err))
		}
	}
	return warnings
}

func restorePortForwardProbeState(state *model.PortForwardProbeState, whitelistScope string) error {
	if state == nil {
		return nil
	}
	originalState := *state
	if liveRule, _ := netpkg.FindLivePortForwardByStableKey(state.RuleKey); liveRule != nil {
		restoreVMName := firstNonEmptyString(strings.TrimSpace(state.VMName), strings.TrimSpace(liveRule.VMName))
		restoreProtocol := firstNonEmptyString(strings.ToLower(strings.TrimSpace(state.Protocol)), strings.ToLower(strings.TrimSpace(liveRule.Protocol)))
		restorePort := firstNonEmptyString(strings.TrimSpace(state.DestPort), strings.TrimSpace(liveRule.DestPort))
		if err := vpcpkg.EnsureSecurityGroupAllowsPortForward(restoreVMName, restoreProtocol, restorePort); err != nil {
			return fmt.Errorf("补充安全组放行失败: %w", err)
		}
		now := timeNow()
		state.Live = true
		state.Banned = false
		state.BanReason = ""
		state.BannedAt = nil
		state.LastResult = PortForwardProbeStatusRestoredByWhitelist
		state.LastError = ""
		state.LastCheckedAt = &now
		state.WhitelistScope = whitelistScope
		return upsertPortForwardProbeState(state)
	}
	restoreVMName := strings.TrimSpace(state.VMName)
	restoreVMIP := resolvePortForwardRestoreIP(state)
	restoreOwnerUsername := resolvePortForwardRestoreOwner(state)
	restoreProtocol := firstNonEmptyString(strings.ToLower(strings.TrimSpace(state.Protocol)), "tcp")
	restoreRuleKey := netpkg.PortForwardRule{
		Protocol: restoreProtocol,
		HostPort: strings.TrimSpace(state.HostPort),
		DestIP:   restoreVMIP,
		DestPort: strings.TrimSpace(state.DestPort),
	}.StableKey()
	params := &netpkg.PortForwardAddParams{
		VMIP:           restoreVMIP,
		HostPort:       state.HostPort,
		VMPort:         state.DestPort,
		Protocol:       restoreProtocol,
		Comment:        restoreVMName,
		CreatedBy:      strings.TrimSpace(state.CreatedBy),
		CreatedByAdmin: state.CreatedByAdmin,
	}
	if err := netpkg.AddPortForward(params); err != nil {
		return err
	}
	if err := vpcpkg.EnsureSecurityGroupAllowsPortForward(restoreVMName, restoreProtocol, state.DestPort); err != nil {
		_ = netpkg.DeleteLivePortForwardByStableKey(restoreRuleKey, true)
		originalState.Live = false
		_ = upsertPortForwardProbeState(&originalState)
		return fmt.Errorf("恢复安全组放行失败: %w", err)
	}
	now := timeNow()
	if restoreRuleKey != originalState.RuleKey {
		_ = model.DB.Where("rule_key = ?", originalState.RuleKey).Delete(&model.PortForwardProbeState{}).Error
	}
	state.RuleKey = restoreRuleKey
	state.ID = 0
	state.Protocol = restoreProtocol
	state.DestIP = restoreVMIP
	state.OwnerUsername = restoreOwnerUsername
	state.CreatedBy = strings.TrimSpace(params.CreatedBy)
	state.CreatedByAdmin = params.CreatedByAdmin
	state.Live = true
	state.Banned = false
	state.BanReason = ""
	state.BannedAt = nil
	state.LastResult = PortForwardProbeStatusRestoredByWhitelist
	state.LastError = ""
	state.LastCheckedAt = &now
	state.WhitelistScope = whitelistScope
	return upsertPortForwardProbeState(state)
}

func resolvePortForwardRestoreIP(state *model.PortForwardProbeState) string {
	if state == nil {
		return ""
	}
	vmName := strings.TrimSpace(state.VMName)
	if vmName != "" {
		if ip, err := netpkg.EnsureStaticIP(vmName); err == nil && strings.TrimSpace(ip) != "" {
			return strings.TrimSpace(ip)
		}
		if ip := strings.TrimSpace(fwpkg.GetFirewallVMIP(vmName)); ip != "" {
			return ip
		}
	}
	return strings.TrimSpace(state.DestIP)
}

func resolvePortForwardRestoreOwner(state *model.PortForwardProbeState) string {
	if state == nil {
		return ""
	}
	vmName := strings.TrimSpace(state.VMName)
	if vmName != "" {
		if owner := strings.TrimSpace(HookFindVMOwner(vmName)); owner != "" {
			return owner
		}
	}
	return strings.TrimSpace(state.OwnerUsername)
}
