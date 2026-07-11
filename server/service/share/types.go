package share

// ShareInfo 共享目录信息
type ShareInfo struct {
	Tag        string `json:"tag"`         // 标签
	Source     string `json:"source"`      // 宿主机路径
	Target     string `json:"target"`      // 挂载标签
	AccessMode string `json:"access_mode"` // 只读/读写
}
