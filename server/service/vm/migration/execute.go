package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/service/vm_xml"
	"qvmhub/utils"
)

var (
	vmMigrationNVRAMTemplateAttr = regexp.MustCompile(`\s+template=['"][^'"]+['"]`)
	vmMigrationNVRAMTemplateFmt  = regexp.MustCompile(`\s+templateFormat=['"][^'"]+['"]`)
	vmMigrationNVRAMTag          = regexp.MustCompile(`(?s)<nvram\b[^>]*(?:/>|>.*?</nvram>)`)
)

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ExecuteVMMigration(ctx context.Context, params VMMigrationTaskParams, progress func(int, string)) (*MigrationAdoptResult, error) {
	if progress == nil {
		progress = func(int, string) {}
	}
	var preview *VMMigrationPreview
	if strings.TrimSpace(params.PreviewID) != "" {
		progress(5, "正在读取迁移预检结果...")
		cachedPreview, err := loadMigrationPreview(params.PreviewID)
		if err != nil {
			return nil, err
		}
		if !cachedPreview.Allowed {
			return nil, fmt.Errorf("迁移预检未通过: %s", strings.Join(cachedPreview.Blockers, "；"))
		}
		if err := validateCachedPreviewForExecution(cachedPreview, params); err != nil {
			return nil, err
		}
		preview = cachedPreview
	} else {
		progress(5, "正在生成迁移执行计划...")
		builtPreview, err := BuildVMMigrationPreview(params.VMName, VMMigrationRequest{
			NodeID:                params.NodeID,
			Mode:                  params.Mode,
			SkipPrecheck:          params.SkipPrecheck,
			TargetStoragePoolID:   params.TargetStoragePoolID,
			DiskStorageTargets:    params.DiskStorageTargets,
			TargetSwitchID:        params.TargetSwitchID,
			TargetSecurityGroupID: params.TargetSecurityGroupID,
			EnableCPUThrottle:     params.EnableCPUThrottle,
			CPUThrottlePercent:    params.CPUThrottlePercent,
		})
		if err != nil {
			return nil, err
		}
		if !builtPreview.Allowed {
			return nil, fmt.Errorf("迁移执行计划未通过: %s", strings.Join(builtPreview.Blockers, "；"))
		}
		preview = builtPreview
	}
	node, err := service.GetHostNode(preview.Node.ID)
	if err != nil {
		return nil, err
	}
	progress(18, "正在导出虚拟机 XML...")
	xmlText := utils.ExecCommand("virsh", "dumpxml", preview.VMName).Stdout
	if strings.TrimSpace(xmlText) == "" {
		return nil, fmt.Errorf("导出虚拟机 XML 失败")
	}
	xmlText = applyMigrationDiskPathsToXML(xmlText, preview.Disks)
	if preview.Mode == MigrationModeCold {
		if err := executeColdMigration(ctx, *node, preview, xmlText, progress); err != nil {
			return nil, err
		}
	} else {
		if err := executeLiveMigration(ctx, *node, preview, params.EnableCPUThrottle, params.CPUThrottlePercent, progress); err != nil {
			return nil, err
		}
	}
	progress(82, "正在让目标面板接管虚拟机...")
	adoptReq := buildAdoptRequest(preview)
	var adoptResult MigrationAdoptResult
	if _, err := service.CallNodeAPI(*node, "POST", "/api/migration/adopt-vm", adoptReq, &adoptResult); err != nil {
		return nil, err
	}
	if len(preview.Warnings) > 0 {
		adoptResult.Warnings = append(preview.Warnings, adoptResult.Warnings...)
	}
	progress(100, "虚拟机迁移完成，源节点副本已保留")
	return &adoptResult, nil
}

// ensureMigratedVMNVRAM 确保迁移后的虚拟机 NVRAM 文件存在。
func ensureMigratedVMNVRAM(vmName string) error {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", vmName, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}
	bootType := vm_xml.ParseVMBootTypeFromDomainXML(xmlResult.Stdout)
	if err := vm_xml.EnsureVMUEFINVRAMFile(vmName, xmlResult.Stdout, bootType); err != nil {
		return fmt.Errorf("创建 UEFI NVRAM 文件失败: %w", err)
	}
	return nil
}

