package user

import (
	"fmt"
	"strings"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/utils"
)

// AssignVMsToUser 分配虚拟机给用户
func AssignVMsToUser(username string, vmNames []string) error {
	return AssignVMsToUserWithQuotas(username, vmNames, nil)
}

func AssignVMsToUserWithQuotas(username string, vmNames []string, lightweightQuotas []LightweightVMQuotaRequest) error {
	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}

	// 写入分配文件
	content := strings.Join(vmNames, "\n")
	utils.ExecShell(fmt.Sprintf("echo %s > %s/%s",
		utils.ShellSingleQuote(content), config.GlobalConfig.VMAccessDir, utils.ShellSingleQuote(username)))

	// 重新生成 polkit 规则
	if err := regeneratePolkitRules(); err != nil {
		return err
	}
	defer HookSyncVMCacheOwnersForAssignment(username, vmNames)
	if !HookIsLightweightCloudType(user.CloudType) {
		for _, vmName := range vmNames {
			if HookIsLightweightCloudVM(vmName) {
				HookCleanupVMVPCBinding(vmName)
				HookCleanupLightweightVMResources(vmName)
			}
		}
		return nil
	}

	quotaByVM := make(map[string]LightweightVMQuotaRequest)
	for _, req := range lightweightQuotas {
		req = HookNormalizeLightweightVMQuotaRequest(req)
		if req.VMName != "" {
			quotaByVM[req.VMName] = req
		}
	}
	vmSet := make(map[string]bool)
	for _, vmName := range vmNames {
		vmName = strings.TrimSpace(vmName)
		if vmName == "" {
			continue
		}
		vmSet[vmName] = true
		req, ok := quotaByVM[vmName]
		if !ok {
			req = HookDefaultLightweightVMQuota(vmName)
		}
		req.VMName = vmName
		if _, err := HookUpsertLightweightVMQuota(username, req); err != nil {
			return err
		}
		if err := HookEnsureLightweightVMNetwork(username, vmName); err != nil {
			return err
		}
	}

	var existing []model.LightweightVMQuota
	model.DB.Where("username = ?", username).Find(&existing)
	for _, quota := range existing {
		if !vmSet[quota.VMName] {
			HookCleanupLightweightVMResources(quota.VMName)
		}
	}
	return nil
}
