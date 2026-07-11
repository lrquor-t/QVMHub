package template

import (
	"errors"
	"regexp"
	"strings"

	"qvmhub/config"
)

// templateNameRE 模板名称字符集校验（与 service/lxc 的容器名规则一致：
// 小写字母、数字、连字符，2-63 字符，首字符为小写字母或数字）。
// 不引入 service/lxc 以保持 package template 独立。
var templateNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

// ImportParams 模板导入参数。SourcePath 为已落地的 rootfs tarball 绝对路径。
type ImportParams struct {
	Name              string `json:"name"`
	DisplayName       string `json:"display_name"`
	Distro            string `json:"distro"`
	Release           string `json:"release"`
	Arch              string `json:"arch"`
	Description       string `json:"description"`
	SourcePath        string `json:"source_path"`
	PostCreateCommand string `json:"post_create_command"`
	OwnerUsername     string `json:"owner_username"`
	Backing           string `json:"backing"` // dir / overlay / zfs（空=用全局默认）
}

func baseContainerName(name string) string {
	return config.GlobalConfig.LXCBasePrefix + name
}

func isBaseContainer(name string) bool {
	return strings.HasPrefix(name, config.GlobalConfig.LXCBasePrefix)
}

// validateTemplateName 模板名称纯校验：非空、非保留基底前缀、字符集正则。
// import 与「从容器制作模板」共用，避免两处重复。
func validateTemplateName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return errors.New("模板名称不能为空")
	}
	// 名称会进入 baseContainerName -> lxc-create/lxc-copy -n <base>（argv 安全）
	// 以及 filepath.Join(LXCLxcPath, base, "rootfs")（路径拼接）。
	// 拒绝路径穿越（如 ../evil）与保留的基底前缀，避免在 lxc 目录之外创建容器。
	if isBaseContainer(n) {
		return errors.New("模板名称不能使用保留的基底前缀")
	}
	if !templateNameRE.MatchString(n) {
		return errors.New("模板名称只能含小写字母、数字、连字符，2-63 字符")
	}
	return nil
}

func validateImportParams(p *ImportParams) error {
	if err := validateTemplateName(p.Name); err != nil {
		return err
	}
	if strings.TrimSpace(p.SourcePath) == "" {
		return errors.New("必须提供 rootfs tarball 路径")
	}
	if p.Arch != "" && p.Arch != "amd64" && p.Arch != "arm64" {
		return errors.New("架构仅支持 amd64/arm64")
	}
	if p.Backing != "" && p.Backing != "dir" && p.Backing != "overlay" && p.Backing != "zfs" {
		return errors.New("后端仅支持 dir / overlay / zfs")
	}
	return nil
}
