package probe

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"qvmhub/model"
	netpkg "qvmhub/service/network"
)

// PortForwardAddParams 镜像类型别名，避免 probe → service root 循环依赖。
type PortForwardAddParams = netpkg.PortForwardAddParams

func SyncPortForwardProbeStateOnAdd(params *PortForwardAddParams, protocol string, ownerUsername string) {
	if params == nil {
		return
	}
	ownerUsername = strings.TrimSpace(ownerUsername)
	vmName := strings.TrimSpace(params.Comment)
	if vmName == "" {
		vmName = strings.TrimSpace(params.VMIP)
	}
	rule := PortForwardRule{
		Protocol:      strings.ToUpper(strings.TrimSpace(protocol)),
		HostPort:      strings.TrimSpace(params.HostPort),
		DestIP:        strings.TrimSpace(params.VMIP),
		DestPort:      strings.TrimSpace(params.VMPort),
		VMName:        vmName,
		OwnerUsername: ownerUsername,
	}
	rule.RuleKey = rule.StableKey()
	now := timeNow()
	state := &model.PortForwardProbeState{
		RuleKey:            rule.RuleKey,
		Protocol:           strings.ToLower(strings.TrimSpace(protocol)),
		HostPort:           rule.HostPort,
		DestIP:             rule.DestIP,
		DestPort:           rule.DestPort,
		VMName:             rule.VMName,
		OwnerUsername:      ownerUsername,
		CreatedBy:          strings.TrimSpace(params.CreatedBy),
		CreatedByAdmin:     params.CreatedByAdmin,
		Live:               true,
		Banned:             false,
		LastResult:         PortForwardProbeStatusPending,
		LastError:          "",
		BanReason:          "",
		WhitelistScope:     "",
		LastCheckedAt:      &now,
		LastHTTPStatusCode: 0,
		BannedAt:           nil,
	}
	_ = upsertPortForwardProbeState(state)
}

func SyncPortForwardProbeStateOnDelete(ruleKey string, deletedByBan bool) {
	ruleKey = strings.TrimSpace(ruleKey)
	if ruleKey == "" {
		return
	}
	state, err := GetPortForwardProbeStateByRuleKey(ruleKey)
	if err != nil || state == nil {
		return
	}
	if !deletedByBan {
		_ = model.DB.Where("rule_key = ?", ruleKey).Delete(&model.PortForwardProbeState{}).Error
		return
	}
	state.Live = false
	_ = upsertPortForwardProbeState(state)
}

func MergePortForwardProbeState(rules []PortForwardRule) []PortForwardRule {
	stateMap, err := getPortForwardProbeStateMap()
	if err != nil {
		return rules
	}
	seen := make(map[string]bool, len(rules))
	merged := make([]PortForwardRule, 0, len(rules))
	for _, rule := range rules {
		rule.RuleKey = rule.StableKey()
		rule.Live = true
		applyProbeStateToRule(&rule, stateMap[rule.RuleKey])
		if strings.EqualFold(strings.TrimSpace(rule.Protocol), "udp") && rule.ProbeStatus == "" {
			rule.ProbeStatus = PortForwardProbeStatusNotApplicable
			rule.ProbeReason = "仅 TCP 转发参与 HTTP 探测"
		}
		seen[rule.RuleKey] = true
		merged = append(merged, rule)
	}
	for _, state := range stateMap {
		if state == nil || !state.Banned || state.Live || seen[state.RuleKey] {
			continue
		}
		rule := buildPortForwardRuleFromProbeState(state)
		if rule.RuleKey == "" {
			continue
		}
		seen[rule.RuleKey] = true
		merged = append(merged, rule)
	}
	return merged
}

// ── Internal state helpers ──

