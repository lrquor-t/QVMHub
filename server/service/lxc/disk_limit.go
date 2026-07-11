package lxc

import (
	"fmt"

	"qvmhub/config"
)

const bytesPerGB = int64(1024 * 1024 * 1024)

// LXCGetDiskLimit 返回容器 refquota（GB）；0=不限。非 zfs 容器返回错误。
func LXCGetDiskLimit(name string) (int, error) {
	parent, isZfs, err := resolveZfsParentIfContainer(name)
	if err != nil {
		return 0, err
	}
	if !isZfs {
		return 0, fmt.Errorf("非 zfs 后端，不支持磁盘配额")
	}
	bytes, err := ZfsGetContainerRefquota(parent, name)
	if err != nil {
		return 0, err
	}
	return int(bytes / bytesPerGB), nil
}

// LXCSetDiskLimit 设容器 refquota（gb>0 设上限；0 取消）。非 zfs 容器返回错误。
func LXCSetDiskLimit(name string, gb int) error {
	parent, isZfs, err := resolveZfsParentIfContainer(name)
	if err != nil {
		return err
	}
	if !isZfs {
		return fmt.Errorf("非 zfs 后端，不支持磁盘配额")
	}
	if gb < 0 {
		gb = 0
	}
	return ZfsSetContainerRefquota(parent, name, int64(gb)*bytesPerGB)
}

// resolveZfsParentIfContainer 解析 lxcpath 的 zfs parent，并校验该容器确为 zfs 容器。
// 返回 (parent, isZfs, err)：lxcpath 非 zfs → ("", false, nil)；容器非 zfs → ("", false, nil)。
func resolveZfsParentIfContainer(name string) (string, bool, error) {
	lxcpath := config.GlobalConfig.LXCLxcPath
	parent, err := ZfsResolveParent(lxcpath)
	if err != nil {
		// lxcpath 不在 zfs 上 → 整体非 zfs
		return "", false, nil
	}
	if !isZfsContainer(name) {
		return "", false, nil
	}
	return parent, true, nil
}
