package lxc

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
)

// mergeCacheRows 把 lxc-ls 结果合并进 lxc_containers：存在则更新，消失则标 Present=false。
// 返回当前在线的缓存行。纯逻辑（不直接执行 lxc-ls），便于单测。
func mergeCacheRows(db *gorm.DB, items []ContainerListItem) ([]model.LXCCache, error) {
	if db == nil {
		return nil, nil
	}
	now := time.Now()
	online := map[string]bool{}
	for _, it := range items {
		online[it.Name] = true
		var row model.LXCCache
		err := db.Where("name = ?", it.Name).First(&row).Error
		if err == gorm.ErrRecordNotFound {
			row = model.LXCCache{
				Name:          it.Name,
				OwnerUsername: "admin", // 后续 create 流程会覆盖真实属主；只读发现的默认 admin
				Status:        it.Status,
				CachedIP:      it.IPv4,
				Present:       true,
				LastSyncedAt:  now,
			}
			if err := db.Create(&row).Error; err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		} else {
			row.Status = it.Status
			row.CachedIP = it.IPv4
			row.Present = true
			row.LastSyncedAt = now
			if err := db.Save(&row).Error; err != nil {
				return nil, err
			}
		}
	}
	// 标记消失的容器（当前 lxc-ls 未返回即视为离线）
	if len(online) == 0 {
		// lxc-ls 返回空：所有已缓存容器都标为离线
		if err := db.Model(&model.LXCCache{}).Where("present = ?", true).Update("present", false).Error; err != nil {
			logger.App.Warn("LXC 批量标记离线失败（online 为空）", "error", err)
		}
	} else {
		if err := db.Model(&model.LXCCache{}).Where("present = ? AND name NOT IN ?", true, keysOf(online)).Update("present", false).Error; err != nil {
			logger.App.Warn("LXC 批量标记离线失败，回退逐条", "error", err)
			var all []model.LXCCache
			db.Find(&all)
			for _, r := range all {
				if !online[r.Name] && r.Present {
					if err := db.Model(&r).Update("present", false).Error; err != nil {
						logger.App.Warn("LXC 逐条标记离线失败", "name", r.Name, "error", err)
					}
				}
			}
		}
	}
	var out []model.LXCCache
	if err := db.Where("present = ?", true).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func keysOf(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// isHiddenContainer 判断是否为应从容器列表隐藏的内部容器（金基底模板容器 lxc__tmpl__*）。
// 这些容器与用户容器同处一个 lxcpath，lxc-ls 会一并列出，需在同步缓存时剔除。
func isHiddenContainer(name string) bool {
	return strings.HasPrefix(name, config.GlobalConfig.LXCBasePrefix)
}

// filterVisibleItems 从 lxc-ls 结果剔除隐藏容器（金基底模板），返回用户可见的容器。
func filterVisibleItems(items []ContainerListItem) []ContainerListItem {
	out := items[:0]
	for _, it := range items {
		if isHiddenContainer(it.Name) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// SyncContainerCache 执行 lxc-ls 并同步缓存。
func SyncContainerCache() error {
	res := LxcLsFancy()
	if res.Error != nil {
		return res.Error
	}
	items, err := ParseLxcLsFancy(res.Stdout)
	if err != nil {
		return err
	}
	_, err = mergeCacheRows(model.DB, filterVisibleItems(items))
	return err
}

// ListContainers 返回属主可见的容器缓存（admin 看全部）。
func ListContainers(username string, isAdmin bool) ([]model.LXCCache, error) {
	if err := SyncContainerCache(); err != nil {
		logger.App.Warn("LXC 缓存同步失败，返回旧缓存", "error", err)
	}
	q := model.DB.Where("present = ?", true)
	if !isAdmin {
		q = q.Where("owner_username = ?", username)
	}
	var rows []model.LXCCache
	if err := q.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetContainerDetail 取容器详情（lxc-info + 解析）。
func GetContainerDetail(name string) (ContainerDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ContainerDetail{}, ErrInvalidName
	}
	res := LxcInfo(name)
	if res.Error != nil {
		return ContainerDetail{}, res.Error
	}
	d, err := ParseLxcInfo(res.Stdout)
	if err != nil {
		return ContainerDetail{}, err
	}
	return d, nil
}
