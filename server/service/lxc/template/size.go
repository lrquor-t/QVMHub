package template

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/utils"
)

// rootfsSizeBytes 计算某基底容器 rootfs 目录的表观大小（字节）。
// 用 du -sb（与 service/user.getDirSizeBytes 同口径：表观大小，而非块占用），
// best-effort：目录不存在 / zfs 数据集未挂载 / du 失败均返回 0——
// 前端 formatSize 对 0 显示 "-"，与现状一致，不会引入回归。
func rootfsSizeBytes(base string) int64 {
	dir := filepath.Join(config.GlobalConfig.LXCLxcPath, base, "rootfs")
	res := utils.ExecShell(fmt.Sprintf("du -sb %s 2>/dev/null | awk '{print $1}'", utils.ShellSingleQuote(dir)))
	if res.Error != nil || res.Stdout == "" {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(res.Stdout), 10, 64)
	return n
}

// BackfillRootfsSizes 为历史「从容器制作模板」记录回填缺失的 rootfs 大小。
//
// 旧版 make.go 未写 RootfsSizeBytes，这些记录落库为 0，前端 rootfs 大小列显示 "-"。
// 仅当 rootfs_size_bytes=0 且对应基底 rootfs 目录仍存在（能算出 >0）时回填；
// 用 UpdateColumn 避免刷新 UpdatedAt。best-effort：失败仅记日志，不阻断启动。
func BackfillRootfsSizes() {
	var rows []model.LXCTemplate
	if err := model.DB.Where("rootfs_size_bytes = 0").Find(&rows).Error; err != nil {
		logger.App.Warn("回填 LXC 模板 rootfs 大小失败（查询）", "error", err)
		return
	}
	filled := 0
	for _, r := range rows {
		size := rootfsSizeBytes(r.BaseContainerName)
		if size <= 0 {
			continue
		}
		if err := model.DB.Model(&model.LXCTemplate{}).Where("id = ?", r.ID).
			UpdateColumn("rootfs_size_bytes", size).Error; err != nil {
			logger.App.Warn("回填 LXC 模板 rootfs 大小失败", "name", r.Name, "error", err)
			continue
		}
		filled++
	}
	if filled > 0 {
		logger.App.Info("回填 LXC 模板 rootfs 大小完成", "count", filled)
	}
}
