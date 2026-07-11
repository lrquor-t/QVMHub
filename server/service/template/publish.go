package template

import (
	"strings"
)

// UpdateTemplatePublish updates a template's publish settings.
func UpdateTemplatePublish(templateName string, params *UpdateTemplatePublishParams) error {
	templatePath, err := EnsureTemplatePath(templateName)
	if err != nil {
		return err
	}
	meta := loadTemplateMeta(templatePath)
	if meta == nil {
		normalized := normalizeLoadedTemplateMeta(templateName, templatePath, nil, false)
		meta = &normalized
	}
	if err := ValidateTemplateCategory(meta.Type, params.Category); err != nil {
		return err
	}
	meta.AdminName = strings.TrimSpace(params.AdminName)
	if meta.AdminName == "" {
		meta.AdminName = templateName
	}
	meta.DisplayName = strings.TrimSpace(params.DisplayName)
	if meta.DisplayName == "" {
		meta.DisplayName = meta.AdminName
	}
	meta.CloneVisible = params.CloneVisible
	meta.Disabled = params.Disabled
	meta.Category = normalizeTemplateCategoryForName(meta.Type, params.Category, templateName)
	meta.PostBootCommand = params.PostBootCommand
	meta.PostBootBlocking = params.PostBootBlocking
	meta.DefaultConfig = normalizeTemplateDefaultConfig(&TemplateDefaultConfig{
		VCPU:                params.VCPU,
		RAM:                 params.RAM,
		DiskSize:            params.DiskSize,
		DiskBus:             params.DiskBus,
		NicModel:            params.NicModel,
		VideoModel:          params.VideoModel,
		CPUTopologyMode:     params.CPUTopologyMode,
		FirstBootRebootMode: params.FirstBootRebootMode,
	})
	return saveTemplateMeta(templatePath, meta)
}

// UpdateTemplateMeta 保留旧接口兼容，允许更新管理员可维护字段和默认创建配置。
func UpdateTemplateMeta(templateName string, params *UpdateTemplateMetaParams) error {
	return UpdateTemplatePublish(templateName, &UpdateTemplatePublishParams{
		AdminName:           params.AdminName,
		DisplayName:         params.DisplayName,
		CloneVisible:        params.CloneVisible,
		Disabled:            params.Disabled,
		Category:            params.Category,
		VCPU:                params.VCPU,
		RAM:                 params.RAM,
		DiskSize:            params.DiskSize,
		DiskBus:             params.DiskBus,
		NicModel:            params.NicModel,
		VideoModel:          params.VideoModel,
		CPUTopologyMode:     params.CPUTopologyMode,
		FirstBootRebootMode: params.FirstBootRebootMode,
		PostBootCommand:     params.PostBootCommand,
		PostBootBlocking:    params.PostBootBlocking,
	})
}
