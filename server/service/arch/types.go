package arch

// ==================== 架构常量 ====================

const (
	ArchX8664   = "x86_64"
	ArchAarch64 = "aarch64"
	ArchRiscv64 = "riscv64"
)

// ==================== ArchProfile 接口 ====================

// ArchProfile 定义了不同 CPU 架构的配置差异，供 XML 生成、Libvirt 调用等模块使用。
type ArchProfile interface {
	Arch() string                                // 架构标识
	DisplayName() string                         // 人类可读名称（如 "x86_64 (AMD64)"、"aarch64 (ARM64)"）
	EmulatorPath() string                        // QEMU 模拟器路径（如 /usr/bin/qemu-system-x86_64）
	DefaultMachineType() string                  // 默认机型（x86: q35, ARM: virt）
	SupportedMachineTypes() []string             // 支持的机型列表
	DefaultBootType() string                     // 默认引导方式（x86: bios, ARM: uefi）
	SupportedBootTypes() []string                // 支持的引导方式列表
	DefaultCPUMode() string                      // 默认 CPU 模式（host-passthrough）
	DefaultCPUModel(virtType string) string      // QEMU 软件虚拟化时的 CPU 模型（x86: qemu64, ARM: cortex-a72）
	SupportedDiskBus() []string                  // 支持的磁盘总线（x86: virtio/scsi/sata/ide, ARM: virtio/scsi）
	GetCDROMBus() string                         // CDROM 设备总线（x86: sata, ARM: usb）
	SupportedNicModels() []string                // 支持的网卡模型（x86: virtio/e1000e/rtl8139, ARM: virtio）
	UEFIFirmwarePath(secureBoot bool) string     // UEFI 固件路径（x86: OVMF, ARM: AAVMF）
	UEFIVarsTemplatePath(secureBoot bool) string // UEFI 变量模板路径
	SupportsBIOS() bool                          // 是否支持 BIOS 引导（x86: true, ARM: false）
	SupportsSecureBoot() bool                    // 是否支持安全引导（x86: true, ARM: false）
	SupportsPAE() bool                           // 是否支持 PAE（x86: true, ARM: false）
	SupportsAPIC() bool                          // 是否支持 APIC（x86: true, ARM: false）
	DefaultWatchdogModel() string                // 默认看门狗模型（x86: itco, ARM: diag288）
}
