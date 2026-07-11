package template

import (
	"errors"

	"qvmhub/model"
)

// DeleteTemplate 删除模板：有派生容器则拒绝；否则销毁基底容器 + 删 DB 行。
func DeleteTemplate(name string) error {
	tpl, err := GetTemplate(name)
	if err != nil {
		return err
	}
	// 派生容器检查（模板名匹配 Template 列）
	var cnt int64
	model.DB.Model(&model.LXCCache{}).Where("template = ? AND present = ?", name, true).Count(&cnt)
	if cnt > 0 {
		return errors.New("存在使用该模板的容器，请先删除相关容器")
	}
	// 基底可能已被手工删除（lxc-info 找不到 config）→ 跳过 destroy 直接删 DB 行；存在才 destroy。
	// existsContainer 用 lxc-info，对 dir/zfs backing 都适用（两者 config 都在 <lxcpath>/<base>/config）。
	if existsContainer(tpl.BaseContainerName) {
		if err := destroyBase(tpl.BaseContainerName, tpl.Backing); err != nil {
			return errors.New("销毁基底容器失败: " + err.Error())
		}
	}
	if err := model.DB.Delete(tpl).Error; err != nil {
		return err
	}
	return nil
}
