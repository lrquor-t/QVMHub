package share

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"qvmhub/utils"
)

// ListShares 列出虚拟机的共享目录
func ListShares(vmName string) ([]ShareInfo, error) {
	return listSharesInternal(vmName, false)
}

// ListSharesInactive 列出虚拟机的持久化配置中的共享目录
func ListSharesInactive(vmName string) ([]ShareInfo, error) {
	return listSharesInternal(vmName, true)
}

// listSharesInternal 内部实现
func listSharesInternal(vmName string, inactive bool) ([]ShareInfo, error) {
	var xmlResult *utils.CmdResult
	if inactive {
		xmlResult = utils.ExecCommand("virsh", "dumpxml", vmName, "--inactive")
	} else {
		xmlResult = utils.ExecCommand("virsh", "dumpxml", vmName)
	}
	if xmlResult.Error != nil {
		return nil, fmt.Errorf("获取虚拟机配置失败")
	}

	var shares []ShareInfo
	// 解析 <filesystem> 标签
	fsRe := regexp.MustCompile(`<filesystem[^>]*>[\s\S]*?</filesystem>`)
	matches := fsRe.FindAllString(xmlResult.Stdout, -1)

	for _, match := range matches {
		share := ShareInfo{}

		// 解析 source
		srcRe := regexp.MustCompile(`<source dir='([^']+)'`)
		if m := srcRe.FindStringSubmatch(match); len(m) > 1 {
			share.Source = m[1]
		}

		// 解析 target
		tgtRe := regexp.MustCompile(`<target dir='([^']+)'`)
		if m := tgtRe.FindStringSubmatch(match); len(m) > 1 {
			share.Tag = m[1]
			share.Target = m[1]
		}

		// 访问模式
		if strings.Contains(match, "readonly") {
			share.AccessMode = "readonly"
		} else {
			share.AccessMode = "readwrite"
		}

		if share.Source != "" {
			shares = append(shares, share)
		}
	}

	return shares, nil
}

// AddShare 添加共享目录
// 9p filesystem 不支持热插拔，VM 必须处于关机状态
func AddShare(vmName, hostPath, tag, securityModel string, readonly bool) error {
	// 检查 VM 状态，必须关机
	vmState := strings.TrimSpace(utils.ExecCommand("virsh", "domstate", vmName).Stdout)
	if vmState == "running" {
		return fmt.Errorf("虚拟机正在运行中，请先关机后再挂载共享目录（9p 不支持热插拔）")
	}

	// 检查是否为 Windows（9p 不支持 Windows）
	xmlResult := utils.ExecCommand("virsh", "dumpxml", vmName, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机配置失败")
	}

	if strings.Contains(xmlResult.Stdout, "hyperv") {
		return fmt.Errorf("Windows 虚拟机不支持 9p 共享目录")
	}

	// 检查是否已存在同名 target（避免 "Target already exists" 错误）
	targetPattern := fmt.Sprintf("<target dir='%s'/>", tag)
	if strings.Contains(xmlResult.Stdout, targetPattern) {
		return fmt.Errorf("该共享目录已挂载（target: %s），无需重复操作", tag)
	}

	if securityModel == "" {
		securityModel = "mapped"
	}

	// 构建 XML
	readonlyTag := ""
	if readonly {
		readonlyTag = "<readonly/>"
	}
	// 9p 共享目录的正确 XML 格式
	fsXML := fmt.Sprintf(
		`<filesystem type='mount' accessmode='mapped'><source dir='%s'/><target dir='%s'/><security_model model='%s'/>%s</filesystem>`,
		hostPath, tag, securityModel, readonlyTag)

	tmpPath := fmt.Sprintf("/tmp/_share-%s-%s.xml", vmName, tag)
	os.WriteFile(tmpPath, []byte(fsXML), 0644)
	result := utils.ExecCommand("virsh", "attach-device", vmName, tmpPath, "--config")
	os.Remove(tmpPath)
	if result.Error != nil {
		return fmt.Errorf("添加共享目录失败: %s", result.Stderr)
	}

	return nil
}

// RemoveShare 移除共享目录
// 9p filesystem 不支持热拔插，VM 必须处于关机状态
func RemoveShare(vmName, tag string) error {
	// 检查 VM 状态，必须关机
	vmState := strings.TrimSpace(utils.ExecCommand("virsh", "domstate", vmName).Stdout)
	if vmState == "running" {
		return fmt.Errorf("虚拟机正在运行中，请先关机后再卸载共享目录（9p 不支持热拔插）")
	}

	// 获取 VM XML
	xmlResult := utils.ExecCommand("virsh", "dumpxml", vmName)
	if xmlResult.Error != nil {
		return fmt.Errorf("获取配置失败")
	}

	fullXML := xmlResult.Stdout

	// 检查是否存在该共享目录
	fsRe := regexp.MustCompile(fmt.Sprintf(`(?s)<filesystem[^>]*>.*?<target dir='%s'/>.*?</filesystem>\s*`, tag))
	if !fsRe.MatchString(fullXML) {
		return fmt.Errorf("未找到共享目录: %s", tag)
	}

	// 从 XML 中移除该 filesystem 节
	newXML := fsRe.ReplaceAllString(fullXML, "")

	// 写入临时文件并 virsh define
	tmpPath := fmt.Sprintf("/tmp/_vm-redefine-%s.xml", vmName)
	if err := os.WriteFile(tmpPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	defer os.Remove(tmpPath)

	result := utils.ExecCommand("virsh", "define", tmpPath)
	if result.Error != nil {
		return fmt.Errorf("更新 VM 配置失败: %s", result.Stderr)
	}

	return nil
}
