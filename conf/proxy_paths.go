package conf

import (
	"fmt"
	"path/filepath"
)

const xrayAPIPortBase = 15000

// ProxyLogDir returns the log directory path for a proxy instance.
func ProxyLogDir(proxyID int) string {
	if proxyID <= 0 {
		return filepath.Join(".", "proxy_configs", "logs")
	}
	return filepath.Join(".", "proxy_configs", fmt.Sprintf("logs_%d", proxyID))
}

// XrayAPIPort returns the API port for the proxy's Xray instance.
func XrayAPIPort(proxyID int) int {
	if proxyID <= 0 {
		return 0
	}
	port := xrayAPIPortBase + proxyID
	if port > 65535 {
		return 0
	}
	return port
}
