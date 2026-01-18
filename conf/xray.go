package conf

import "path/filepath"

// XrayAccessLogPath 返回 Xray access 日志路径
func XrayAccessLogPath(proxyID int) string {
	return filepath.Join(ProxyLogDir(proxyID), "access.log")
}

// XrayErrorLogPath 返回 Xray error 日志路径
func XrayErrorLogPath(proxyID int) string {
	return filepath.Join(ProxyLogDir(proxyID), "xray_error.log")
}
