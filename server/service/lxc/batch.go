package lxc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/taskqueue"
	"qvmhub/utils"
)

// BatchCreateContainerParams 批量创建容器参数（task.Params JSON）。
type BatchCreateContainerParams struct {
	Prefix          string                   `json:"prefix"`
	StartNum        int                      `json:"start_num"`
	Count           int                      `json:"count"`
	OwnerUsername   string                   `json:"owner_username"`
	Source          string                   `json:"source"` // clone（默认/空）| download
	Template        string                   `json:"template"`
	Distro          string                   `json:"distro"`
	Release         string                   `json:"release"`
	Arch            string                   `json:"arch"`
	Remark          string                   `json:"remark"`
	GroupName       string                   `json:"group_name"`
	CPUShares       int                      `json:"cpu_shares"`
	MemoryMB        int                      `json:"memory_mb"`
	DiskLimitGB     int                      `json:"disk_limit_gb"`
	Autostart       bool                     `json:"autostart"`
	SwitchID        uint                     `json:"switch_id"`
	SecurityGroupID uint                     `json:"security_group_id"`
	ExtraNics       []AddLXCInterfaceRequest `json:"extra_nics"`
}

// LXCBatchResult 单个容器的批量创建结果。
type LXCBatchResult struct {
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
}

// BatchName 生成批量容器名：prefix-NN（2 位补零）。
// 预检/创建/前端预览共用此函数，杜绝格式漂移。
func BatchName(prefix string, n int) string {
	return fmt.Sprintf("%s-%02d", prefix, n)
}

// ParseBatchCreateContainerParams 反序列化批量创建任务参数。
func ParseBatchCreateContainerParams(s string) (*BatchCreateContainerParams, error) {
	var p BatchCreateContainerParams
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// NameExists 报告容器名是否已被占用：DB 缓存行(present) 或 lxc-info 命中。
func NameExists(name string) bool {
	var count int64
	model.DB.Model(&model.LXCCache{}).Where("name = ? AND present = ?", name, true).Count(&count)
	if count > 0 {
		return true
	}
	// lxc-info 退出码 0 = 容器存在
	if res := utils.ExecCommandQuiet("lxc-info", "-n", name); res.ExitCode == 0 {
		return true
	}
	return false
}

// BatchCreateContainer 并发批量创建容器。
// 部分成功：单项失败记 Error、不回滚、不中断其余（CreateContainer 失败自带 DestroyContainer 回滚）。
// 支持 ctx 取消：返回 (results, taskqueue.ErrTaskCanceled)。并发上限复用 VM 的 BatchCloneMaxConcurrency。
func BatchCreateContainer(ctx context.Context, params *BatchCreateContainerParams, progressFn func(int, string)) ([]LXCBatchResult, error) {
	maxConcurrency := config.GlobalConfig.BatchCloneMaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]LXCBatchResult, params.Count)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)
	var completed, cancelled int32

	if progressFn == nil {
		progressFn = func(int, string) {}
	}
	progressFn(0, fmt.Sprintf("开始批量创建 %d 个容器（最大并发 %d）…", params.Count, maxConcurrency))

	for i := 0; i < params.Count; i++ {
		select {
		case <-ctx.Done():
			wg.Wait()
			return results[:completed], taskqueue.ErrTaskCanceled
		default:
		}
		wg.Add(1)
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Done()
			wg.Wait()
			return results[:completed], taskqueue.ErrTaskCanceled
		}
		go func(index int) {
			defer utils.RecoverAndLog("lxc-batch")
			defer wg.Done()
			defer func() { <-sem }()

			if atomic.LoadInt32(&cancelled) == 1 {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}

			name := BatchName(params.Prefix, params.StartNum+index)
			cp := &CreateContainerParams{
				Name:            name,
				OwnerUsername:   params.OwnerUsername,
				Source:          params.Source,
				Template:        params.Template,
				Distro:          params.Distro,
				Release:         params.Release,
				Arch:            params.Arch,
				Remark:          params.Remark,
				GroupName:       params.GroupName,
				CPUShares:       params.CPUShares,
				MemoryMB:        params.MemoryMB,
				DiskLimitGB:     params.DiskLimitGB,
				Autostart:       params.Autostart,
				SwitchID:        params.SwitchID,
				SecurityGroupID: params.SecurityGroupID,
			}
			// 子项内部进度不外报，整体进度按完成数计。
			err := CreateContainer(cp, func(int, string) {})
			if err == taskqueue.ErrTaskCanceled {
				atomic.StoreInt32(&cancelled, 1)
				cancel()
				return
			}
			mu.Lock()
			if err != nil {
				results[index] = LXCBatchResult{Name: name, Error: err.Error()}
			} else {
				results[index] = LXCBatchResult{Name: name}
			}
			done := atomic.AddInt32(&completed, 1)
			progressFn(int(done*100/int32(params.Count)), fmt.Sprintf("已完成 %d/%d 个", done, params.Count))
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	// CreateContainer 不收 ctx，cancelled 标志永不会被 ErrTaskCanceled 置位；
	// 故 wait 后显式检查 ctx，确保取消时返回 ErrTaskCanceled（任务标记 canceled，而非误报 success）。
	if ctx.Err() != nil {
		return results, taskqueue.ErrTaskCanceled
	}
	if atomic.LoadInt32(&cancelled) == 1 {
		return results, taskqueue.ErrTaskCanceled
	}
	return results, nil
}