// GetPortForwardProbeStateByRuleKey 导出版本，供 Hook 和内部共用。
func GetPortForwardProbeStateByRuleKey(ruleKey string) (*model.PortForwardProbeState, error) {
	var state model.PortForwardProbeState
	if err := model.DB.Where("rule_key = ?", strings.TrimSpace(ruleKey)).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func getPortForwardProbeStateMap() (map[string]*model.PortForwardProbeState, error) {
	var states []model.PortForwardProbeState
	if err := model.DB.Find(&states).Error; err != nil {
		return nil, err
	}
	result := make(map[string]*model.PortForwardProbeState, len(states))
	for i := range states {
		state := states[i]
		result[strings.TrimSpace(state.RuleKey)] = &state
	}
	return result, nil
}

func upsertPortForwardProbeState(state *model.PortForwardProbeState) error {
	if state == nil || strings.TrimSpace(state.RuleKey) == "" {
		return nil
	}
	var existing model.PortForwardProbeState
	if err := model.DB.Where("rule_key = ?", state.RuleKey).First(&existing).Error; err == nil {
		state.ID = existing.ID
		if strings.TrimSpace(state.CreatedBy) == "" {
			state.CreatedBy = strings.TrimSpace(existing.CreatedBy)
		}
		if !state.CreatedByAdmin {
			state.CreatedByAdmin = existing.CreatedByAdmin
		}
		return model.DB.Model(&existing).Updates(map[string]interface{}{
			"protocol":              state.Protocol,
			"host_port":             state.HostPort,
			"dest_ip":               state.DestIP,
			"dest_port":             state.DestPort,
			"vm_name":               state.VMName,
			"owner_username":        state.OwnerUsername,
			"created_by":            state.CreatedBy,
			"created_by_admin":      state.CreatedByAdmin,
			"live":                  state.Live,
			"banned":                state.Banned,
			"whitelist_scope":       state.WhitelistScope,
			"last_checked_at":       state.LastCheckedAt,
			"last_http_status_code": state.LastHTTPStatusCode,
			"last_result":           state.LastResult,
			"last_error":            state.LastError,
			"ban_reason":            state.BanReason,
			"banned_at":             state.BannedAt,
		}).Error
	}
	return model.DB.Create(state).Error
}

func applyProbeStateToRule(rule *PortForwardRule, state *model.PortForwardProbeState) {
	if rule == nil {
		return
	}
	rule.Live = true
	rule.RuleKey = rule.StableKey()
	if state == nil {
		if strings.EqualFold(strings.TrimSpace(rule.Protocol), "tcp") {
			rule.ProbeStatus = PortForwardProbeStatusPending
			rule.ProbeReason = "等待首次 HTTP 探测"
		}
		return
	}
	rule.Live = state.Live || !state.Banned
	rule.Banned = state.Banned
	rule.ProbeStatus = strings.TrimSpace(state.LastResult)
	rule.ProbeReason = strings.TrimSpace(state.BanReason)
	if rule.ProbeReason == "" {
		if state.LastResult == PortForwardProbeStatusHTTPWhitelisted {
			rule.ProbeReason = buildWhitelistReason(state.WhitelistScope, state.LastHTTPStatusCode)
		} else if state.LastResult == PortForwardProbeStatusClear {
			rule.ProbeReason = "最近一次探测未发现明文 HTTP 服务"
		} else if state.LastResult == PortForwardProbeStatusError {
			rule.ProbeReason = strings.TrimSpace(state.LastError)
		}
	}
	rule.ProbeWhitelistScope = strings.TrimSpace(state.WhitelistScope)
	rule.ProbeHTTPStatusCode = state.LastHTTPStatusCode
	rule.ProbeLastCheckedAt = formatTimePointer(state.LastCheckedAt)
	if strings.EqualFold(strings.TrimSpace(rule.Protocol), "udp") && rule.ProbeStatus == "" {
		rule.ProbeStatus = PortForwardProbeStatusNotApplicable
		rule.ProbeReason = "仅 TCP 转发参与 HTTP 探测"
	}
}

func buildPortForwardRuleFromProbeState(state *model.PortForwardProbeState) PortForwardRule {
	rule := PortForwardRule{
		ID:                    -int(state.ID),
		Protocol:              strings.ToUpper(strings.TrimSpace(state.Protocol)),
		HostPort:              strings.TrimSpace(state.HostPort),
		AccessIP:              netpkg.GetHostIP(),
		AccessAddress:         netpkg.BuildPortForwardAccessAddress(netpkg.GetHostIP(), state.HostPort),
		DestIP:                strings.TrimSpace(state.DestIP),
		DestPort:              strings.TrimSpace(state.DestPort),
		VMName:                strings.TrimSpace(state.VMName),
		OwnerUsername:         strings.TrimSpace(state.OwnerUsername),
		FirewallKey:           strings.TrimSpace(state.RuleKey),
		RegionFilterEnabled:   true,
		RegionFilterInherited: true,
		RuleKey:               strings.TrimSpace(state.RuleKey),
		Live:                  false,
		Banned:                state.Banned,
	}
	applyProbeStateToRule(&rule, state)
	rule.Live = false
	return rule
}

// ── Small utility functions ──

func formatTimePointer(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func buildWhitelistReason(scope string, statusCode int) string {
	switch strings.TrimSpace(scope) {
	case PortForwardWhitelistScopeAdmin:
		return fmt.Sprintf("检测到明文 HTTP 响应，状态码 %d，但该转发归属管理员，默认白名单放行", statusCode)
	case model.PortForwardWhitelistScopeUser:
		return fmt.Sprintf("检测到明文 HTTP 响应，状态码 %d，但归属用户已在白名单中", statusCode)
	case model.PortForwardWhitelistScopeVM:
		return fmt.Sprintf("检测到明文 HTTP 响应，状态码 %d，但虚拟机已在白名单中", statusCode)
	default:
		return fmt.Sprintf("检测到明文 HTTP 响应，状态码 %d，但当前规则命中白名单", statusCode)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

// timeNow 包装 time.Now，便于测试。
func timeNow() time.Time {
	return time.Now()
}
