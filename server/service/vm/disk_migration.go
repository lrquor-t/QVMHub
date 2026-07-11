package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"qvmhub/utils"
)

// VMDiskMigrationRequest 是提交本机硬盘迁移任务的请求体。
type VMDiskMigrationRequest struct {
	TargetStoragePoolID string `json:"target_storage_pool_id"`
}

// VMDiskMigrationTaskParams 是任务队列保存的硬盘迁移参数。
type VMDiskMigrationTaskParams struct {
	VMName              string `json:"vm_name"`
	Device              string `json:"device"`
	TargetStoragePoolID string `json:"target_storage_pool_id"`
}

// VMDiskMigrationOptions 是迁移硬盘弹窗需要的表单选项。
type VMDiskMigrationOptions struct {
	VMName               string                `json:"vm_name"`
	SourceState          string                `json:"source_state"`
	Mode                 string                `json:"mode"`
	Disks                []VMDiskMigrationDisk `json:"disks"`
	TargetStorageTargets []VMStorageTarget     `json:"target_storage_targets"`
	Warnings             []string              `json:"warnings,omitempty"`
}

// VMDiskMigrationDisk 描述当前 VM 中可展示的磁盘迁移候选项。
type VMDiskMigrationDisk struct {
	Device              string `json:"device"`
	Path                string `json:"path"`
	CapacityGB          string `json:"capacity_gb"`
	UsedGB              string `json:"used_gb"`
	Bus                 string `json:"bus"`
	Format              string `json:"format"`
	BackingPath         string `json:"backing_path"`
	VirtualSize         int64  `json:"virtual_size"`
	ActualSize          int64  `json:"actual_size"`
	CanMigrate          bool   `json:"can_migrate"`
	BlockReason         string `json:"block_reason,omitempty"`
	SourceStoragePoolID string `json:"source_storage_pool_id,omitempty"`
}

type vmDiskMigrationPlan struct {
	VMName        string
	Device        string
	Mode          string
	SourceState   string
	SourcePath    string
	TargetPath    string
	Format        string
	BackingPath   string
	BackingFormat string
	VirtualSize   int64
	ActualSize    int64
	TargetStorage VMStorageTarget
}

// ParseVMDiskMigrationTaskParams 解析硬盘迁移任务参数。
func ParseVMDiskMigrationTaskParams(raw string) (VMDiskMigrationTaskParams, error) {
	var params VMDiskMigrationTaskParams
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return params, err
	}
	return params, nil
}

// GetVMDiskMigrationOptions 返回本机硬盘迁移表单选项。
func GetVMDiskMigrationOptions(vmName string) (*VMDiskMigrationOptions, error) {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return nil, fmt.Errorf("虚拟机名称不能为空")
	}
	if !DomainExists(vmName) {
		return nil, fmt.Errorf("虚拟机不存在")
	}

	state := GetDomainState(vmName)
	options := &VMDiskMigrationOptions{
		VMName:      vmName,
		SourceState: state,
		Mode:        D.HookDetectMigrationModeFromState(state),
	}
	if err := validateVMDiskMigrationState(state); err != nil {
		options.Warnings = append(options.Warnings, err.Error())
	}

	targets, err := D.ListVMStorageTargets(true)
	if err != nil {
		return nil, fmt.Errorf("读取目标存储失败: %w", err)
	}
	options.TargetStorageTargets = targets

	if hasExternal, names, err := D.CheckVMSnapshotSafety(vmName); err != nil {
		options.Warnings = append(options.Warnings, "读取快照状态失败: "+err.Error())
	} else if hasExternal {
		options.Warnings = append(options.Warnings, "虚拟机存在外部快照，迁移硬盘前请先删除这些快照: "+strings.Join(names, "、"))
	}

	disks, err := listVMDiskMigrationCandidates(vmName)
	if err != nil {
		return nil, err
	}
	options.Disks = disks
	return options, nil
}

