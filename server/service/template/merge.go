package template

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"qvmhub/utils"
)

// normalizeMergeMode 归一化合并模式，非法值返回错误。
func normalizeMergeMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case TemplateMergeModeFlatten:
		return TemplateMergeModeFlatten, nil
	case TemplateMergeModeCommitToParent:
		return TemplateMergeModeCommitToParent, nil
	default:
		return "", fmt.Errorf("不支持的合并模式: %s", mode)
	}
}

// ParseMergeTemplateParams 从 JSON 解析合并参数并归一化模式。
func ParseMergeTemplateParams(jsonStr string) (*MergeTemplateParams, error) {
	var params MergeTemplateParams
	if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
		return nil, err
	}
	params.TemplateName = strings.TrimSpace(params.TemplateName)
	if params.TemplateName == "" {
		return nil, fmt.Errorf("参数解析失败: template_name 为空")
	}
	mode, err := normalizeMergeMode(params.Mode)
	if err != nil {
		return nil, err
	}
	params.Mode = mode
	return &params, nil
}

// vmStateOrUnknown 返回状态，空则 unknown（不依赖 HookFirstNonEmpty，便于单测）。
func vmStateOrUnknown(status string) string {
	if s := strings.TrimSpace(status); s != "" {
		return s
	}
	return "unknown"
}

// buildFlattenBlockers 计算模式一（平铺）阻塞项。
func buildFlattenBlockers(hasBacking bool, subtreeVMs []TemplateRelatedVM) []string {
	if !hasBacking {
		return []string{"模板已是独立镜像，无需合并"}
	}
	var blockers []string
	for _, vm := range subtreeVMs {
		if !isVMStateShutoff(vm.Status) {
			blockers = append(blockers, fmt.Sprintf("关联虚拟机 %s 当前状态为 %s，请先关机", vm.Name, vmStateOrUnknown(vm.Status)))
		}
	}
	return blockers
}

// buildCommitBlockers 计算模式二（回写父模板）阻塞项。
func buildCommitBlockers(hasParent bool, parentDirectVMs []TemplateRelatedVM, parentOtherChildren []TemplateInfo, subtreeVMs []TemplateRelatedVM) []string {
	var blockers []string
	if !hasParent {
		blockers = append(blockers, "根模板没有父节点，无法回写")
	}
	if len(parentDirectVMs) > 0 {
		names := make([]string, 0, len(parentDirectVMs))
		for _, vm := range parentDirectVMs {
			names = append(names, vm.Name)
		}
		blockers = append(blockers, fmt.Sprintf("父模板存在直接依赖虚拟机（%s），不允许回写", strings.Join(names, "、")))
	}
	if len(parentOtherChildren) > 0 {
		names := make([]string, 0, len(parentOtherChildren))
		for _, c := range parentOtherChildren {
			names = append(names, c.Name)
		}
		blockers = append(blockers, fmt.Sprintf("父模板存在其它子模板（%s），不允许回写", strings.Join(names, "、")))
	}
	for _, vm := range subtreeVMs {
		if !isVMStateShutoff(vm.Status) {
			blockers = append(blockers, fmt.Sprintf("关联虚拟机 %s 当前状态为 %s，请先关机", vm.Name, vmStateOrUnknown(vm.Status)))
		}
	}
	return blockers
}

// parentOtherChildrenList 返回父模板下除指定节点外的其它子模板。
func parentOtherChildrenList(tree *templateTreeData, parent *TemplateInfo, excludeNodeID string) []TemplateInfo {
	if parent == nil {
		return nil
	}
	children := directChildTemplates(tree, parent.NodeID)
	out := make([]TemplateInfo, 0, len(children))
	for _, c := range children {
		if c.NodeID != excludeNodeID {
			out = append(out, c)
		}
	}
	return out
}

