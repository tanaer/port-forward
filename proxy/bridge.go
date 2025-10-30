package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
	running       bool
	mu            sync.Mutex
}

// NewBridge 创建代理桥接
func NewBridge(id int) *Bridge {
	execPath, _ := os.Executable()
	baseDir := filepath.Join(filepath.Dir(execPath), "proxy_configs")

	return &Bridge{
		id:         id,
		xrayConfig: filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", id)),
		hy2Config:  filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", id)),
		running:    false,
	}
}

// Start 启动桥接
func (b *Bridge) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("桥接已在运行")
	}

	// 先启动Hysteria2客户端（提供SOCKS5服务）
	b.hy2Client = hysteria.NewClient(b.hy2Config)
	if err := b.hy2Client.Start(); err != nil {
		return fmt.Errorf("启动Hysteria2失败: %v", err)
	}

	// 等待Hysteria2启动
	// time.Sleep(2 * time.Second)

	// 再启动Xray（连接到Hysteria2的SOCKS5）
	b.xrayManager = xray.NewManager(b.xrayConfig)
	if err := b.xrayManager.Start(); err != nil {
		// 如果Xray启动失败，停止Hysteria2
		b.hy2Client.Stop()
		return fmt.Errorf("启动Xray失败: %v", err)
	}

	b.running = true
	fmt.Printf("[Bridge-%d] 代理桥接已启动\n", b.id)
	return nil
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

	// 再停止Hysteria2
	if b.hy2Client != nil {
		if err := b.hy2Client.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("停止Hysteria2失败: %v", err))
		}
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
func (bm *BridgeManager) AddBridge(id int) *Bridge {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bridge, exists := bm.bridges[id]; exists {
		return bridge
	}

	bridge := NewBridge(id)
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
	}
}
