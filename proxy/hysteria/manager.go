package hysteria

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"goForward/conf"
)

// Manager Hysteria2管理器
type Manager struct {
	clients map[int]*Client // ID -> Hysteria2客户端实例
	configs map[int]string  // ID -> 配置文件路径
	mu      sync.RWMutex    // 读写锁
}

// NewManager 创建Hysteria2管理器
func NewManager() *Manager {
	return &Manager{
		clients: make(map[int]*Client),
		configs: make(map[int]string),
	}
}

// CreateAndStart 创建并启动Hysteria2实例
func (m *Manager) CreateAndStart(id int, cfg conf.ProxyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成配置文件路径
	baseDir := filepath.Join(".", "proxy_configs")
	os.MkdirAll(baseDir, 0755)
	configPath := filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", id))
	logDir := filepath.Join(baseDir, fmt.Sprintf("logs_%d", id))

	// 生成Hysteria2配置
	hy2Config := GenerateHy2Config(GenerateHy2ConfigParams(cfg))
	if err := hy2Config.SaveConfig(configPath); err != nil {
		return fmt.Errorf("生成Hysteria2配置失败: %v", err)
	}

	// 如果实例已存在但未运行，复用旧客户端
	if client, exists := m.clients[id]; exists {
		if client.IsRunning() {
			return fmt.Errorf("Hysteria2实例ID=%d已存在", id)
		}
		// 更新配置路径并重新启动
		client.configPath = configPath
		client.logDir = logDir
		m.configs[id] = configPath
		if err := client.Start(); err != nil {
			return fmt.Errorf("启动Hysteria2失败: %v", err)
		}

		fmt.Printf("[Hysteria2Manager] ID=%d 已重新启动，SOCKS5端口=%d\n",
			id, hy2Config.Socks5.ListenPort())
		return nil
	}

	// 清理可能存在的孤儿进程
	cleanupProcessByConfig(configPath)

	// 创建并启动新的Hysteria2客户端
	client := NewClient(configPath, logDir)
	if err := client.Start(); err != nil {
		return fmt.Errorf("启动Hysteria2失败: %v", err)
	}

	// 保存到管理器
	m.clients[id] = client
	m.configs[id] = configPath

	socks5Port := hy2Config.Socks5.ListenPort()
	fmt.Printf("[Hysteria2Manager] ID=%d 已启动，SOCKS5端口=%d，等待端口就绪...\n", id, socks5Port)

	// 等待 SOCKS5 端口就绪（最多等待 30 秒），如果进程提前退出则立即失败
	if err := waitForPortReadyWithClient(socks5Port, 30*time.Second, client); err != nil {
		// 端口未就绪，停止客户端并返回错误
		client.Stop()
		delete(m.clients, id)
		delete(m.configs, id)
		return fmt.Errorf("Hysteria2 SOCKS5端口(%d)启动失败: %v", socks5Port, err)
	}

	fmt.Printf("[Hysteria2Manager] ID=%d SOCKS5端口(%d)已就绪\n", id, socks5Port)
	return nil
}

// UpdateAndRestart 更新并重启Hysteria2实例（如果实例不存在则自动创建）
func (m *Manager) UpdateAndRestart(id int, cfg conf.ProxyConfig) error {
	m.mu.Lock()

	// 检查是否存在，如果不存在则释放锁并调用CreateAndStart创建新实例
	if _, exists := m.clients[id]; !exists {
		m.mu.Unlock()
		fmt.Printf("[Hysteria2Manager] ID=%d 实例不存在，创建新实例\n", id)
		return m.CreateAndStart(id, cfg)
	}

	// 停止现有实例
	if err := m.clients[id].Stop(); err != nil {
		fmt.Printf("[Hysteria2Manager] 停止ID=%d失败: %v\n", id, err)
	}

	// 生成新配置
	baseDir := filepath.Join(".", "proxy_configs")
	configPath := filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", id))
	logDir := filepath.Join(baseDir, fmt.Sprintf("logs_%d", id))

	hy2Config := GenerateHy2Config(GenerateHy2ConfigParams(cfg))
	if err := hy2Config.SaveConfig(configPath); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("更新Hysteria2配置失败: %v", err)
	}
	m.clients[id].configPath = configPath
	m.clients[id].logDir = logDir

	// 重新启动
	if err := m.clients[id].Start(); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("重启Hysteria2失败: %v", err)
	}

	socks5Port := hy2Config.Socks5.ListenPort()
	m.mu.Unlock()

	fmt.Printf("[Hysteria2Manager] ID=%d 已重启，SOCKS5端口=%d，等待端口就绪...\n", id, socks5Port)

	// 等待 SOCKS5 端口就绪（最多等待 30 秒），如果进程提前退出则立即返回
	if err := waitForPortReadyWithClient(socks5Port, 30*time.Second, m.clients[id]); err != nil {
		fmt.Printf("[Hysteria2Manager] 警告: ID=%d SOCKS5端口(%d)等待失败: %v\n", id, socks5Port, err)
		return fmt.Errorf("Hysteria2 SOCKS5端口(%d)启动失败: %v", socks5Port, err)
	}

	fmt.Printf("[Hysteria2Manager] ID=%d SOCKS5端口(%d)已就绪\n", id, socks5Port)
	return nil
}