// GetMergePreview 返回模板合并预览（供前端渲染确认弹窗）。
func GetMergePreview(templateName string) (*MergePreview, error) {
	tree, err := buildTemplateTreeData()
	if err != nil {
		return nil, err
	}
	tpl, ok := tree.byName[templateName]
	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", templateName)
	}

	var parent *TemplateInfo
	if pid := strings.TrimSpace(tpl.ParentNodeID); pid != "" {
		if p, ok := tree.byNodeID[pid]; ok {
			pp := p
			parent = &pp
		}
	}

	// 物理上是否有 backing（元数据 ParentNodeID 与物理 backing 任一判定）。
	hasBacking := strings.TrimSpace(tpl.ParentNodeID) != ""
	if chain, qErr := HookQemuInfoChain(tpl.Path); qErr == nil && len(chain) >= 2 {
		hasBacking = true
	} else if qErr != nil {
		// qemu-img 读取失败时不阻断预览，仅以元数据为准。
	}

	// subtreeVMs 同时用于两模式：两者都要求 B 子树 VM 全部关机；MergeTemplate 的 ExpectedVMs 比对依赖此同源切片。
	subtreeVMs := hydrateTemplateRelatedVMs(collectTemplateSubtreeVMs(tree, tpl.NodeID))
	flattenBlockers := buildFlattenBlockers(hasBacking, subtreeVMs)

	var parentDirectVMs []TemplateRelatedVM
	if parent != nil {
		parentDirectVMs = hydrateTemplateRelatedVMs(filterLinkedVMs(tree.vmByNode[parent.NodeID]))
	}
	parentOtherChildren := parentOtherChildrenList(tree, parent, tpl.NodeID)
	childTemplates := directChildTemplates(tree, tpl.NodeID)
	commitBlockers := buildCommitBlockers(parent != nil, parentDirectVMs, parentOtherChildren, subtreeVMs)

	return &MergePreview{
		Template:       tpl,
		ParentTemplate: parent,
		IsIncremental:  hasBacking,
		Flatten: MergeFlattenPreview{
			Can:        len(flattenBlockers) == 0,
			Blockers:   flattenBlockers,
			SubtreeVMs: subtreeVMs,
		},
		CommitToParent: MergeCommitPreview{
			Can:                 len(commitBlockers) == 0,
			Blockers:            commitBlockers,
			ParentDirectVMs:     parentDirectVMs,
			ParentOtherChildren: parentOtherChildren,
			ChildTemplates:      childTemplates,
			SubtreeVMs:          subtreeVMs,
		},
	}, nil
}

// buildFlattenConvertCmd 构造平铺用的 qemu-img convert 命令（自动拉平整条 backing 链）。
// compress 为 true 时加 -c 用 zlib 压缩各簇：产物体积更小，代价是子节点/虚拟机读取时需解压。
func buildFlattenConvertCmd(src, dst string, compress bool) string {
	flags := "-f qcow2 -O qcow2"
	if compress {
		flags = "-c -f qcow2 -O qcow2"
	}
	return fmt.Sprintf("qemu-img convert %s %s %s",
		flags, utils.ShellSingleQuote(src), utils.ShellSingleQuote(dst))
}

// mergeTemplateFlatten 模式一：把增量模板 B 平铺为独立镜像（原地替换）。
func mergeTemplateFlatten(params *MergeTemplateParams, preview *MergePreview, progressFn func(int, string)) (*MergeTemplateResult, error) {
	if preview == nil {
		return nil, fmt.Errorf("合并预览为空")
	}
	if len(preview.Flatten.Blockers) > 0 {
		return nil, fmt.Errorf("无法平铺: %s", strings.Join(preview.Flatten.Blockers, "；"))
	}
	bPath, err := EnsureTemplatePath(params.TemplateName)
	if err != nil {
		return nil, err
	}
	tree, err := buildTemplateTreeData()
	if err != nil {
		return nil, err
	}
	bTpl, ok := tree.byName[params.TemplateName]
	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", params.TemplateName)
	}

	tmpPath := bPath + ".merge-" + time.Now().Format("20060102150405")
	backupPath := bPath + ".merge-old-" + time.Now().Format("20060102150405")

	progressFn(15, "正在平铺 backing 链为独立镜像...")
	convertCmd := buildFlattenConvertCmd(bPath, tmpPath, params.Compress)
	if r := utils.ExecShellContextWithTimeout(context.Background(), convertCmd, 2*time.Hour); r.Error != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("平铺磁盘失败: %s", r.Stderr)
	}

	progressFn(70, "正在原地替换模板文件...")
	_ = utils.RemoveFileImmutable(bPath) // 解除不可变以允许 rename
	if err := os.Rename(bPath, backupPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("备份原模板失败: %w", err)
	}
	if err := os.Rename(tmpPath, bPath); err != nil {
		_ = os.Rename(backupPath, bPath) // 回滚
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("替换模板文件失败: %w", err)
	}
	_ = utils.ChownLibvirtQEMU(bPath)
	_ = utils.SetFileImmutable(bPath)
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		// 非致命：旧备份残留，记日志即可
	}

	progressFn(85, "正在更新模板元数据...")
	newUID := generateTemplateID("tpl")
	meta := GetTemplateMeta(params.TemplateName)
	if meta == nil {
		meta = &TemplateMeta{}
	}
	meta.ParentNodeID = ""
	meta.RootNodeID = bTpl.NodeID
	meta.TemplateUID = newUID
	if hash, err := CalculateFileHashes(bPath); err == nil {
		meta.MD5, meta.SHA256, meta.FileSize = hash.MD5, hash.SHA256, hash.FileSize
	}
	if err := saveTemplateMeta(bPath, meta); err != nil {
		return nil, err
	}

	// 子树族归一：B 的后代 RootNodeID/TemplateUID 同步到 B 的新族（保持按 template_uid 分族展示一致）。
	for _, n := range collectTemplateSubtree(tree, bTpl.NodeID) {
		if n.NodeID == bTpl.NodeID {
			continue
		}
		if m := loadTemplateMeta(n.Path); m != nil {
			m.RootNodeID = bTpl.NodeID
			m.TemplateUID = newUID
			_ = saveTemplateMeta(n.Path, m)
		}
	}

	progressFn(100, "模板已平铺为独立镜像")
	return &MergeTemplateResult{
		TemplateName: params.TemplateName,
		Mode:         TemplateMergeModeFlatten,
		Flattened:    true,
	}, nil
}

