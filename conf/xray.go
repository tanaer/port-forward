package conf

import (
	"fmt"
	"path/filepath"
)

const (
	// BaseXrayAPIPort 是为每个代理分配 Xray API 端口的基准
	BaseXrayAPIPort = 15000
)

// XrayAPIPort 根据代理 ID 计算 API 端口
func XrayAPIPort(proxyID int) int {
	return BaseXrayAPIPort + proxyID
}

// ProxyLogDir 获取代理专属日志目录
func ProxyLogDir(proxyID int) string {
	return filepath.Join(".", "proxy_configs", fmt.Sprintf("logs_%d", proxyID))
}

// XrayAccessLogPath 返回 Xray access 日志路径
func XrayAccessLogPath(proxyID int) string {
	return filepath.Join(ProxyLogDir(proxyID), "access.log")
}

// XrayErrorLogPath 返回 Xray error 日志路径
func XrayErrorLogPath(proxyID int) string {
	return filepath.Join(ProxyLogDir(proxyID), "xray_error.log")
}
