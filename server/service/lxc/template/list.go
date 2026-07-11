package template

import (
	"errors"
	"strings"

	"qvmhub/model"
)

func ListTemplates() ([]model.LXCTemplate, error) {
	var rows []model.LXCTemplate
	if err := model.DB.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ErrTemplateNotFound 模板不存在（可比较 sentinel，handler 用 errors.Is 判 404）
var ErrTemplateNotFound = errors.New("模板不存在")

func GetTemplate(name string) (*model.LXCTemplate, error) {
	var tpl model.LXCTemplate
	if err := model.DB.Where("name = ?", name).First(&tpl).Error; err != nil {
		return nil, ErrTemplateNotFound
	}
	return &tpl, nil
}

// UpdateParams 模板展示/管理元数据更新入参（5 项核心字段）。
type UpdateParams struct {
	DisplayName       string
	Description       string
	PostCreateCommand string
	CloneVisible      bool
	Disabled          bool
}

// UpdateTemplate 更新模板展示/管理元数据。
// 用显式 map 的 Updates：bool 字段可能被设为 false，struct 形式 Updates 会跳过零值。
// 输入长度校验由 handler 边界完成（返回 400）；此处只负责存在性（404）+ DB 写入（500）。
func UpdateTemplate(name string, p *UpdateParams) error {
	if _, err := GetTemplate(name); err != nil {
		return ErrTemplateNotFound
	}
	return model.DB.Model(&model.LXCTemplate{}).
		Where("name = ?", name).
		Updates(map[string]interface{}{
			"display_name":        strings.TrimSpace(p.DisplayName),
			"description":         strings.TrimSpace(p.Description),
			"post_create_command": p.PostCreateCommand,
			"clone_visible":       p.CloneVisible,
			"disabled":            p.Disabled,
		}).Error
}