// ExecuteVMDiskMigration 执行本机硬盘迁移。
func ExecuteVMDiskMigration(ctx context.Context, params VMDiskMigrationTaskParams, progress func(int, string)) (map[string]string, error) {
	if progress == nil {
		progress = func(int, string) {}
	}
	progress(5, "正在检查硬盘迁移参数...")
	plan, err := buildVMDiskMigrationPlan(params)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	progress(12, fmt.Sprintf("硬盘 %s 将按%s迁移到目标存储...", plan.Device, diskMigrationModeLabel(plan.Mode)))
	if plan.Mode == D.HookMigrationModeLive() {
		err = executeLiveVMDiskMigration(ctx, plan, progress)
	} else {
		err = executeColdVMDiskMigration(ctx, plan, progress)
	}
	if err != nil {
		return nil, err
	}

	progress(100, fmt.Sprintf("硬盘 %s 已迁移到目标存储", plan.Device))
	return map[string]string{
		"vm_name":     plan.VMName,
		"device":      plan.Device,
		"mode":        plan.Mode,
		"source_path": plan.SourcePath,
		"target_path": plan.TargetPath,
	}, nil
}

func buildVMDiskMigrationPlan(params VMDiskMigrationTaskParams) (*vmDiskMigrationPlan, error) {
	vmName := strings.TrimSpace(params.VMName)
	device := strings.TrimSpace(params.Device)
	poolID := strings.TrimSpace(params.TargetStoragePoolID)
	if vmName == "" {
		return nil, fmt.Errorf("虚拟机名称不能为空")
	}
	if device == "" {
		return nil, fmt.Errorf("请选择要迁移的硬盘")
	}
	if poolID == "" {
		return nil, fmt.Errorf("请选择目标存储")
	}
	if !DomainExists(vmName) {
		return nil, fmt.Errorf("虚拟机不存在")
	}
	if hasExternal, names, err := D.CheckVMSnapshotSafety(vmName); err != nil {
		return nil, fmt.Errorf("检查快照状态失败: %w", err)
	} else if hasExternal {
		return nil, fmt.Errorf("虚拟机存在外部快照（%s），请先删除这些快照后再迁移硬盘", strings.Join(names, "、"))
	}

	targetDir, resolvedPoolID, err := D.ResolveVMStorageDir(poolID, true)
	if err != nil {
		return nil, err
	}
	targetStorage, err := findLocalVMStorageTarget(resolvedPoolID)
	if err != nil {
		return nil, err
	}

	candidates, err := listVMDiskMigrationCandidates(vmName)
	if err != nil {
		return nil, err
	}
	var disk VMDiskMigrationDisk
	found := false
	for _, item := range candidates {
		if item.Device == device {
			disk = item
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("未找到硬盘设备: %s", device)
	}
	if !disk.CanMigrate {
		if disk.BlockReason != "" {
			return nil, fmt.Errorf("%s", disk.BlockReason)
		}
		return nil, fmt.Errorf("该硬盘不能迁移")
	}

	if SameCleanPath(filepath.Dir(disk.Path), targetDir) {
		return nil, fmt.Errorf("硬盘已位于目标存储目录")
	}
	targetPath, err := buildUniqueDiskMigrationTargetPath(targetDir, disk.Path, time.Now())
	if err != nil {
		return nil, err
	}
	required := disk.ActualSize
	if required <= 0 {
		required = disk.VirtualSize
	}
	if targetStorage.Available > 0 && required > targetStorage.Available {
		return nil, fmt.Errorf("目标存储可用空间不足")
	}

	state := GetDomainState(vmName)
	if err := validateVMDiskMigrationState(state); err != nil {
		return nil, err
	}
	return &vmDiskMigrationPlan{
		VMName:        vmName,
		Device:        device,
		Mode:          D.HookDetectMigrationModeFromState(state),
		SourceState:   state,
		SourcePath:    disk.Path,
		TargetPath:    targetPath,
		Format:        disk.Format,
		BackingPath:   disk.BackingPath,
		BackingFormat: backingFormatForDisk(disk.Path),
		VirtualSize:   disk.VirtualSize,
		ActualSize:    disk.ActualSize,
		TargetStorage: targetStorage,
	}, nil
}

func validateVMDiskMigrationState(state string) error {
	normalized := strings.ToLower(strings.TrimSpace(state))
	if strings.Contains(normalized, "running") || strings.Contains(normalized, "shut off") {
		return nil
	}
	return fmt.Errorf("当前虚拟机状态不支持硬盘迁移，请先开机执行热迁移或关机执行冷迁移")
}

func listVMDiskMigrationCandidates(vmName string) ([]VMDiskMigrationDisk, error) {
	detailResult := utils.ExecCommand("virsh", "domblklist", vmName, "--details")
	if detailResult.Error != nil {
		return nil, fmt.Errorf("读取虚拟机硬盘失败: %s", detailResult.Stderr)
	}
	diskInfoMap := map[string]DiskInfo{}
	if disks, err := D.ListDisks(vmName); err == nil {
		for _, disk := range disks {
			diskInfoMap[disk.Device] = disk
		}
	}

	var result []VMDiskMigrationDisk
	for _, line := range strings.Split(detailResult.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] == "Type" || strings.HasPrefix(line, "-") {
			continue
		}
		diskType := fields[0]
		deviceType := fields[1]
		target := fields[2]
		source := strings.Join(fields[3:], " ")
		if deviceType != "disk" || source == "" || source == "-" {
			continue
		}
		if diskType != "file" {
			result = append(result, VMDiskMigrationDisk{
				Device:      target,
				Path:        source,
				CanMigrate:  false,
				BlockReason: "仅支持 file 类型硬盘迁移",
			})
			continue
		}

		item := VMDiskMigrationDisk{
			Device:     target,
			Path:       source,
			CanMigrate: true,
		}
		if info, ok := diskInfoMap[target]; ok {
			item.CapacityGB = info.CapacityGB
			item.UsedGB = info.UsedGB
			item.Bus = info.Bus
			item.Format = info.Format
		}

		chain, err := QemuInfoChain(source)
		if err != nil {
			item.CanMigrate = false
			item.BlockReason = "读取硬盘镜像信息失败: " + err.Error()
			result = append(result, item)
			continue
		}
		if len(chain) == 0 {
			item.CanMigrate = false
			item.BlockReason = "硬盘镜像链为空"
			result = append(result, item)
			continue
		}
		item.Format = D.FirstNonEmpty(item.Format, chain[0].Format)
		item.VirtualSize = chain[0].VirtualSize
		item.ActualSize = chain[0].ActualSize
		if item.ActualSize <= 0 {
			if st, statErr := os.Stat(source); statErr == nil {
				item.ActualSize = st.Size()
			}
		}
		if item.CapacityGB == "" || item.CapacityGB == "-" {
			item.CapacityGB = bytesToGBString(item.VirtualSize)
		}
		if item.UsedGB == "" || item.UsedGB == "-" {
			item.UsedGB = bytesToGBString(item.ActualSize)
		}
		if len(chain) > 1 {
			item.BackingPath = D.FirstNonEmpty(chain[0].FullBackingFilename, chain[0].BackingFilename, chain[1].Filename)
		}
		if strings.TrimSpace(item.Format) == "" {
			item.CanMigrate = false
			item.BlockReason = "硬盘格式未知，无法迁移"
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("未找到可迁移的 file 类型硬盘")
	}
	return result, nil
}

func executeLiveVMDiskMigration(ctx context.Context, plan *vmDiskMigrationPlan, progress func(int, string)) error {
	if strings.TrimSpace(plan.Format) == "" {
		return fmt.Errorf("硬盘格式未知，无法热迁移")
	}
	reuseExternal := false
	if plan.BackingPath != "" {
		progress(18, "正在预创建链式硬盘目标 overlay...")
		if err := prepareLiveShallowDiskTarget(ctx, plan); err != nil {
			return err
		}
		reuseExternal = true
	}
	progress(20, "正在启动热迁移 blockcopy...")
	cmdParts := []string{
		"virsh blockcopy",
		utils.ShellSingleQuote(plan.VMName),
		utils.ShellSingleQuote(plan.Device),
		"--dest", utils.ShellSingleQuote(plan.TargetPath),
		"--format", utils.ShellSingleQuote(plan.Format),
		"--wait --verbose --pivot --transient-job",
	}
	if plan.BackingPath != "" {
		cmdParts = append(cmdParts, "--shallow")
	}
	if reuseExternal {
		cmdParts = append(cmdParts, "--reuse-external")
	}
	cmd := strings.Join(cmdParts, " ")
	result := utils.ExecShellContextWithTimeout(ctx, cmd, 8*time.Hour)
	if result.Error != nil {
		_ = abortLiveDiskBlockJob(plan)
		_ = os.Remove(plan.TargetPath)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("热迁移硬盘失败: %s", D.FirstNonEmpty(result.Stderr, result.Error.Error()))
	}

	progress(86, "正在同步持久化 XML...")
	if err := UpdateInactiveDomainDiskPath(plan.VMName, plan.SourcePath, plan.TargetPath); err != nil {
		return err
	}
	_ = SetLibvirtDiskFileOwner(plan.TargetPath)

	progress(94, "正在清理源硬盘文件...")
	if err := removeSourceDiskAfterPivot(plan); err != nil {
		return err
	}
	return nil
}

func prepareLiveShallowDiskTarget(ctx context.Context, plan *vmDiskMigrationPlan) error {
	if plan.VirtualSize <= 0 {
		return fmt.Errorf("链式硬盘缺少容量信息，无法热迁移")
	}
	if strings.TrimSpace(plan.BackingFormat) == "" {
		return fmt.Errorf("链式硬盘缺少 backing 格式，无法热迁移")
	}
	if err := os.MkdirAll(filepath.Dir(plan.TargetPath), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}
	cmd := "qemu-img create -f " + utils.ShellSingleQuote(plan.Format) +
		" -F " + utils.ShellSingleQuote(plan.BackingFormat) +
		" -b " + utils.ShellSingleQuote(plan.BackingPath) +
		" " + utils.ShellSingleQuote(plan.TargetPath) +
		" " + strconv.FormatInt(plan.VirtualSize, 10)
	result := utils.ExecShellContextWithTimeout(ctx, cmd, 10*time.Minute)
	if result.Error != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("创建热迁移目标硬盘失败: %s", D.FirstNonEmpty(result.Stderr, result.Error.Error()))
	}
	_ = SetLibvirtDiskFileOwner(plan.TargetPath)
	return nil
}

