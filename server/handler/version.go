package handler

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"qvmhub/config"
	"qvmhub/service/arch"
)

// GetVersion 返回系统版本信息
func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"version":    Version,
			"build_time": BuildTime,
			"site_title": config.GlobalConfig.SiteTitle,
		},
	})
}

// GetPublicSystemInfo 返回系统运行环境信息（需登录认证）
func GetPublicSystemInfo(c *gin.Context) {
	hostname, _ := os.Hostname()

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"go_version":    runtime.Version(),
			"os":            runtime.GOOS,
			"distro":        getDistroName(),
			"arch":          arch.GetHostArchDisplayName(),
			"num_cpu":       runtime.NumCPU(),
			"hostname":      hostname,
			"num_goroutine": runtime.NumGoroutine(),
			"kernel":        getKernelVersion(),
			"uptime":        getSystemUptime(),
			"libvirt":       getLibvirtVersion(),
			"qemu":          getQEMUVersion(),
		},
	})
}

func getKernelVersion() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return "-"
	}
	return strings.TrimSpace(string(out))
}

func getSystemUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "-"
	}
	var uptimeSeconds float64
	fmt.Sscanf(string(data), "%f", &uptimeSeconds)
	d := time.Duration(uptimeSeconds * float64(time.Second))
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%d 天 %d 小时", days, hours)
	}
	return fmt.Sprintf("%d 小时", hours)
}

func getLibvirtVersion() string {
	out, err := exec.Command("libvirtd", "--version").Output()
	if err != nil {
		return "-"
	}
	return strings.TrimSpace(string(out))
}

func getQEMUVersion() string {
	out, err := exec.Command("qemu-system-x86_64", "--version").Output()
	if err != nil {
		out, err = exec.Command("qemu-kvm", "--version").Output()
		if err != nil {
			return "-"
		}
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) > 0 {
		return lines[0]
	}
	return "-"
}

func getDistroName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "-"
	}
	name := ""
	version := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			break
		}
		if strings.HasPrefix(line, "NAME=") && name == "" {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		}
		if strings.HasPrefix(line, "VERSION=") && version == "" {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
		}
	}
	if name != "" {
		return name
	}
	if version != "" {
		return version
	}
	return "-"
}

// Version 通过 ldflags 在构建时注入，格式: -X qvmhub/handler.Version=v1.0.0
var Version = "dev"

// BuildTime 通过 ldflags 在构建时注入，格式: -X qvmhub/handler.BuildTime=2025-01-01T00:00:00Z
var BuildTime = ""
