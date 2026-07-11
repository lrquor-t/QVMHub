package probe

import (
	"context"
	"fmt"
	"strings"

	"qvmhub/model"
)

func ExecuteManualPortForwardHTTPProbe(ctx context.Context, task *PortForwardHTTPProbeTaskParams, createdBy string, progress func(int, string)) (string, error) {
	vmName := ""
	if task != nil {
		vmName = strings.TrimSpace(task.VMName)
	}
	result, err := RunPortForwardHTTPProbeScan(ctx, vmName, "manual", progress)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"scanned":%d,"banned":%d,"whitelisted":%d,"clear":%d,"skipped":%d,"errors":%d,"matched_vm":%q}`,
		result.Scanned, result.Banned, result.Whitelisted, result.Clear, result.Skipped, result.Errors, result.MatchedVM), nil
}

func DeleteBannedPortForwardByRuleKey(ruleKey string) error {
	ruleKey = strings.TrimSpace(ruleKey)
	if ruleKey == "" {
		return fmt.Errorf("规则标识不能为空")
	}
	state, err := GetPortForwardProbeStateByRuleKey(ruleKey)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("封禁记录不存在")
	}
	if state.Live {
		return fmt.Errorf("该端口转发当前仍处于启用状态，请使用常规删除接口")
	}
	return model.DB.Where("rule_key = ?", ruleKey).Delete(&model.PortForwardProbeState{}).Error
}
