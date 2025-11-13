package hysteria

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"goForward/conf"
)

// Manager Hysteria2管理器
type Manager struct {
	clients map[int]*Client  // ID -> Hysteria2客户端实例
	configs map[int]string   // ID -> 配置文件路径
	mu      sync.RWMutex     // 读写锁
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

	// 检查是否已存在
	if _, exists := m.clients[id]; exists {
		return fmt.Errorf("Hysteria2实例ID=%d已存在", id)
	}

	// 生成配置文件路径
	baseDir := filepath.Join(".", "proxy_configs")
	configPath := filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", id))

	// 生成Hysteria2配置
	hy2Config := GenerateHy2Config(GenerateHy2ConfigParams(cfg))
	if err := hy2Config.SaveConfig(configPath); err != nil {
		return fmt.Errorf("生成Hysteria2配置失败: %v", err)
	}

	// 创建并启动Hysteria2客户端
	client := NewClient(configPath)
	if err := client.Start(); err != nil {
		return fmt.Errorf("启动Hysteria2失败: %v", err)
	}

	// 保存到管理器
	m.clients[id] = client
	m.configs[id] = configPath

	fmt.Printf("[Hysteria2Manager] ID=%d 已启动，SOCKS5端口=%d\n",
		id, hy2Config.Socks5.ListenPort())
	return nil
}

// UpdateAndRestart 更新并重启Hysteria2实例
func (m *Manager) UpdateAndRestart(id int, cfg conf.ProxyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否存在
	if _, exists := m.clients[id]; !exists {
		return fmt.Errorf("Hysteria2实例ID=%d不存在", id)
	}

	// 停止现有实例
	if err := m.clients[id].Stop(); err != nil {
		fmt.Printf("[Hysteria2Manager] 停止ID=%d失败: %v\n", id, err)
	}

	// 生成新配置
	baseDir := filepath.Join(".", "proxy_configs")
	configPath := filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", id))

	hy2Config := GenerateHy2Config(GenerateHy2ConfigParams(cfg))
	if err := hy2Config.SaveConfig(configPath); err != nil {
		return fmt.Errorf("更新Hysteria2配置失败: %v", err)
	}

	// 重新启动
	if err := m.clients[id].Start(); err != nil {
		return fmt.Errorf("重启Hysteria2失败: %v", err)
	}

	fmt.Printf("[Hysteria2Manager] ID=%d 已重启，SOCKS5端口=%d\n",
		id, hy2Config.Socks5.ListenPort())
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
	}

	// 删除配置文件
	configPath, ok := m.configs[id]
	if ok {
		os.Remove(configPath)
	}

	// 从管理器中移除
	delete(m.clients, id)
	delete(m.configs, id)

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

// GlobalHysteria2Manager 全局Hysteria2管理器实例
var globalHysteria2Manager = NewManager()

// GetGlobalManager 获取全局Hysteria2管理器
func GetGlobalManager() *Manager {
	return globalHysteria2Manager
}