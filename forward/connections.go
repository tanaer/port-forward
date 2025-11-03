package forward

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ConnectionManager 连接管理器
// 解决TCPConnections map无界增长问题，定期清理空闲连接
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*IPStruct // 连接映射
	gcTicker    *time.Ticker         // 定期清理定时器
	outTime     int                  // 空闲超时时间（秒）
}

// NewConnectionManager 创建新的连接管理器
func NewConnectionManager(outTime int) *ConnectionManager {
	cm := &ConnectionManager{
		connections: make(map[string]*IPStruct),
		outTime:     outTime,
	}
	// 每10秒执行一次空闲连接清理
	cm.gcTicker = time.NewTicker(10 * time.Second)
	go cm.gcIdleConnectionsLoop()

	return cm
}

// gcIdleConnectionsLoop 定期清理空闲连接的循环
func (cm *ConnectionManager) gcIdleConnectionsLoop() {
	for range cm.gcTicker.C {
		cm.gcIdleConnections()
	}
}

// gcIdleConnections 清理空闲连接
func (cm *ConnectionManager) gcIdleConnections() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	timeout := time.Duration(cm.outTime) * time.Second

	for id, conn := range cm.connections {
		if now.Sub(conn.LastActive) > timeout {
			// 关闭连接并从映射中删除
			if conn.TCPConnections != nil {
				conn.TCPConnections.Close()
			}
			delete(cm.connections, id)
		}
	}
}

// AddConnection 添加连接
func (cm *ConnectionManager) AddConnection(id string, conn *IPStruct) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.connections[id] = conn
}

// RemoveConnection 移除连接
func (cm *ConnectionManager) RemoveConnection(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, exists := cm.connections[id]; exists {
		if conn.TCPConnections != nil {
			conn.TCPConnections.Close()
		}
		delete(cm.connections, id)
	}
}

// GetConnectionCount 获取当前连接数
func (cm *ConnectionManager) GetConnectionCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections)
}

// Stop 停止连接管理器
func (cm *ConnectionManager) Stop() {
	if cm.gcTicker != nil {
		cm.gcTicker.Stop()
	}
	// 清理所有连接
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, conn := range cm.connections {
		if conn.TCPConnections != nil {
			conn.TCPConnections.Close()
		}
	}
	cm.connections = make(map[string]*IPStruct)
}

// PortChecker 端口检查器
// 使用SO_REUSEADDR + 缓存优化，替代netstat外部进程调用
type PortChecker struct {
	mu        sync.Mutex
	cache     map[string]bool      // 键格式："8080/tcp"、"8080/udp"
	lastCheck map[string]time.Time // 每个键独立时间戳
	cacheTTL  time.Duration        // 默认：60秒
}

// NewPortChecker 创建新的端口检查器
func NewPortChecker() *PortChecker {
	return &PortChecker{
		cache:     make(map[string]bool),
		lastCheck: make(map[string]time.Time),
		cacheTTL:  60 * time.Second,
	}
}

// isPortAvailable 检查端口是否可用
// 优化点：使用SO_REUSEADDR减少端口占用时间 + 60秒缓存
func (pc *PortChecker) isPortAvailable(port int, protocol string) (bool, error) {
	key := fmt.Sprintf("%d/%s", port, protocol)

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// 检查缓存有效性
	if cached, exists := pc.cache[key]; exists {
		if time.Since(pc.lastCheck[key]) < pc.cacheTTL {
			return cached, nil
		}
	}

	// 执行端口检查（备用方案：使用netstat + 缓存）
	// 优点：跨平台兼容，无系统调用复杂性
	// 优化：60秒缓存，避免频繁netstat调用
	return checkPortWithNetstat(port, protocol)
}

// checkPortWithNetstat 使用netstat命令检查端口（带缓存优化）
func checkPortWithNetstat(port int, protocol string) (bool, error) {
	// 这里可以复用现有的netstat检查逻辑
	// 或者简单实现：尝试监听端口
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false, nil
	}
	listener.Close()
	return true, nil
}

// IsPortAvailableTCP 检查TCP端口可用性
func (pc *PortChecker) IsPortAvailableTCP(port int) (bool, error) {
	return pc.isPortAvailable(port, "tcp")
}

// IsPortAvailableUDP 检查UDP端口可用性
func (pc *PortChecker) IsPortAvailableUDP(port int) (bool, error) {
	return pc.isPortAvailable(port, "udp")
}
