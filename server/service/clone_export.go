package service

// Clone export adapters - export unexported functions/types for clone Deps injection
import (
	"context"

	clonepkg "qvmhub/service/clone"
	ovspkg "qvmhub/service/ovs"
	vmpkg "qvmhub/service/vm"
)

// GetTemplateMetaForClone returns a clone-compatible TemplateMeta
func GetTemplateMetaForClone(templateName string) *clonepkg.TemplateMeta {
	meta := GetTemplateMeta(templateName)
	if meta == nil {
		return nil
	}
	result := &clonepkg.TemplateMeta{
		Type:             meta.Type,
		Category:         meta.Category,
		BootType:         meta.BootType,
		RootPassword:     meta.RootPassword,
		TemplateUser:     meta.TemplateUser,
		CloudInitMode:    meta.CloudInitMode,
		PostBootCommand:  meta.PostBootCommand,
		PostBootBlocking: meta.PostBootBlocking,
		NVRAMPath:        meta.NVRAMPath,
	}
	if meta.DefaultConfig != nil {
		result.DefaultConfig = &clonepkg.TemplateDefaultConfig{
			DiskBus:             meta.DefaultConfig.DiskBus,
			VideoModel:          meta.DefaultConfig.VideoModel,
			CPUTopologyMode:     meta.DefaultConfig.CPUTopologyMode,
			FirstBootRebootMode: meta.DefaultConfig.FirstBootRebootMode,
		}
	}
	return result
}

// ListAllVPCStaticHostsForClone converts service OVSStaticHost to clone.OVSStaticHost
func ListAllVPCStaticHostsForClone() ([]clonepkg.OVSStaticHost, error) {
	hosts, err := ovspkg.ListAllVPCStaticHosts()
	if err != nil {
		return nil, err
	}
	result := make([]clonepkg.OVSStaticHost, len(hosts))
	for i, h := range hosts {
		result[i] = clonepkg.OVSStaticHost{MAC: h.MAC, IP: h.IP}
	}
	return result, nil
}

// WaitForVMShutOff is now defined in vm_delegate.go

// GetVMDiskInfoForClone exports GetVMDiskInfo result for clone Deps
func GetVMDiskInfoForClone(name string) clonepkg.VMDiskInfoResult {
	info := vmpkg.GetVMDiskInfo(name)
	return clonepkg.VMDiskInfoResult{
		Path:   info.Path,
		Device: info.Device,
		Size:   info.Size,
	}
}

// InjectPCIERootPortsExported exports InjectPCIERootPorts for clone Deps
func InjectPCIERootPortsExported(xmlContent string, portCount int) string {
	return vmpkg.InjectPCIERootPorts(xmlContent, portCount)
}

// EnsureTemplatePathExported exports EnsureTemplatePath for clone Deps
func EnsureTemplatePathExported(templateName string) (string, error) {
	return EnsureTemplatePath(templateName)
}

// PrepareFnOSSystemDiskExpansionExported exports prepareFnOSSystemDiskExpansion for clone Deps
func PrepareFnOSSystemDiskExpansionExported(ctx context.Context, cloneDisk string, progressFn func(int, string)) error {
	return prepareFnOSSystemDiskExpansion(ctx, cloneDisk, progressFn)
}

// PrepareWindowsSystemDiskExpansionExported exports prepareWindowsSystemDiskExpansion for clone Deps
func PrepareWindowsSystemDiskExpansionExported(ctx context.Context, cloneDisk string, progressFn func(int, string)) error {
	return prepareWindowsSystemDiskExpansion(ctx, cloneDisk, progressFn)
}
