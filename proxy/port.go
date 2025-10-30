package proxy

import (
	"fmt"
	"math/rand"
	"net"
	"time"
)

// GetRandomAvailablePort 获取随机可用端口（10000以上）
func GetRandomAvailablePort() int {
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < 100; i++ {
		port := rand.Intn(55535) + 10000 // 10000-65535
		if isPortAvailable(port) {
			return port
		}
	}

	// 如果随机100次都没找到，顺序查找
	for port := 10000; port < 65535; port++ {
		if isPortAvailable(port) {
			return port
		}
	}

	return 10443 // 默认端口
}

// isPortAvailable 检查端口是否可用
func isPortAvailable(port int) bool {
	addr := net.JoinHostPort("0.0.0.0", fmt.Sprintf("%d", port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}