func executeColdVMDiskMigration(ctx context.Context, plan *vmDiskMigrationPlan, progress func(int, string)) error {
	progress(20, "正在复制硬盘到目标存储...")
	if err := CopyDiskFileSparse(ctx, plan.SourcePath, plan.TargetPath); err != nil {
		_ = os.Remove(plan.TargetPath)
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = os.Remove(plan.TargetPath)
		return err
	}
	_ = SetLibvirtDiskFileOwner(plan.TargetPath)

	if plan.BackingPath != "" {
		progress(62, "正在修正链式硬盘 backing 路径...")
		if err := rebaseDiskBackingUnsafe(plan); err != nil {
			_ = os.Remove(plan.TargetPath)
			return err
		}
	}

	progress(78, "正在更新虚拟机持久化 XML...")
	if err := UpdateInactiveDomainDiskPath(plan.VMName, plan.SourcePath, plan.TargetPath); err != nil {
		_ = os.Remove(plan.TargetPath)
		return err
	}

	progress(92, "正在删除源硬盘文件...")
	if err := os.Remove(plan.SourcePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("硬盘已迁移，但删除源文件失败: %w", err)
	}
	return nil
}

func UpdateInactiveDomainDiskPath(vmName, sourcePath, targetPath string) error {
	xmlText, err := GetVMInactiveDomainXML(vmName)
	if err != nil {
		return err
	}
	if !strings.Contains(xmlText, sourcePath) {
		if strings.Contains(xmlText, targetPath) {
			return nil
		}
		return fmt.Errorf("持久化 XML 中未找到源硬盘路径，迁移已停止")
	}
	nextXML := strings.ReplaceAll(xmlText, sourcePath, targetPath)
	if err := SetVMInactiveDomainXML(vmName, nextXML); err != nil {
		return fmt.Errorf("更新虚拟机硬盘路径失败: %w", err)
	}
	return nil
}