// MergeTemplate 合并模板（异步任务逻辑）。执行前重跑预览做服务端校验防竞态。
func MergeTemplate(params *MergeTemplateParams, progressFn func(int, string)) (*MergeTemplateResult, error) {
	if progressFn == nil {
		progressFn = func(int, string) {}
	}
	if params == nil || strings.TrimSpace(params.TemplateName) == "" {
		return nil, fmt.Errorf("模板名称不能为空")
	}
	mode, err := normalizeMergeMode(params.Mode)
	if err != nil {
		return nil, err
	}
	params.Mode = mode

	progressFn(5, "正在校验合并条件...")
	preview, err := GetMergePreview(params.TemplateName)
	if err != nil {
		return nil, err
	}

	// 防竞态：B 子树 VM 列表须与前端预览时一致。
	currentVMs := make([]string, 0, len(preview.Flatten.SubtreeVMs))
	for _, vm := range preview.Flatten.SubtreeVMs {
		currentVMs = append(currentVMs, vm.Name)
	}
	if len(params.ExpectedVMs) > 0 && !sameStringSet(currentVMs, params.ExpectedVMs) {
		return nil, fmt.Errorf("模板关联虚拟机列表已发生变化，请刷新页面后重新确认")
	}

	switch mode {
	case TemplateMergeModeFlatten:
		return mergeTemplateFlatten(params, preview, progressFn)
	case TemplateMergeModeCommitToParent:
		return mergeTemplateCommitToParent(params, preview, progressFn)
	}
	return nil, fmt.Errorf("不支持的合并模式: %s", mode)
}

