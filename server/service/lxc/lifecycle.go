package lxc

import (
	"os"
	"path/filepath"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/utils"
)

// StartContainer 启动容器并回填运行态字段。
func StartContainer(name string) error {
	res := utils.ExecCommandLongRunning("lxc-start", "-n", name)
	if res.Error != nil {
		return res.Error
	}
	_ = RefreshRuntimeFields(name)
	_ = ReconcileContainerNICs(name) // 重启/启动后重新接入 OVS+VLAN+限速
	return nil
}

// StopContainer 停止容器。
func StopContainer(name string) error {
	res := utils.ExecCommandLongRunning("lxc-stop", "-n", name)
	if res.Error != nil {
		return res.Error
	}
	return updateStatus(name, "STOPPED")
}

// RestartContainer 重启容器（先停后启，忽略未运行错误）。
func RestartContainer(name string) error {
	_ = StopContainer(name)
	return StartContainer(name)
}

// DestroyContainer 先停后删，并清理缓存行与 VPC 绑定。
func DestroyContainer(name string) error {
	// 先解除 VPC 接入（删 OVS 端口、清理绑定），再删除容器。
	_ = DetachContainerFromVPC(name)
	// 先停后删
	_ = utils.ExecCommandQuiet("lxc-stop", "-n", name).Error
	// 按容器实际是否 zfs dataset 分支（filesystem 检测 isZfsContainer，比 DB Backing 稳，不受孤儿/篡改影响）
	if isZfsContainer(name) {
		if parent, err := ZfsResolveParent(config.GlobalConfig.LXCLxcPath); err == nil {
			if err := zfsDestroyContainer(parent, name); err != nil {
				logger.App.Warn("zfs 销毁容器 dataset 失败（dataset 可能残留）", "name", name, "error", err)
			}
		}
		_ = os.RemoveAll(filepath.Join(config.GlobalConfig.LXCLxcPath, name))
	} else {
		if res := utils.ExecCommandLongRunning("lxc-destroy", "-n", name); res.Error != nil {
			return res.Error
		}
	}
	model.DB.Where("name = ?", name).Delete(&model.LXCCache{})
	// 清理该容器的定时任务（与 VM 删除路径一致）
	if err := model.DeleteLXCSchedulesByContainer(name); err != nil {
		logger.App.Warn("清理 LXC 容器定时任务失败", "name", name, "error", err)
	}
	// 清理容器写入 VmStatsRecord 的历史流量行，避免同名容器复用基线（与 VM 删除路径一致）。
	if err := model.DB.Where("vm_name = ?", name).Delete(&model.VmStatsRecord{}).Error; err != nil {
		logger.App.Warn("清理 LXC 容器流量记录失败", "name", name, "error", err)
	}
	ResetContainerStatsState(name) // 清 CPU 采样状态，避免同名容器复用基线
	return nil
}

func updateStatus(name, status string) error {
	return model.DB.Model(&model.LXCCache{}).Where("name = ?", name).Update("status", status).Error
}
