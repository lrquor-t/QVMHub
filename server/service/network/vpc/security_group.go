package vpc

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"qvmhub/logger"
	"qvmhub/model"
)

func ListVPCSecurityGroups(operator, role, requestedUsername string) ([]model.VPCSecurityGroup, error) {
	if role != "admin" && HookIsLightweightCloudUser(operator) {
		vmNames := HookGetUserVMList(operator)
		if len(vmNames) == 0 {
			return []model.VPCSecurityGroup{}, nil
		}
		var groups []model.VPCSecurityGroup
		if err := model.DB.Preload("Rules").
			Where("is_vm_scoped = ? AND vm_name IN ?", true, vmNames).
			Order("vm_name ASC, id ASC").
			Find(&groups).Error; err != nil {
			return nil, err
		}
		return groups, nil
	}
	if role != "admin" {
		if _, err := EnsureDefaultSecurityGroup(operator); err != nil {
			return nil, err
		}
	} else if strings.TrimSpace(requestedUsername) != "" {
		_, _ = EnsureDefaultSecurityGroup(strings.TrimSpace(requestedUsername))
	}
	cleanupInvalidVPCSecurityGroupRules(operator, role, requestedUsername)
	query := model.DB.Preload("Rules").Model(&model.VPCSecurityGroup{})
	if role != "admin" {
		query = query.Where("username = ?", operator)
	} else if strings.TrimSpace(requestedUsername) != "" {
		query = query.Where("username = ?", strings.TrimSpace(requestedUsername))
	}
	var groups []model.VPCSecurityGroup
	if err := query.Order("username ASC, is_default DESC, id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func cleanupInvalidVPCSecurityGroupRules(operator, role, requestedUsername string) {
	if model.DB == nil {
		return
	}
	groupQuery := model.DB.Model(&model.VPCSecurityGroup{})
	if role != "admin" {
		groupQuery = groupQuery.Where("username = ?", operator)
	} else if strings.TrimSpace(requestedUsername) != "" {
		groupQuery = groupQuery.Where("username = ?", strings.TrimSpace(requestedUsername))
	}
	var groups []model.VPCSecurityGroup
	if err := groupQuery.Find(&groups).Error; err != nil || len(groups) == 0 {
		return
	}
	groupUsernames := map[uint]string{}
	groupIDs := make([]uint, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
		groupUsernames[group.ID] = group.Username
	}
	var rules []model.VPCSecurityGroupRule
	if err := model.DB.Where("security_group_id IN ? AND target_type IN ?", groupIDs, []string{"switch", "security_group"}).Find(&rules).Error; err != nil {
		return
	}
	for _, rule := range rules {
		username := groupUsernames[rule.SecurityGroupID]
		if err := validateSecurityGroupRuleTarget(username, rule.TargetType, rule.TargetValue); err != nil {
			logger.App.Warn("清理异常安全组规则", "id", rule.ID, "error", err)
			_ = model.DB.Delete(&rule).Error
		}
	}
}

func CreateVPCSecurityGroup(operator, role string, req VPCSecurityGroupRequest) (*model.VPCSecurityGroup, error) {
	if role != "admin" && HookIsLightweightCloudUser(operator) {
		return nil, fmt.Errorf("轻量云用户不能创建全局安全组")
	}
	username, err := resolveVPCUsername(operator, role, req.Username)
	if err != nil {
		return nil, err
	}
	req.Name = normalizeVPCName(req.Name)
	if req.Name == "" {
		return nil, fmt.Errorf("安全组名称不能为空")
	}
	var count int64
	model.DB.Model(&model.VPCSecurityGroup{}).Where("username = ? AND name = ?", username, req.Name).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("安全组名称已存在")
	}
	group := &model.VPCSecurityGroup{Username: username, Name: req.Name, Remark: strings.TrimSpace(req.Remark)}
	if err := model.DB.Create(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

func UpdateVPCSecurityGroup(operator, role string, id uint, req VPCSecurityGroupRequest) (*model.VPCSecurityGroup, error) {
	if role != "admin" && HookIsLightweightCloudUser(operator) {
		return nil, fmt.Errorf("轻量云用户不能修改安全组")
	}
	var group model.VPCSecurityGroup
	if err := model.DB.First(&group, id).Error; err != nil {
		return nil, fmt.Errorf("安全组不存在")
	}
	if role != "admin" && group.Username != operator {
		return nil, fmt.Errorf("无权操作此安全组")
	}
	nextName := group.Name
	if strings.TrimSpace(req.Name) != "" {
		nextName = normalizeVPCName(req.Name)
		if nextName == "" {
			return nil, fmt.Errorf("安全组名称不能为空")
		}
	}
	if group.IsDefault && nextName != group.Name {
		return nil, fmt.Errorf("默认安全组不能修改名称")
	}
	if nextName != group.Name {
		var count int64
		model.DB.Model(&model.VPCSecurityGroup{}).
			Where("username = ? AND name = ? AND id <> ?", group.Username, nextName, group.ID).
			Count(&count)
		if count > 0 {
			return nil, fmt.Errorf("安全组名称已存在")
		}
		group.Name = nextName
	}
	group.Remark = strings.TrimSpace(req.Remark)
	if err := model.DB.Save(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func DeleteVPCSecurityGroup(operator, role string, id uint) error {
	var group model.VPCSecurityGroup
	if err := model.DB.First(&group, id).Error; err != nil {
		return fmt.Errorf("安全组不存在")
	}
	if role != "admin" && HookIsLightweightCloudUser(operator) {
		return fmt.Errorf("轻量云用户不能删除安全组")
	}
	if role != "admin" && group.Username != operator {
		return fmt.Errorf("无权操作此安全组")
	}
	if group.IsDefault {
		return fmt.Errorf("默认安全组不能删除")
	}
	var count int64
	model.DB.Model(&model.VPCVMBinding{}).Where("security_group_id = ?", id).Count(&count)
	if count > 0 {
		return fmt.Errorf("安全组仍被虚拟机使用，不能删除")
	}
	model.DB.Where("security_group_id = ?", id).Delete(&model.VPCSecurityGroupRule{})
	return model.DB.Delete(&group).Error
}

func AddVPCSecurityGroupRule(operator, role string, groupID uint, req VPCSecurityGroupRuleRequest) (*model.VPCSecurityGroupRule, error) {
	var group model.VPCSecurityGroup
	if err := model.DB.First(&group, groupID).Error; err != nil {
		return nil, fmt.Errorf("安全组不存在")
	}
	if role != "admin" && group.Username != operator && !(group.IsVMScoped && HookUserOwnsVM(operator, group.VMName)) {
		return nil, fmt.Errorf("无权操作此安全组")
	}
	if role != "admin" && HookIsLightweightCloudUser(operator) {
		targetType := strings.ToLower(strings.TrimSpace(req.TargetType))
		if targetType == "" {
			targetType = "cidr"
		}
		if targetType != "cidr" {
			return nil, fmt.Errorf("轻量云安全组规则仅支持 CIDR 目标")
		}
	}
	rule, err := normalizeSecurityGroupRule(groupID, req)
	if err != nil {
		return nil, err
	}
	if err := validateSecurityGroupRuleTarget(group.Username, rule.TargetType, rule.TargetValue); err != nil {
		return nil, err
	}
	if err := model.DB.Create(rule).Error; err != nil {
		return nil, err
	}
	return rule, nil
}

func DeleteVPCSecurityGroupRule(operator, role string, ruleID uint) error {
	var rule model.VPCSecurityGroupRule
	if err := model.DB.First(&rule, ruleID).Error; err != nil {
		return fmt.Errorf("安全组规则不存在")
	}
	var group model.VPCSecurityGroup
	if err := model.DB.First(&group, rule.SecurityGroupID).Error; err != nil {
		if role == "admin" {
			return model.DB.Delete(&rule).Error
		}
		return fmt.Errorf("安全组不存在")
	}
	if role != "admin" && group.Username != operator && !(group.IsVMScoped && HookUserOwnsVM(operator, group.VMName)) {
		return fmt.Errorf("无权操作此安全组规则")
	}
	return model.DB.Delete(&rule).Error
}

func normalizeSecurityGroupRule(groupID uint, req VPCSecurityGroupRuleRequest) (*model.VPCSecurityGroupRule, error) {
	direction := strings.ToLower(strings.TrimSpace(req.Direction))
	if direction == "" {
		direction = "ingress"
	}
	if direction != "ingress" && direction != "egress" {
		return nil, fmt.Errorf("方向只支持 ingress 或 egress")
	}
	proto := strings.ToLower(strings.TrimSpace(req.Protocol))
	if proto == "" {
		proto = "tcp"
	}
	if proto != "tcp" && proto != "udp" && proto != "icmp" && proto != "all" {
		return nil, fmt.Errorf("协议只支持 tcp/udp/icmp/all")
	}
	targetType := strings.ToLower(strings.TrimSpace(req.TargetType))
	if targetType == "" {
		targetType = "cidr"
	}
	if targetType != "cidr" && targetType != "switch" && targetType != "security_group" {
		return nil, fmt.Errorf("目标类型无效")
	}
	targetValue := strings.TrimSpace(req.TargetValue)
	if targetValue == "" {
		if targetType == "cidr" {
			targetValue = "0.0.0.0/0"
		} else {
			return nil, fmt.Errorf("请选择目标交换机或安全组")
		}
	}
	if targetType == "cidr" {
		if _, err := netip.ParsePrefix(normalizeCIDROrIP(targetValue)); err != nil {
			return nil, fmt.Errorf("CIDR 无效: %s", targetValue)
		}
		targetValue = normalizeCIDROrIP(targetValue)
	}
	if req.PortEnd == 0 {
		req.PortEnd = req.PortStart
	}
	if (proto == "tcp" || proto == "udp") && (req.PortStart < 1 || req.PortStart > 65535 || req.PortEnd < req.PortStart || req.PortEnd > 65535) {
		return nil, fmt.Errorf("端口范围无效")
	}
	if proto == "icmp" || proto == "all" {
		req.PortStart = 0
		req.PortEnd = 0
	}
	return &model.VPCSecurityGroupRule{
		SecurityGroupID: groupID,
		Direction:       direction,
		Protocol:        proto,
		PortStart:       req.PortStart,
		PortEnd:         req.PortEnd,
		TargetType:      targetType,
		TargetValue:     targetValue,
		Remark:          strings.TrimSpace(req.Remark),
	}, nil
}

func validateSecurityGroupRuleTarget(username, targetType, targetValue string) error {
	switch targetType {
	case "switch":
		id, err := strconv.Atoi(strings.TrimSpace(targetValue))
		if err != nil || id <= 0 {
			return fmt.Errorf("请选择有效的目标交换机")
		}
		var count int64
		model.DB.Model(&model.VPCSwitch{}).Where("id = ? AND username = ?", id, username).Count(&count)
		if count == 0 {
			return fmt.Errorf("目标交换机不存在或不属于该用户")
		}
	case "security_group":
		id, err := strconv.Atoi(strings.TrimSpace(targetValue))
		if err != nil || id <= 0 {
			return fmt.Errorf("请选择有效的目标安全组")
		}
		var count int64
		model.DB.Model(&model.VPCSecurityGroup{}).Where("id = ? AND username = ?", id, username).Count(&count)
		if count == 0 {
			return fmt.Errorf("目标安全组不存在或不属于该用户")
		}
	}
	return nil
}
