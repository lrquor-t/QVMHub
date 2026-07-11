package bridge

import (
	"fmt"
	"path/filepath"

	"qvmhub/utils"
)

func writeBridgeRestoreUnit() error {
	content := `[Unit]
Description=KVM Console managed OVS bridge restore
After=network-online.target openvswitch-switch.service
Wants=network-online.target openvswitch-switch.service

[Service]
Type=oneshot
ExecStart=/bin/bash -c 'RC=0; for f in /etc/kvm-console/bridges/*.sh; do [ -e "$f" ] && /bin/bash "$f" || RC=1; done; exit $RC'
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`
	changed, err := HookWriteFileIfChanged("/etc/systemd/system/kvm-console-bridges.service", []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("写入桥接网桥 systemd 服务失败: %w", err)
	}
	if changed {
		utils.ExecCommand("systemctl", "daemon-reload")
	}
	ensureSystemdUnitEnabled("kvm-console-bridges.service")
	return nil
}

func disableBridgeRestoreUnitIfEmpty() {
	matches, err := filepath.Glob(filepath.Join(bridgeConfigDir, "*.sh"))
	if err != nil || len(matches) > 0 {
		return
	}
	utils.ExecCommand("systemctl", "disable", "--now", "kvm-console-bridges.service")
	utils.ExecCommand("systemctl", "reset-failed", "kvm-console-bridges.service")
}

func ensureSystemdUnitEnabled(unit string) {
	if result := utils.ExecCommand("systemctl", "is-enabled", "--quiet", unit); result.Error != nil {
		utils.ExecCommand("systemctl", "enable", unit)
	}
}
