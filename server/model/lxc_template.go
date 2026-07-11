package model

import "time"

// LXCTemplate 表示一个 LXC 模板：金基底容器 + 本行展示/管理元数据。
// lxc 原生默认配置（cgroup/net/autostart）放在金基底容器的 config 文件里，由 lxc-copy 继承。
type LXCTemplate struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	Name              string    `json:"name" gorm:"uniqueIndex;size:128;not null"`
	DisplayName       string    `json:"display_name" gorm:"size:255"`
	Distro            string    `json:"distro" gorm:"size:64"`
	Release           string    `json:"release" gorm:"size:64"`
	Arch              string    `json:"arch" gorm:"size:16"`
	Description       string    `json:"description" gorm:"size:512"`
	BaseContainerName string    `json:"base_container_name" gorm:"size:128;not null"`
	Backing           string    `json:"backing" gorm:"size:32;default:'overlay'"`
	RootfsSizeBytes   int64     `json:"rootfs_size_bytes"`
	CloneVisible      bool      `json:"clone_visible" gorm:"default:true"`
	Disabled          bool      `json:"disabled" gorm:"default:false"`
	OwnerUsername     string    `json:"owner_username" gorm:"size:64;not null;default:'admin'"`
	PostCreateCommand string    `json:"post_create_command" gorm:"type:text"` // 可选，创建后 lxc-attach 执行
	SHA256            string    `json:"sha256" gorm:"size:64"`                // 原始 tarball 校验（审计用）
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (LXCTemplate) TableName() string { return "lxc_templates" }
