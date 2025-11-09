package version

import (
	"fmt"
	"runtime"
	"strings"
)

// VersionInfo 版本信息结构
type VersionInfo struct {
	Version     string // 版本号，如 v1.3.0
	BuildTime   string // 构建时间
	GoVersion   string // Go版本
	GitCommit   string // Git提交ID
	BuildUser   string // 构建用户
	BuildArch   string // 构建架构
	OS          string // 操作系统
	Description string // 版本描述
}

var (
	// Version 应用版本（通过构建时注入���
	Version = "v1.7.0"

	// BuildTime 构建时间（通过构建时注入）
	BuildTime = "2025-11-09 20:00:00"

	// GitCommit Git提交ID（通过构建时注入）
	GitCommit = "unknown"

	// BuildDescription 构建描述
	BuildDescription = "Phase 3 功能增强完成版本 - API接口增强 + 监控面板"
)

// GetVersion 获取完整版本信息
func GetVersion() VersionInfo {
	return VersionInfo{
		Version:     Version,
		BuildTime:   BuildTime,
		GoVersion:   runtime.Version(),
		GitCommit:   GitCommit,
		BuildUser:   "build",
		BuildArch:   runtime.GOARCH,
		OS:          runtime.GOOS,
		Description: BuildDescription,
	}
}

// GetVersionString 获取版本字符串
func GetVersionString() string {
	v := GetVersion()
	return fmt.Sprintf("%s (Build: %s, Go: %s, Commit: %s)",
		v.Version, v.BuildTime, v.GoVersion, v.GitCommit)
}

// PrintVersion 打印版本信息
func PrintVersion() {
	v := GetVersion()
	fmt.Printf("goForward - 高性能端口转发工具\n")
	fmt.Printf("版本: %s\n", v.Version)
	fmt.Printf("描述: %s\n", v.Description)
	fmt.Printf("构建时间: %s\n", v.BuildTime)
	fmt.Printf("Go版本: %s\n", v.GoVersion)
	fmt.Printf("Git提交: %s\n", v.GitCommit)
	fmt.Printf("架构: %s/%s\n", v.OS, v.BuildArch)
}

// ShowVersionAndExit 显示版本信息并退出
func ShowVersionAndExit() {
	PrintVersion()
}

// IsProduction 判断是否为生产环境版本
func IsProduction() bool {
	return strings.Contains(Version, "v") && !strings.Contains(Version, "dev")
}

// GetMajorMinor 获取主版本号（如 v1.6）
func GetMajorMinor() string {
	parts := strings.Split(Version, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return Version
}