func CopyDiskFileSparse(ctx context.Context, sourcePath, targetPath string) error {
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("源硬盘文件不可读: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}
	cmd := "cp --sparse=always --reflink=auto --preserve=mode,ownership,timestamps " +
		utils.ShellSingleQuote(sourcePath) + " " + utils.ShellSingleQuote(targetPath)
	result := utils.ExecShellContextWithTimeout(ctx, cmd, 8*time.Hour)
	if result.Error != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("复制硬盘文件失败: %s", D.FirstNonEmpty(result.Stderr, result.Error.Error()))
	}
	return nil
}

func rebaseDiskBackingUnsafe(plan *vmDiskMigrationPlan) error {
	if strings.TrimSpace(plan.BackingPath) == "" {
		return nil
	}
	backingFormat := strings.TrimSpace(plan.BackingFormat)
	if backingFormat == "" {
		return fmt.Errorf("链式硬盘缺少 backing 格式，无法迁移")
	}
	cmd := "qemu-img rebase -u -f " + utils.ShellSingleQuote(plan.Format) +
		" -F " + utils.ShellSingleQuote(backingFormat) +
		" -b " + utils.ShellSingleQuote(plan.BackingPath) +
		" " + utils.ShellSingleQuote(plan.TargetPath)
	result := utils.ExecShellContextWithTimeout(context.Background(), cmd, 10*time.Minute)
	if result.Error != nil {
		return fmt.Errorf("修正 backing 路径失败: %s", D.FirstNonEmpty(result.Stderr, result.Error.Error()))
	}
	return nil
}