// mergeTemplateCommitToParent 模式二：把 B 的增量回写到父 A（commit），B 的子模板/VM 改挂到 A，最后删除 B。
func mergeTemplateCommitToParent(params *MergeTemplateParams, preview *MergePreview, progressFn func(int, string)) (*MergeTemplateResult, error) {
	if preview == nil {
		return nil, fmt.Errorf("合并预览为空")
	}
	if len(preview.CommitToParent.Blockers) > 0 {
		return nil, fmt.Errorf("无法回写到父模板: %s", strings.Join(preview.CommitToParent.Blockers, "；"))
	}
	parent := preview.ParentTemplate
	if parent == nil {
		return nil, fmt.Errorf("根模板没有父节点，无法回写")
	}
	bPath, err := EnsureTemplatePath(params.TemplateName)
	if err != nil {
		return nil, err
	}
	aPath, err := EnsureTemplatePath(parent.Name)
	if err != nil {
		return nil, fmt.Errorf("父模板不可用: %w", err)
	}

	// 1) commit：把 B 增量写入 A（需先解 A 不可变，commit 后立即恢复）。
	progressFn(15, "正在把增量回写到父模板...")
	_ = utils.RemoveFileImmutable(aPath)
	if r := utils.ExecCommandWithTimeout("qemu-img", 6*time.Hour, "commit", "-f", "qcow2", bPath); r.Error != nil {
		_ = utils.SetFileImmutable(aPath) // 恢复不可变再返回
		return nil, fmt.Errorf("回写增量失败: %s", r.Stderr)
	}
	_ = utils.SetFileImmutable(aPath)

	// 2) B 的直接子模板 C 改挂到 A（rebase + 族归一）。
	tree, err := buildTemplateTreeData()
	if err != nil {
		return nil, err
	}
	rebasedTemplates := make([]string, 0, len(preview.CommitToParent.ChildTemplates))
	for i, c := range preview.CommitToParent.ChildTemplates {
		progressFn(30+((i+1)*30/maxInt(len(preview.CommitToParent.ChildTemplates), 1)), fmt.Sprintf("正在改挂子模板 %s ...", c.AdminName))
		_ = utils.RemoveFileImmutable(c.Path)
		if err := rebaseQcow2BackingToParent(c.Path, bPath, aPath); err != nil {
			_ = utils.SetFileImmutable(c.Path) // 尽力恢复不可变再返回
			return nil, fmt.Errorf("改挂子模板 %s 失败: %w", c.AdminName, err)
		}
		_ = utils.SetFileImmutable(c.Path)
		if err := updatePromotedTemplateMeta(c, *parent); err != nil {
			return nil, err
		}
		// C 的后代族归一到 A 族。
		for _, n := range collectTemplateSubtree(tree, c.NodeID) {
			if n.NodeID == c.NodeID {
				continue
			}
			if m := loadTemplateMeta(n.Path); m != nil {
				m.RootNodeID = parent.RootNodeID
				m.TemplateUID = parent.TemplateUID
				_ = saveTemplateMeta(n.Path, m)
			}
		}
		rebasedTemplates = append(rebasedTemplates, c.Name)
	}

	// 3) B 的直接 linked VM 改挂到 A。
	directVMs := directVMsOfNode(tree, params.TemplateName)
	rebasedVMs := make([]string, 0, len(directVMs))
	for i, vm := range directVMs {
		progressFn(60+((i+1)*20/maxInt(len(directVMs), 1)), fmt.Sprintf("正在改挂虚拟机 %s ...", vm.Name))
		di := HookGetVMDiskInfo(vm.Name)
		if strings.TrimSpace(di.Path) == "" {
			return nil, fmt.Errorf("无法获取虚拟机 %s 的磁盘路径", vm.Name)
		}
		if err := rebaseQcow2BackingToParent(di.Path, bPath, aPath); err != nil {
			return nil, fmt.Errorf("改挂虚拟机 %s 失败: %w", vm.Name, err)
		}
		if err := WriteVMTemplateSource(vm.Name, parent.Name, "linked"); err != nil {
			return nil, err
		}
		rebasedVMs = append(rebasedVMs, vm.Name)
	}

	// 4) B 已无依赖，删除。
	progressFn(85, "正在删除已合并的模板节点...")
	if err := deleteTemplateFiles(params.TemplateName); err != nil {
		return nil, err
	}

	// 5) 重算 A 的 hash。
	if meta := GetTemplateMeta(parent.Name); meta != nil {
		if hash, err := CalculateFileHashes(aPath); err == nil {
			meta.MD5, meta.SHA256, meta.FileSize = hash.MD5, hash.SHA256, hash.FileSize
			_ = saveTemplateMeta(aPath, meta)
		}
	}

	progressFn(100, "增量已回写到父模板，当前模板节点已删除")
	return &MergeTemplateResult{
		TemplateName:     params.TemplateName,
		Mode:             TemplateMergeModeCommitToParent,
		DeletedTemplates: []string{params.TemplateName},
		RebasedTemplates: rebasedTemplates,
		RebasedVMs:       rebasedVMs,
	}, nil
}

// directVMsOfNode 返回直接挂在某模板名下的 linked VM（取自树 vmByNode）。
func directVMsOfNode(tree *templateTreeData, templateName string) []TemplateRelatedVM {
	tpl, ok := tree.byName[templateName]
	if !ok {
		return nil
	}
	return filterLinkedVMs(tree.vmByNode[tpl.NodeID])
}
