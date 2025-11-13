package proxy

import (
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"time"

	"goForward/proxy/hysteria"
	"goForward/proxy/xray"
)

// Bridge 代理桥接管理器
type Bridge struct {
	id            int
	xrayManager   *xray.Manager
	hy2Client     *hysteria.Client
	xrayConfig    string
	hy2Config     string
	socks5Port    int  // Hysteria2 SOCKS5端口
	running       bool
	mu            sync.Mutex
	// 流量监控器
	trafficMonitor *TrafficMonitor
}

// NewBridge 创建代理桥接
func NewBridge(id int, socks5Port int) *Bridge {
	baseDir := filepath.Join(".", "proxy_configs")

	if socks5Port == 0 {
		socks5Port = 10808 // 默认端口
	}

	return &Bridge{
		id:              id,
		xrayConfig:      filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", id)),
		hy2Config:       filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", id)),
		socks5Port:      socks5Port,
		running:         false,
		trafficMonitor:  NewTrafficMonitor(),
	}
}

// Start 启动桥接
func (b *Bridge) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("桥接已在运行")
	}

	// 检查Hysteria2是否已由Hysteria2Manager管理
	hy2Manager := hysteria.GetGlobalManager()
	if hy2Manager.IsRunning(b.id) {
		// Hysteria2已由管理器启动，等待SOCKS5端口就绪
		fmt.Printf("[Bridge-%d] Hysteria2实例已由管理器管理，等待SOCKS5端口(%d)就绪...\n", b.id, b.socks5Port)
		if err := b.waitForSocks5Ready(b.socks5Port, 10*time.Second); err != nil {
			return fmt.Errorf("Hysteria2 SOCKS5端口未就绪: %v", err)
		}
		fmt.Printf("[Bridge-%d] Hysteria2 SOCKS5端口(%d)已就绪\n", b.id, b.socks5Port)
	} else {
		// 启动独立的Hysteria2客户端（提供SOCKS5服务）
		b.hy2Client = hysteria.NewClient(b.hy2Config)
		if err := b.hy2Client.Start(); err != nil {
			return fmt.Errorf("启动Hysteria2失败: %v", err)
		}

		// 等待Hysteria2的SOCKS5端口就绪
		fmt.Printf("[Bridge-%d] 等待Hysteria2 SOCKS5端口(%d)就绪...\n", b.id, b.socks5Port)
		if err := b.waitForSocks5Ready(b.socks5Port, 10*time.Second); err != nil {
			b.hy2Client.Stop()
			return fmt.Errorf("Hysteria2 SOCKS5端口未就绪: %v", err)
		}
		fmt.Printf("[Bridge-%d] Hysteria2 SOCKS5端口(%d)已就绪\n", b.id, b.socks5Port)
	}

	// 启动Xray（连接到Hysteria2的SOCKS5）
	b.xrayManager = xray.NewManager(b.xrayConfig)
	if err := b.xrayManager.Start(); err != nil {
		// 如果Xray启动失败，停止Hysteria2（仅当Hysteria2由Bridge管理时）
		if b.hy2Client != nil {
			b.hy2Client.Stop()
		}
		return fmt.Errorf("启动Xray失败: %v", err)
	}

	b.running = true
	fmt.Printf("[Bridge-%d] 代理桥接已启动\n", b.id)

	// 启动流量监控
	b.trafficMonitor.Start()

	return nil
}

// waitForSocks5Ready 等待SOCKS5端口就绪
func (b *Bridge) waitForSocks5Ready(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("超时: SOCKS5端口 %d 在 %v 内未就绪", port, timeout)
}

// Stop 停止桥接
func (b *Bridge) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return fmt.Errorf("桥接未运行")
	}

	var errs []error

	// 先停止Xray
	if b.xrayManager != nil {
		if err := b.xrayManager.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("停止Xray失败: %v", err))
		}
	}

	// 检查Hysteria2是否由Hysteria2Manager管理
	hy2Manager := hysteria.GetGlobalManager()
	if !hy2Manager.IsRunning(b.id) {
		// Hysteria2由Bridge管理，需要停止
		if b.hy2Client != nil {
			if err := b.hy2Client.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("停止Hysteria2失败: %v", err))
			}
		}
	}

	// 停止流量监控
	if b.trafficMonitor != nil {
		b.trafficMonitor.Stop()
	}

	b.running = false
	fmt.Printf("[Bridge-%d] 代理桥接已停止\n", b.id)

	if len(errs) > 0 {
		return fmt.Errorf("停止时发生错误: %v", errs)
	}

	return nil
}

// Restart 重启桥接
func (b *Bridge) Restart() error {
	if err := b.Stop(); err != nil {
		fmt.Printf("[Bridge-%d] 停止失败: %v\n", b.id, err)
	}

	return b.Start()
}

// IsRunning 检查桥接是否运行
func (b *Bridge) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// GetStatus 获取桥接状态
func (b *Bridge) GetStatus() map[string]interface{} {
	b.mu.Lock()
	defer b.mu.Unlock()

	status := map[string]interface{}{
		"id":      b.id,
		"running": b.running,
	}

	if b.xrayManager != nil {
		status["xray_running"] = b.xrayManager.IsRunning()
	}

	if b.hy2Client != nil {
		status["hy2_running"] = b.hy2Client.IsRunning()
	}

	return status
}

// BridgeManager 全局桥接管理器
type BridgeManager struct {
	bridges map[int]*Bridge
	mu      sync.RWMutex
}

var globalBridgeManager = &BridgeManager{
	bridges: make(map[int]*Bridge),
}

// GetBridgeManager 获取全局桥接管理器
func GetBridgeManager() *BridgeManager {
	return globalBridgeManager
}

// AddBridge 添加桥接
func (bm *BridgeManager) AddBridge(id int, socks5Port int) *Bridge {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bridge, exists := bm.bridges[id]; exists {
		return bridge
	}

	bridge := NewBridge(id, socks5Port)
	bm.bridges[id] = bridge
	return bridge
}

// GetBridge 获取桥接
func (bm *BridgeManager) GetBridge(id int) (*Bridge, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	bridge, exists := bm.bridges[id]
	return bridge, exists
}

// RemoveBridge 移除桥接
func (bm *BridgeManager) RemoveBridge(id int) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bridge, exists := bm.bridges[id]; exists {
		if bridge.IsRunning() {
			if err := bridge.Stop(); err != nil {
				return err
			}
		}
		// 停止流量监控
		if bridge.trafficMonitor != nil {
			bridge.trafficMonitor.Stop()
		}
		delete(bm.bridges, id)
	}

	return nil
}

// StopAll 停止所有桥接
func (bm *BridgeManager) StopAll() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for id, bridge := range bm.bridges {
		if bridge.IsRunning() {
			if err := bridge.Stop(); err != nil {
				fmt.Printf("[Bridge-%d] 停止失败: %v\n", id, err)
			}
		}
		// 停止流量监控
		if bridge.trafficMonitor != nil {
			bridge.trafficMonitor.Stop()
		}
	}
}
