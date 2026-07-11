package lxc

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"os/exec"
	"strconv"
	"strings"

	"qvmhub/model"
	"qvmhub/utils"
)

// IsLxcAvailable 报告 PATH 中是否存在 lxc-create（用于集成测试跳过判定）。
func IsLxcAvailable() bool {
	_, err := exec.LookPath("lxc-create")
	return err == nil
}

// runCmd 是对 utils.ExecCommand 的本包薄封装，便于在 lxc 包内调用并保持调用点简洁。
func runCmd(name string, args ...string) *utils.CmdResult {
	return utils.ExecCommand(name, args...)
}

// LxcLsFancy 执行 lxc-ls --fancy。
func LxcLsFancy() *utils.CmdResult {
	return utils.ExecCommand("lxc-ls", "--fancy")
}

// LxcInfo 执行 lxc-info -n <name>。
func LxcInfo(name string) *utils.CmdResult {
	return utils.ExecCommand("lxc-info", "-n", name)
}

// ParseLxcLsFancy 解析 `lxc-ls --fancy` 输出。
// 表头行形如：NAME STATE IPV4 IPV6 AUTOSTART TYPE GROUP
func ParseLxcLsFancy(stdout string) ([]ContainerListItem, error) {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return []ContainerListItem{}, nil
	}
	lines := strings.Split(stdout, "\n")
	if len(lines) < 2 {
		return nil, errors.New("lxc-ls 输出格式异常：缺少表头")
	}
	header := strings.Fields(lines[0])
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToUpper(h)] = i
	}
	// 至少要有 NAME 与 STATE 列
	if _, ok := idx["NAME"]; !ok {
		return nil, errors.New("lxc-ls 输出缺少 NAME 列")
	}
	out := make([]ContainerListItem, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		item := ContainerListItem{}
		if i, ok := idx["NAME"]; ok && i < len(fields) {
			item.Name = fields[i]
		}
		if i, ok := idx["STATE"]; ok && i < len(fields) {
			item.Status = fields[i]
			item.Running = strings.EqualFold(item.Status, "RUNNING")
		}
		if i, ok := idx["IPV4"]; ok && i < len(fields) {
			v := fields[i]
			if v != "-" {
				item.IPv4 = v
			}
		}
		if i, ok := idx["AUTOSTART"]; ok && i < len(fields) {
			item.Autostart = strings.ToUpper(fields[i])
		}
		out = append(out, item)
	}
	return out, nil
}

// ParseLxcInfo 解析 `lxc-info -n <name>` 的 "Key: Value" 输出。
func ParseLxcInfo(stdout string) (ContainerDetail, error) {
	d := ContainerDetail{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		switch strings.ToLower(key) {
		case "name":
			d.Name = val
		case "state":
			d.Status = val
		case "ip":
			d.IP = val
		case "pid":
			d.PID = val
		case "arch", "architecture":
			d.Arch = val
		}
	}
	return d, nil
}

// genMacByName 由名称派生本地管理 MAC（02: 前缀，避免与 VM 段冲突）。
func genMacByName(seed string) string {
	h := sha1.Sum([]byte(seed))
	hx := hex.EncodeToString(h[:5])
	return "02:" + hx[0:2] + ":" + hx[2:4] + ":" + hx[4:6] + ":" + hx[6:8] + ":" + hx[8:10]
}

// NICMAC 返回容器某网卡 order 的确定性 MAC。
// order 0 与创建流程一致（genMacByName(name)），order≥1 用 name+"#"+order 派生，
// 保证同容器不同网卡 MAC 互不冲突且稳定。
func NICMAC(name string, order int) string {
	if order <= 0 {
		return genMacByName(name)
	}
	return genMacByName(name + "#" + strconv.Itoa(order))
}

// RefreshRuntimeFields 启动后解析 host veth（按容器 MAC 匹配 ip link）与 IP（lxc-info）回填 LXCCache。
func RefreshRuntimeFields(name string) error {
	var row model.LXCCache
	if err := model.DB.Where("name = ?", name).First(&row).Error; err != nil {
		return err
	}
	// IP / Status
	res := LxcInfo(name)
	if res.ExitCode == 0 {
		if d, err := ParseLxcInfo(res.Stdout); err == nil {
			row.CachedIP = d.IP
			row.Status = d.Status
		}
	}
	// veth by 网络命名空间（host 侧 veth MAC 与容器 MAC 无关，不能按 MAC 匹配）
	if veth := findContainerHostVeth(name, 0); veth != "" {
		row.VethName = veth
	}
	return model.DB.Save(&row).Error
}

// findContainerHostVeth 找到容器 <name> 第 <order> 块网卡（容器内默认 eth<order>）在 host 侧的 veth 名。
//
// LXC 的 host 侧 veth MAC 由内核随机分配，与容器内 eth<order> 的 lxc.net.<order>.hwaddr 无关，
// 故不能按 MAC 匹配。这里改用 peer ifindex：
//   1. lxc-attach 进容器 `ip -o link show dev eth<order>`，取其 peer ifindex——即 `eth0@ifN` 里的 N，
//      它是 host 侧 veth 在 host 上的 ifindex（host 上唯一，不受多容器 eth0 同 ifindex 影响）；
//   2. host 上 `ip -o link` 找 ifindex == N 的接口名。
//
// 容器未运行/找不到时返回空串。
func findContainerHostVeth(name string, order int) string {
	ifname := "eth" + strconv.Itoa(order)
	res := utils.ExecShell("lxc-attach -n " + utils.ShellSingleQuote(name) + " -- ip -o link show dev " + ifname + " 2>/dev/null")
	hostIf := parsePeerIfindex(res.Stdout)
	if hostIf == "" {
		return ""
	}
	out := utils.ExecShell("ip -o link")
	return findIfaceByIfindexFromText(out.Stdout, hostIf)
}

// parsePeerIfindex 取 `ip -o link` 输出首行 ifname 的 peer ifindex：`N: ifname@ifP` → "P"。
// 兼容 @ifP / @P 两种写法。纯函数。
func parsePeerIfindex(out string) string {
	line := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
	at := strings.Index(line, "@")
	if at < 0 {
		return ""
	}
	rest := strings.TrimPrefix(line[at+1:], "if")
	var b []byte
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c < '0' || c > '9' {
			break
		}
		b = append(b, c)
	}
	return string(b)
}

// findIfaceByIfindexFromText 在 `ip -o link` 输出里找 ifindex == idx 的接口名（`idx: ifname:...` → ifname）。纯函数。
func findIfaceByIfindexFromText(out, idx string) string {
	prefix := idx + ":"
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, prefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(t, prefix))
		// rest 形如 "vethXXXX: ..." 或 "vethXXXX@if2: ..." → 截到首个 ':'/'@'/空格
		name := rest
		for _, sep := range []string{":", "@", " "} {
			if j := strings.Index(name, sep); j >= 0 {
				name = name[:j]
			}
		}
		if name != "" {
			return name
		}
	}
	return ""
}