func abortLiveDiskBlockJob(plan *vmDiskMigrationPlan) error {
	result := utils.ExecCommand("virsh", "blockjob", plan.VMName, plan.Device, "--abort", "--async")
	if result.Error != nil {
		return fmt.Errorf("取消硬盘热迁移任务失败: %s", result.Stderr)
	}
	return nil
}

func removeSourceDiskAfterPivot(plan *vmDiskMigrationPlan) error {
	currentPath := D.GetDiskFilePath(plan.VMName, plan.Device)
	if currentPath != "" && !SameCleanPath(currentPath, plan.TargetPath) {
		return fmt.Errorf("硬盘已复制，但当前运行态仍指向源文件，已跳过删除源文件")
	}
	if SameCleanPath(plan.SourcePath, plan.TargetPath) {
		return nil
	}
	if err := os.Remove(plan.SourcePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("硬盘已迁移，但删除源文件失败: %w", err)
	}
	return nil
}

func SetLibvirtDiskFileOwner(path string) error {
	result := utils.ExecCommand("chown", "libvirt-qemu:kvm", path)
	if result.Error != nil {
		return fmt.Errorf("设置硬盘文件权限失败: %s", result.Stderr)
	}
	return nil
}

func findLocalVMStorageTarget(id string) (VMStorageTarget, error) {
	targets, err := D.ListVMStorageTargets(true)
	if err != nil {
		return VMStorageTarget{}, err
	}
	for _, target := range targets {
		if target.ID == id && target.Enabled && strings.TrimSpace(target.VMDir) != "" {
			return target, nil
		}
	}
	return VMStorageTarget{}, fmt.Errorf("目标存储不存在或不可用于 VM")
}

func buildUniqueDiskMigrationTargetPath(targetDir, sourcePath string, now time.Time) (string, error) {
	if strings.TrimSpace(targetDir) == "" {
		return "", fmt.Errorf("目标存储目录为空")
	}
	if strings.TrimSpace(sourcePath) == "" {
		return "", fmt.Errorf("源硬盘路径为空")
	}
	base := filepath.Base(sourcePath)
	if base == "." || base == string(filepath.Separator) {
		return "", fmt.Errorf("源硬盘文件名无效")
	}
	candidate := filepath.Join(targetDir, base)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate, nil
	} else if err != nil {
		return "", fmt.Errorf("检查目标文件失败: %w", err)
	}
	ext := filepath.Ext(base)
	nameOnly := strings.TrimSuffix(base, ext)
	if nameOnly == "" {
		nameOnly = "disk"
	}
	stamp := now.Format("20060102150405")
	for i := 0; i < 100; i++ {
		suffix := stamp
		if i > 0 {
			suffix = suffix + "-" + strconv.Itoa(i)
		}
		candidate = filepath.Join(targetDir, fmt.Sprintf("%s_migrated_%s%s", nameOnly, suffix, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("检查目标文件失败: %w", err)
		}
	}
	return "", fmt.Errorf("无法生成不冲突的目标硬盘路径")
}

func backingFormatForDisk(path string) string {
	chain, err := QemuInfoChain(path)
	if err != nil || len(chain) < 2 {
		return ""
	}
	return chain[1].Format
}

func SameCleanPath(a, b string) bool {
	return filepath.Clean(strings.TrimSpace(a)) == filepath.Clean(strings.TrimSpace(b))
}

func bytesToGBString(value int64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", float64(value)/1024/1024/1024)
}

func diskMigrationModeLabel(mode string) string {
	if mode == D.HookMigrationModeLive() {
		return "热迁移"
	}
	return "冷迁移"
}
