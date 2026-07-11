package clone

import (
	"fmt"

	"qvmhub/utils"
)

// prepareLinuxCloneFirstBootIdentity 执行 Linux 克隆完整离线初始化
// 使用 cloud-init NoCloud 文件方式，无需 SSH 连接
func prepareLinuxCloneFirstBootIdentity(params *CloneParams, cloneDisk string) error {
	return prepareLinuxNoCloudInit(params, cloneDisk)
}

// PrepareLinuxCloneFirstBootIdentityExported 是 prepareLinuxCloneFirstBootIdentity 的导出版本
// 供 clone_delegate.go 等外部包通过代理调用
func PrepareLinuxCloneFirstBootIdentityExported(params *CloneParams, cloneDisk string) error {
	return prepareLinuxCloneFirstBootIdentity(params, cloneDisk)
}

// buildLinuxHostsCommand 生成 /etc/hosts hostname 条目更新命令
// 被 linux_cloudinit.go 的 prepareLinuxNoCloudInit 调用
func buildLinuxHostsCommand(hostname string) string {
	return fmt.Sprintf(`TARGET_HOSTNAME=%s
if grep -q '^127\.0\.1\.1[[:space:]]' /etc/hosts; then
  sed -i "s/^127\.0\.1\.1[[:space:]].*/127.0.1.1\t${TARGET_HOSTNAME}/" /etc/hosts
else
  printf '127.0.1.1\t%%s\n' "$TARGET_HOSTNAME" >> /etc/hosts
fi`, utils.ShellSingleQuote(hostname))
}
