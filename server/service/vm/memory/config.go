package memory

import (
	"fmt"
	"time"

	"github.com/digitalocean/go-libvirt"
	"qvmhub/service/libvirt_rpc"
)

// SetVMMemoryDynamicConfig 由管理员启用/修改动态内存配置，运行中 VM 只写待应用 metadata。
func SetVMMemoryDynamicConfig(name string, req *VMMemoryDynamicRequest) (string, error) {
	if req == nil {
		return "", nil
	}

	xmlResult, err := libvirt_rpc.GetDomainXMLRPC(name, libvirt.DomainXMLInactive)
	if err != nil {
		return "", fmt.Errorf("获取虚拟机 XML 失败: %w", err)
	}
	state, _ := libvirt_rpc.GetDomainStateRPC(name)
	values := ParseDomainMemoryXML(xmlResult)
	if values.CurrentMemoryMB <= 0 {
		values.CurrentMemoryMB = values.MemoryMB
	}

	if req.DynamicEnabled != nil && !*req.DynamicEnabled {
		targetMB := values.CurrentMemoryMB
		if req.MemoryInitial > 0 {
			targetMB = req.MemoryInitial * 1024
		}
		if targetMB <= 0 {
			targetMB = values.MemoryMB
		}
		if targetMB <= 0 {
			return "", fmt.Errorf("关闭动态内存失败: 未能确定静态内存大小")
		}
		if err := applyStaticVMMemoryToInactiveXML(name, targetMB); err != nil {
			return "", err
		}
		meta := &VMMemoryMetadata{
			Version:         1,
			DynamicEnabled:  false,
			MemoryBackend:   MemoryBackendBalloon,
			MemoryInitialMB: targetMB,
			MemoryMinMB:     DefaultMinMemoryMB(targetMB),
			MemoryMaxMB:     targetMB,
			AutoBalloon:     false,
			PendingApply:    false,
			UpdatedAt:       time.Now().Unix(),
		}
		if err := WriteVMMemoryMetadata(name, meta); err != nil {
			return "", err
		}
		return "动态内存已关闭", nil
	}

	initialGB := MBToRoundedGB(values.CurrentMemoryMB)
	if req.MemoryInitial > 0 {
		initialGB = req.MemoryInitial
	}
	maxGB := DefaultDynamicMemoryMaxGB(initialGB)
	if req.MemoryMax > 0 {
		maxGB = req.MemoryMax
	}
	minGB := DefaultDynamicMemoryMinGB(initialGB)
	if req.MemoryMin > 0 {
		minGB = req.MemoryMin
	}
	autoBalloon := true
	if req.AutoBalloon != nil {
		autoBalloon = *req.AutoBalloon
	}
	backend := NormalizeMemoryBackend(req.MemoryBackend)
	if backend == MemoryBackendVirtioMem {
		minGB = initialGB
		autoBalloon = false
	}

	meta, err := NewVMMemoryMetadataForBackend(initialGB*1024, minGB*1024, maxGB*1024, autoBalloon, state == "running", backend)
	if err != nil {
		return "", err
	}

	if backend == MemoryBackendVirtioMem {
		if state == "running" {
			oldMeta, _ := ReadVMMemoryMetadata(name)
			if oldMeta != nil &&
				oldMeta.DynamicEnabled &&
				NormalizeMemoryBackend(oldMeta.MemoryBackend) == MemoryBackendVirtioMem &&
				oldMeta.MemoryInitialMB == meta.MemoryInitialMB &&
				oldMeta.MemoryMaxMB == meta.MemoryMaxMB {
				return "Windows 弹性内存（实验）基础配置未变更", nil
			}
			return "", fmt.Errorf("Windows 弹性内存（实验）需要先关机后启用或修改基础配置")
		}
		if err := applyVMMemoryMetadataToInactiveXML(name, meta); err != nil {
			return "", err
		}
		if err := WriteVMMemoryMetadata(name, meta); err != nil {
			return "", err
		}
		return "Windows 弹性内存（实验）配置已应用", nil
	}

	if state == "running" {
		if !HasUsableMemballoon(xmlResult) {
			return "", fmt.Errorf("运行中的虚拟机未配置 virtio-balloon，请先关机后再启用动态内存")
		}
		if err := WriteVMMemoryMetadata(name, meta); err != nil {
			return "", err
		}
		return "动态内存配置已保存，将在下次关机后启动时应用", nil
	}

	if err := applyVMMemoryMetadataToInactiveXML(name, meta); err != nil {
		return "", err
	}
	meta.PendingApply = false
	if err := WriteVMMemoryMetadata(name, meta); err != nil {
		return "", err
	}
	return "动态内存配置已应用", nil
}