func executeColdMigration(ctx context.Context, node model.HostNode, preview *VMMigrationPreview, xmlText string, progress func(int, string)) error {
	for i, disk := range preview.Disks {
		progress(25+(i*30)/maxInt(1, len(preview.Disks)), "正在复制磁盘 overlay: "+filepath.Base(disk.SourcePath))
		if _, err := service.RemoteSSHCommand(ctx, node, "test ! -e "+utils.ShellSingleQuote(disk.TargetPath), 30*time.Second); err != nil {
			return fmt.Errorf("目标磁盘已存在或无法访问: %s", disk.TargetPath)
		}
		if err := service.RemoteRsyncFile(ctx, node, disk.SourcePath, disk.TargetPath, 6*time.Hour); err != nil {
			return fmt.Errorf("复制磁盘 %s 失败: %w", disk.SourcePath, err)
		}
		_, _ = service.RemoteSSHCommand(ctx, node, "chown libvirt-qemu:kvm "+utils.ShellSingleQuote(disk.TargetPath)+" || true", 30*time.Second)
	}
	progress(62, "正在复制并定义虚拟机 XML...")
	targetXML := "/tmp/kvm-migrate-" + preview.VMName + ".xml"
	if err := service.WriteRemoteFile(ctx, node, xmlText, targetXML, 30*time.Second); err != nil {
		return err
	}
	if _, err := service.RemoteSSHCommand(ctx, node, "virsh define "+utils.ShellSingleQuote(targetXML), 60*time.Second); err != nil {
		return fmt.Errorf("目标节点定义 VM 失败: %w", err)
	}
	return nil
}

func executeLiveMigration(ctx context.Context, node model.HostNode, preview *VMMigrationPreview, enableCPUThrottle bool, cpuThrottlePercent int, progress func(int, string)) error {
	progress(32, "正在评估热迁移线路与脏页速率...")
	assessment, err := AssessLiveMigration(ctx, node, preview.VMName, enableCPUThrottle, cpuThrottlePercent)
	if err != nil {
		return err
	}
	preview.LiveAssessment = assessment
	preview.Warnings = append(preview.Warnings, assessment.Warnings...)
	if !assessment.Allowed {
		return fmt.Errorf("%s", assessment.BlockReason)
	}
	var cpuRestore *migrationCPUThrottleRestore
	if assessment.CPUThrottleEnabled {
		progress(34, fmt.Sprintf("正在限制 VM CPU 使用率为 %d%% 以降低脏页生成...", assessment.CPUThrottlePercent))
		cpuRestore, err = applyMigrationCPUThrottle(preview.VMName, assessment.CPUThrottlePercent)
		if err != nil {
			return fmt.Errorf("设置迁移 CPU 限制失败: %w", err)
		}
		defer func() {
			if restoreErr := cpuRestore.Restore(ctx, node, preview.VMName); restoreErr != nil {
				preview.Warnings = append(preview.Warnings, "迁移后恢复 CPU 限制失败: "+restoreErr.Error())
			}
		}()
	}
	progress(36, "正在准备热迁移 SSH 信任...")
	if err := service.EnsureDefaultSSHKeyTrusted(ctx, node); err != nil {
		return fmt.Errorf("准备热迁移 SSH 信任失败: %w", err)
	}
	progress(40, "正在准备热迁移目标磁盘...")
	createdTargets, err := prepareLiveMigrationTargets(ctx, node, preview)
	if err != nil {
		return err
	}
	cleanupTargets := true
	defer func() {
		if cleanupTargets && len(createdTargets) > 0 {
			cleanupLiveMigrationTargets(ctx, node, createdTargets)
		}
	}()

	progress(43, "正在准备热迁移目标 NVRAM...")
	vmXML := utils.ExecCommand("virsh", "dumpxml", preview.VMName).Stdout
	vmXML = applyMigrationDiskPathsToXML(vmXML, preview.Disks)
	nvramPath, vmXML, err := prepareMigrationNVRAMOnTarget(ctx, node, vmXML)
	if err != nil {
		return err
	}
	if nvramPath != "" {
		createdTargets = append(createdTargets, nvramPath)
	}

	progress(46, "正在执行热迁移...")
	sshURI := fmt.Sprintf("qemu+ssh://%s@%s/system", node.SSHUser, node.SSHHost)
	localXML := filepath.Join("/tmp", "kvm-migrate-"+preview.VMName+"-live.xml")
	if err := writeLocalFile(localXML, vmXML); err != nil {
		return fmt.Errorf("写入热迁移目标 XML 失败: %w", err)
	}
	migrateHost := migrationURIHost(node.SSHHost)
	cmd := fmt.Sprintf("virsh migrate --live --persistent --copy-storage-inc --verbose --xml %s --migrateuri %s --disks-uri %s %s %s",
		utils.ShellSingleQuote(localXML),
		utils.ShellSingleQuote("tcp://"+migrateHost),
		utils.ShellSingleQuote("tcp://"+migrateHost),
		utils.ShellSingleQuote(preview.VMName),
		utils.ShellSingleQuote(sshURI))
	result := utils.ExecShellContextWithTimeout(ctx, cmd, 6*time.Hour)
	if result.Error != nil {
		return fmt.Errorf("热迁移失败: %s", firstNonEmpty(result.Stderr, result.Error.Error()))
	}
	cleanupTargets = false
	return nil
}

func migrationURIHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func prepareLiveMigrationTargets(ctx context.Context, node model.HostNode, preview *VMMigrationPreview) ([]string, error) {
	var created []string

	// 在创建目标磁盘前，按目录校验远程节点剩余空间
	requiredByDir := map[string]int64{}
	for _, disk := range preview.Disks {
		if strings.TrimSpace(disk.TargetPath) == "" || disk.VirtualSize <= 0 {
			continue
		}
		dir := filepath.Dir(disk.TargetPath)
		requiredByDir[dir] += disk.VirtualSize / (1024 * 1024) // bytes → MB
	}
	for dir, requiredMB := range requiredByDir {
		if requiredMB <= 0 {
			continue
		}
		checkCmd := fmt.Sprintf("df -BM --output=avail %s | tail -1 | tr -d ' M'", utils.ShellSingleQuote(dir))
		out, err := service.RemoteSSHCommand(ctx, node, checkCmd, 30*time.Second)
		if err != nil {
			return created, fmt.Errorf("检查目标目录 %s 磁盘空间失败: %w", dir, err)
		}
		availMB, parseErr := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		if parseErr != nil {
			return created, fmt.Errorf("解析目标目录 %s 可用空间失败: %w", dir, parseErr)
		}
		if availMB < requiredMB {
			return created, fmt.Errorf("目标目录 %s 可用空间不足，需要 %d MB，当前可用 %d MB", dir, requiredMB, availMB)
		}
	}

	for _, disk := range preview.Disks {
		if strings.TrimSpace(disk.TargetPath) == "" {
			return created, fmt.Errorf("目标磁盘路径为空: %s", disk.Target)
		}
		if _, err := service.RemoteSSHCommand(ctx, node, "test ! -e "+utils.ShellSingleQuote(disk.TargetPath), 30*time.Second); err != nil {
			return created, fmt.Errorf("目标磁盘已存在或无法访问: %s", disk.TargetPath)
		}
		cmd, err := buildLiveMigrationTargetCreateCommand(disk)
		if err != nil {
			return created, err
		}
		if _, err := service.RemoteSSHCommand(ctx, node, cmd, 2*time.Minute); err != nil {
			return created, fmt.Errorf("创建热迁移目标磁盘 %s 失败: %w", disk.TargetPath, err)
		}
		created = append(created, disk.TargetPath)
	}
	return created, nil
}

func buildLiveMigrationTargetCreateCommand(disk MigrationDisk) (string, error) {
	format := strings.TrimSpace(disk.Format)
	if format == "" {
		return "", fmt.Errorf("磁盘 %s 缺少格式信息，无法创建热迁移目标盘", disk.SourcePath)
	}
	if disk.VirtualSize <= 0 {
		return "", fmt.Errorf("磁盘 %s 缺少容量信息，无法创建热迁移目标盘", disk.SourcePath)
	}
	dir := filepath.Dir(disk.TargetPath)
	cmd := "set -e; mkdir -p " + utils.ShellSingleQuote(dir) + "; test ! -e " + utils.ShellSingleQuote(disk.TargetPath) + "; qemu-img create -f " + utils.ShellSingleQuote(format)
	if strings.TrimSpace(disk.BackingPath) != "" {
		backingFormat := strings.TrimSpace(disk.BackingFormat)
		if backingFormat == "" {
			return "", fmt.Errorf("磁盘 %s 缺少 backing 格式信息，无法创建热迁移目标盘", disk.SourcePath)
		}
		cmd += " -F " + utils.ShellSingleQuote(backingFormat) + " -b " + utils.ShellSingleQuote(disk.BackingPath)
	}
	cmd += " " + utils.ShellSingleQuote(disk.TargetPath) + " " + strconv.FormatInt(disk.VirtualSize, 10) + "; chown libvirt-qemu:kvm " + utils.ShellSingleQuote(disk.TargetPath) + " || true"
	return cmd, nil
}

func cleanupLiveMigrationTargets(ctx context.Context, node model.HostNode, paths []string) {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		_, _ = service.RemoteSSHCommand(ctx, node, "rm -f "+utils.ShellSingleQuote(path), 30*time.Second)
	}
}

