package probe

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/model"
	netpkg "qvmhub/service/network"
)

// PortForwardRule 镜像 network.PortForwardRule，避免 probe → network 循环依赖。
// 由于 probe 可以直接 import network 包，此处直接使用类型别名。
type PortForwardRule = netpkg.PortForwardRule

func RunPortForwardHTTPProbeScan(ctx context.Context, vmName, trigger string, progress func(int, string)) (*PortForwardHTTPProbeRunResult, error) {
	registerPortForwardProbeScheduler()
	if progress == nil {
		progress = func(int, string) {}
	}
	liveRules, err := netpkg.ListLivePortForwardsFromIPTables()
	if err != nil {
		return nil, err
	}
	vmName = strings.TrimSpace(vmName)
	filtered := make([]PortForwardRule, 0, len(liveRules))
	for _, rule := range liveRules {
		if strings.EqualFold(strings.TrimSpace(rule.Protocol), "udp") {
			continue
		}
		if vmName != "" && strings.TrimSpace(rule.VMName) != vmName {
			continue
		}
		filtered = append(filtered, rule)
	}

	result := &PortForwardHTTPProbeRunResult{
		Scanned:   len(filtered),
		MatchedVM: vmName,
	}
	if len(filtered) == 0 {
		progress(100, "没有需要探测的 TCP 端口转发")
		return result, nil
	}

	whitelistSet, err := loadPortForwardWhitelistSet()
	if err != nil {
		return nil, err
	}
	timeoutSeconds := 3
	if config.GlobalConfig != nil && config.GlobalConfig.PortForwardHTTPProbeTimeoutSeconds > 0 {
		timeoutSeconds = config.GlobalConfig.PortForwardHTTPProbeTimeoutSeconds
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	for idx, rule := range filtered {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		progressPercent := int(float64(idx) / float64(len(filtered)) * 100)
		progress(progressPercent, fmt.Sprintf("正在探测 %s (%d/%d)", rule.AccessAddress, idx+1, len(filtered)))
		if err := runPortForwardProbeForRule(&rule, whitelistSet, timeout, trigger); err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, err.Error())
			continue
		}

		state, stateErr := GetPortForwardProbeStateByRuleKey(rule.RuleKey)
		if stateErr != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, stateErr.Error())
			continue
		}
		if state == nil {
			result.Skipped++
			continue
		}
		switch strings.TrimSpace(state.LastResult) {
		case PortForwardProbeStatusHTTPBanned:
			result.Banned++
		case PortForwardProbeStatusHTTPWhitelisted:
			result.Whitelisted++
		case PortForwardProbeStatusClear:
			result.Clear++
		case PortForwardProbeStatusError:
			result.Errors++
			if strings.TrimSpace(state.LastError) != "" {
				result.ErrorDetails = append(result.ErrorDetails, state.LastError)
			}
		default:
			result.Skipped++
		}
	}

	progress(100, fmt.Sprintf("探测完成，扫描 %d 条 TCP 转发", result.Scanned))
	return result, nil
}

func runPortForwardProbeForRule(rule *PortForwardRule, whitelistSet *portForwardWhitelistSet, timeout time.Duration, trigger string) error {
	if rule == nil {
		return nil
	}
	rule.RuleKey = rule.StableKey()
	state, err := GetPortForwardProbeStateByRuleKey(rule.RuleKey)
	if err != nil {
		return err
	}
	if state == nil {
		state = &model.PortForwardProbeState{
			RuleKey:       rule.RuleKey,
			Protocol:      strings.ToLower(strings.TrimSpace(rule.Protocol)),
			HostPort:      strings.TrimSpace(rule.HostPort),
			DestIP:        strings.TrimSpace(rule.DestIP),
			DestPort:      strings.TrimSpace(rule.DestPort),
			VMName:        strings.TrimSpace(rule.VMName),
			OwnerUsername: strings.TrimSpace(rule.OwnerUsername),
		}
	}

	state.Live = true
	state.Protocol = strings.ToLower(strings.TrimSpace(rule.Protocol))
	state.HostPort = strings.TrimSpace(rule.HostPort)
	state.DestIP = strings.TrimSpace(rule.DestIP)
	state.DestPort = strings.TrimSpace(rule.DestPort)
	state.VMName = strings.TrimSpace(rule.VMName)
	state.OwnerUsername = strings.TrimSpace(rule.OwnerUsername)
	now := timeNow()
	state.LastCheckedAt = &now
	state.LastError = ""
	state.LastHTTPStatusCode = 0

	if strings.EqualFold(strings.TrimSpace(rule.Protocol), "udp") {
		state.LastResult = PortForwardProbeStatusNotApplicable
		state.Banned = false
		state.BanReason = ""
		state.WhitelistScope = ""
		return upsertPortForwardProbeState(state)
	}

	httpDetected, statusCode, probeErr := detectHTTPService(rule.DestIP, rule.DestPort, timeout)
	if probeErr != nil {
		state.LastResult = PortForwardProbeStatusError
		state.LastError = probeErr.Error()
		state.WhitelistScope = ""
		return upsertPortForwardProbeState(state)
	}
	if !httpDetected {
		state.Banned = false
		state.BanReason = ""
		state.BannedAt = nil
		state.LastResult = PortForwardProbeStatusClear
		state.LastError = ""
		state.WhitelistScope = ""
		return upsertPortForwardProbeState(state)
	}

	state.LastHTTPStatusCode = statusCode
	scope := whitelistSet.Match(rule.OwnerUsername, rule.VMName, state.CreatedByAdmin)
	state.WhitelistScope = scope
	if scope != PortForwardWhitelistScopeNone {
		state.Banned = false
		state.BanReason = ""
		state.BannedAt = nil
		state.LastResult = PortForwardProbeStatusHTTPWhitelisted
		state.LastError = ""
		return upsertPortForwardProbeState(state)
	}

	event := startPortForwardProbeEvent(rule, statusCode)
	state.Banned = true
	state.Live = false
	state.LastResult = PortForwardProbeStatusHTTPBanned
	state.BanReason = portForwardProbeBanReason
	state.LastError = ""
	state.BannedAt = &now
	if err := upsertPortForwardProbeState(state); err != nil {
		finishPortForwardProbeEventFailure(event, fmt.Sprintf("写入封禁状态失败: %v", err))
		return err
	}
	if err := netpkg.DeleteLivePortForwardByStableKey(rule.RuleKey, true); err != nil {
		state.Live = true
		_ = upsertPortForwardProbeState(state)
		finishPortForwardProbeEventFailure(event, fmt.Sprintf("自动封禁失败: %v", err))
		return fmt.Errorf("自动封禁 %s 失败: %w", rule.AccessAddress, err)
	}
	finishPortForwardProbeEventSuccess(event, fmt.Sprintf("已自动封禁 %s，命中 HTTP 状态码 %d", rule.AccessAddress, statusCode))
	_ = trigger
	return nil
}

func detectHTTPService(destIP, destPort string, timeout time.Duration) (bool, int, error) {
	target := net.JoinHostPort(strings.TrimSpace(destIP), strings.TrimSpace(destPort))
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return false, 0, nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	host := strings.TrimSpace(destIP)
	if host == "" {
		host = "localhost"
	}
	request := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\nUser-Agent: kvm-console-port-probe\r\n\r\n", host)
	if _, err := conn.Write([]byte(request)); err != nil {
		return false, 0, nil
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, 0, nil
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "HTTP/") {
		return false, 0, nil
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return false, 0, nil
	}
	statusCode, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, 0, nil
	}
	return true, statusCode, nil
}
