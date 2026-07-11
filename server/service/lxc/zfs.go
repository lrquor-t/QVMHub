package lxc

// zfs backing 命令/纯函数已下沉到 service/lxc/zfsbacking 子包，以打破 import 环：
// service/lxc/create.go 依赖 service/lxc/template，而 template 也要调 zfs 命令。
// 本文件保留 lxc 包内的同名封装（既有 lxc.Zfs* 调用方与小写内部调用方继续可用），
// 实现全部转发到 zfsbacking。纯函数单测见 zfs_test.go（此处仅保持测试用的小写别名）。

import "qvmhub/service/lxc/zfsbacking"

const zfsBaseSnap = zfsbacking.BaseSnap

// —— 纯函数：dataset 名构造（便于单测；转发到 zfsbacking）——

func zfsBaseDataset(parent, base string) string  { return zfsbacking.BaseDataset(parent, base) }
func zfsBaseSnapshot(parent, base string) string { return zfsbacking.BaseSnapshot(parent, base) }
func zfsContainerDataset(parent, name string) string {
	return zfsbacking.ContainerDataset(parent, name)
}
func zfsContainerMountpoint(lxcpath, name string) string {
	return zfsbacking.ContainerMountpoint(lxcpath, name)
}
func zfsContainerSnapshot(parent, name, snap string) string {
	return zfsbacking.ContainerSnapshot(parent, name, snap)
}
func zfsSnapshotContainer(parent, name, snap string) error {
	return zfsbacking.SnapshotContainer(parent, name, snap)
}
func zfsSetSnapshotComment(parent, name, snap, comment string) error {
	return zfsbacking.SetSnapshotComment(parent, name, snap, comment)
}
func zfsRollbackContainer(parent, name, snap string) error {
	return zfsbacking.RollbackContainer(parent, name, snap)
}
func zfsDestroyContainerSnapshot(parent, name, snap string) error {
	return zfsbacking.DestroyContainerSnapshot(parent, name, snap)
}
func zfsListContainerSnapshots(parent, name string) ([]zfsbacking.ZfsSnapshot, error) {
	return zfsbacking.ListContainerSnapshots(parent, name)
}
func zfsCloneContainerFromSnapshot(parent, src, snap, dst string) error {
	return zfsbacking.CloneContainerFromSnapshot(parent, src, snap, dst)
}
func zfsSnapshotHasClones(parent, name, snap string) (bool, error) {
	return zfsbacking.SnapshotHasClones(parent, name, snap)
}
func rewriteRootfsPathForClone(cfg, oldRootfsPath, newRootfsPath string) string {
	return zfsbacking.RewriteRootfsPathForClone(cfg, oldRootfsPath, newRootfsPath)
}

// —— 跨包用→导出（转发到 zfsbacking）——

func ZfsResolveParent(lxcpath string) (string, error) { return zfsbacking.ResolveParent(lxcpath) }
func ZfsCreateBase(parent, base string) error         { return zfsbacking.CreateBase(parent, base) }
func ZfsSnapshotBase(parent, base string) error       { return zfsbacking.SnapshotBase(parent, base) }
func ZfsDestroyBase(parent, base string) error        { return zfsbacking.DestroyBase(parent, base) }
func zfsCloneContainer(parent, base, name string) error {
	return zfsbacking.CloneContainer(parent, base, name)
}
func zfsDestroyContainer(parent, name string) error { return zfsbacking.DestroyContainer(parent, name) }
func zfsCreateContainerDataset(parent, name string) error {
	return zfsbacking.CreateContainerDataset(parent, name)
}
func isZfsContainer(name string) bool { return zfsbacking.IsZfsContainer(name) }

// —— 容器磁盘配额（refquota）——
func ZfsSetContainerRefquota(parent, name string, bytes int64) error {
	return zfsbacking.SetContainerRefquota(parent, name, bytes)
}
func ZfsGetContainerRefquota(parent, name string) (int64, error) {
	return zfsbacking.GetContainerRefquota(parent, name)
}