func applyStaticVMMemoryToInactiveXML(name string, memoryMB int) error {
	xmlResult, err := libvirt_rpc.GetDomainXMLRPC(name, libvirt.DomainXMLInactive)
	if err != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %w", err)
	}
	xmlStr, err := ApplyStaticMemoryConfigToDomainXML(xmlResult, memoryMB)
	if err != nil {
		return err
	}
	if _, err := libvirt_rpc.DefineDomainXMLRPC(xmlStr); err != nil {
		return fmt.Errorf("恢复静态内存配置失败: %w", err)
	}
	return nil
}

// MBToRoundedGB 将 MB 向上取整到 GB。
func MBToRoundedGB(mb int) int {
	if mb <= 0 {
		return 0
	}
	return (mb + 1023) / 1024
}

// SetVMMemoryCurrent 手动调整运行中 VM 当前内存，单位 MB。
func SetVMMemoryCurrent(name string, targetMB int, pauseAuto bool) error {
	if targetMB <= 0 {
		return fmt.Errorf("当前内存必须大于 0")
	}
	meta, _ := ReadVMMemoryMetadata(name)
	if meta != nil && meta.DynamicEnabled {
		if NormalizeMemoryBackend(meta.MemoryBackend) == MemoryBackendVirtioMem {
			if targetMB < meta.MemoryInitialMB || targetMB > meta.MemoryMaxMB {
				return fmt.Errorf("当前内存必须在 %dMB 到 %dMB 之间", meta.MemoryInitialMB, meta.MemoryMaxMB)
			}
			if pauseAuto {
				meta.ManualPauseUntil = time.Now().Add(10 * time.Minute).Unix()
				_ = WriteVMMemoryMetadata(name, meta)
			}
			return setVirtioMemRequestedLive(name, targetMB-meta.MemoryInitialMB)
		}
		if targetMB < meta.MemoryMinMB || targetMB > meta.MemoryMaxMB {
			return fmt.Errorf("当前内存必须在 %dMB 到 %dMB 之间", meta.MemoryMinMB, meta.MemoryMaxMB)
		}
		if pauseAuto {
			meta.ManualPauseUntil = time.Now().Add(10 * time.Minute).Unix()
			_ = WriteVMMemoryMetadata(name, meta)
		}
	}
	err := libvirt_rpc.SetDomainMemoryFlagsRPC(name, uint64(targetMB*1024), libvirt.DomainMemoryModFlags(1)) // 1=VIR_DOMAIN_AFFECT_LIVE
	if err != nil {
		return fmt.Errorf("调整当前内存失败: %w", err)
	}
	return nil
}

// ApplyPendingVMMemoryConfig 在 VM 开机前应用待迁移配置。
func ApplyPendingVMMemoryConfig(name string) error {
	meta, err := ReadVMMemoryMetadata(name)
	if err != nil || meta == nil || !meta.DynamicEnabled || !meta.PendingApply {
		return err
	}
	if err := applyVMMemoryMetadataToInactiveXML(name, meta); err != nil {
		return err
	}
	meta.PendingApply = false
	return WriteVMMemoryMetadata(name, meta)
}

func applyVMMemoryMetadataToInactiveXML(name string, meta *VMMemoryMetadata) error {
	xmlResult, err := libvirt_rpc.GetDomainXMLRPC(name, libvirt.DomainXMLInactive)
	if err != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %w", err)
	}
	var xmlStr string
	if NormalizeMemoryBackend(meta.MemoryBackend) == MemoryBackendVirtioMem {
		xmlStr, err = ApplyVirtioMemConfigToDomainXML(xmlResult, meta.MemoryInitialMB, meta.MemoryMaxMB)
	} else {
		xmlStr, err = ApplyDynamicMemoryConfigToDomainXML(xmlResult, meta.MemoryInitialMB, meta.MemoryMaxMB, true)
	}
	if err != nil {
		return err
	}
	if _, err := libvirt_rpc.DefineDomainXMLRPC(xmlStr); err != nil {
		return fmt.Errorf("应用动态内存配置失败: %w", err)
	}
	return nil
}