// stripNVRAMTemplateFromXML 从 XML 中移除 <nvram> 标签的 template 和 templateFormat 属性
// 用于热迁移场景：目标节点已预先创建好 qcow2 NVRAM 文件，不需要 libvirt 再从模板转换
func stripNVRAMTemplateFromXML(xmlContent string) string {
	return vmMigrationNVRAMTag.ReplaceAllStringFunc(xmlContent, func(tag string) string {
		tag = vmMigrationNVRAMTemplateAttr.ReplaceAllString(tag, "")
		tag = vmMigrationNVRAMTemplateFmt.ReplaceAllString(tag, "")
		return tag
	})
}

// prepareMigrationNVRAMOnTarget 在目标节点预创建 UEFI NVRAM qcow2 文件
// 返回 NVRAM 文件路径（用于失败清理）和修改后的 XML（已移除 template 属性）
func prepareMigrationNVRAMOnTarget(ctx context.Context, node model.HostNode, xmlText string) (nvramPath string, modifiedXML string, err error) {
	bootType := vm_xml.ParseVMBootTypeFromDomainXML(xmlText)
	if bootType != vm_xml.VMBootTypeUEFI && bootType != vm_xml.VMBootTypeUEFISecure {
		return "", xmlText, nil
	}
	nvramPath = vm_xml.ExtractDomainNVRAMPath(xmlText)
	if nvramPath == "" {
		return "", xmlText, nil
	}
	secure := bootType == vm_xml.VMBootTypeUEFISecure
	templatePath := vm_xml.ResolveOVMFVarsTemplatePath(secure)
	nvramDir := filepath.Dir(nvramPath)
	mkdirCmd := "mkdir -p " + utils.ShellSingleQuote(nvramDir)
	convertCmd := fmt.Sprintf("qemu-img convert -f raw -O qcow2 %s %s && chmod 600 %s && (chown libvirt-qemu:kvm %s 2>/dev/null || chown qemu:qemu %s 2>/dev/null || true)",
		utils.ShellSingleQuote(templatePath),
		utils.ShellSingleQuote(nvramPath),
		utils.ShellSingleQuote(nvramPath),
		utils.ShellSingleQuote(nvramPath),
		utils.ShellSingleQuote(nvramPath))
	fullCmd := mkdirCmd + " && " + convertCmd
	if _, err := service.RemoteSSHCommand(ctx, node, fullCmd, 60*time.Second); err != nil {
		return nvramPath, xmlText, fmt.Errorf("目标节点创建 NVRAM 文件失败: %w", err)
	}
	modifiedXML = stripNVRAMTemplateFromXML(xmlText)
	return nvramPath, modifiedXML, nil
}

func buildAdoptRequest(preview *VMMigrationPreview) MigrationAdoptRequest {
	req := MigrationAdoptRequest{
		VMName:                preview.VMName,
		Owner:                 preview.Owner,
		CloudType:             preview.CloudType,
		TargetSwitchID:        preview.TargetSwitchID,
		TargetSecurityGroupID: preview.TargetSecurityGroupID,
		User:                  loadUserSnapshot(preview.Owner),
		Credential:            preview.Credential,
		PortForwards:          preview.PortForwards,
	}
	if quota, err := service.GetLightweightVMQuota(preview.VMName); err == nil && quota != nil {
		req.LightweightQuota = &service.LightweightVMQuotaRequest{
			VMName:            preview.VMName,
			TrafficDownGB:     quota.TrafficDownGB,
			TrafficUpGB:       quota.TrafficUpGB,
			BandwidthDownMbps: quota.BandwidthDownMbps,
			BandwidthUpMbps:   quota.BandwidthUpMbps,
			MaxPortForwards:   quota.MaxPortForwards,
			MaxSnapshots:      quota.MaxSnapshots,
			MaxRuntimeHours:   quota.MaxRuntimeHours,
		}
	}
	req.User.CloudType = preview.CloudType
	if preview.IsLightweight {
		req.User.DedicatedVPCSwitchID = preview.TargetSwitchID
	}
	return req
}

func applyMigrationDiskPathsToXML(xmlText string, disks []MigrationDisk) string {
	for _, disk := range disks {
		if strings.TrimSpace(disk.SourcePath) == "" || strings.TrimSpace(disk.TargetPath) == "" || disk.SourcePath == disk.TargetPath {
			continue
		}
		xmlText = strings.ReplaceAll(xmlText, disk.SourcePath, disk.TargetPath)
	}
	return xmlText
}

func writeLocalFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件 %s 失败: %v", path, err)
	}
	return nil
}