// Stop 停止Hysteria2实例
func (m *Manager) Stop(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return fmt.Errorf("Hysteria2实例ID=%d不存在", id)
	}

	if err := client.Stop(); err != nil {
		return fmt.Errorf("停止Hysteria2失败: %v", err)
	}

	delete(m.clients, id)
	delete(m.configs, id)
	fmt.Printf("[Hysteria2Manager] ID=%d 已停止\n", id)
	return nil
}

// Start 启动Hysteria2实例
func (m *Manager) Start(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return fmt.Errorf("Hysteria2实例ID=%d不存在", id)
	}

	if err := client.Start(); err != nil {
		return fmt.Errorf("启动Hysteria2失败: %v", err)
	}

	fmt.Printf("[Hysteria2Manager] ID=%d 已启动\n", id)
	return nil
}

// Delete 删除Hysteria2实例
func (m *Manager) Delete(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return fmt.Errorf("Hysteria2实例ID=%d不存在", id)
	}

	// 停止实例
	if err := client.Stop(); err != nil {
		fmt.Printf("[Hysteria2Manager] 停止ID=%d失败: %v\n", id, err)
		delete(m.clients, id)
		delete(m.configs, id)
	} else {
		// 清理潜在的孤儿进程
		configPath := filepath.Join(".", "proxy_configs", fmt.Sprintf("hy2_%d.yaml", id))
		cleanupProcessByConfig(configPath)
	}

	// 删除配置文件
	configPath, ok := m.configs[id]
	if ok {
		os.Remove(configPath)
	}

	fmt.Printf("[Hysteria2Manager] ID=%d 已删除\n", id)
	return nil
}

// GetSocks5Port 获取Hysteria2实例的SOCKS5端口
func (m *Manager) GetSocks5Port(id int) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configPath, exists := m.configs[id]
	if !exists {
		return 0, fmt.Errorf("Hysteria2实例ID=%d不存在", id)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		return 0, fmt.Errorf("加载配置失败: %v", err)
	}

	return config.Socks5.ListenPort(), nil
}

// IsRunning 检查Hysteria2实例是否运行中
func (m *Manager) IsRunning(id int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[id]
	if !exists {
		return false
	}

	return client.IsRunning()
}

// ForceStop 无论当前是否在管理器中记录，均尝试停止ID对应的进程
func (m *Manager) ForceStop(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, exists := m.clients[id]; exists {
		if err := client.Stop(); err != nil {
			return err
		}
		delete(m.clients, id)
		delete(m.configs, id)
		return nil
	}

	configPath := filepath.Join(".", "proxy_configs", fmt.Sprintf("hy2_%d.yaml", id))
	cleanupProcessByConfig(configPath)
	delete(m.configs, id)
	return nil
}

// StopAll 停止所有Hysteria2实例
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, client := range m.clients {
		client.Stop()
		fmt.Printf("[Hysteria2Manager] ID=%d 已停止\n", id)
	}

	m.clients = make(map[int]*Client)
}

// waitForPortReady 等待端口就绪
func waitForPortReady(port int, timeout time.Duration) error {
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

	return fmt.Errorf("端口 %d 在 %v 内未就绪", port, timeout)
}

// waitForPortReadyWithClient 在等待端口就绪时，同时检测关联的客户端是否已经退出，避免长时间卡住
func waitForPortReadyWithClient(port int, timeout time.Duration, client *Client) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		// 进程已退出则立即返回
		if client != nil && !client.IsRunning() {
			return fmt.Errorf("Hysteria2进程已退出")
		}

		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 超时前再检查一次进程状态，提供更准确的错误
	if client != nil && !client.IsRunning() {
		return fmt.Errorf("Hysteria2进程已退出")
	}

	return fmt.Errorf("端口 %d 在 %v 内未就绪", port, timeout)
}

// GlobalHysteria2Manager 全局Hysteria2管理器实例
var globalHysteria2Manager = NewManager()

// GetGlobalManager 获取全局Hysteria2管理器
func GetGlobalManager() *Manager {
	return globalHysteria2Manager
}
