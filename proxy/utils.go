package proxy

import (
	"fmt"
	"os"
	"path/filepath"

	"goForward/conf"
	"goForward/proxy/hysteria"
	"goForward/proxy/xray"
	"goForward/sql"
)

// ProxyManager 代理管理器
type ProxyManager struct{}

// CreateProxyFromConfig 从配置创建代理
func (pm *ProxyManager) CreateProxyFromConfig(cfg conf.ProxyConfig) error {
	// 生成配置文件目录
	baseDir := filepath.Join(".", "proxy_configs")
	os.MkdirAll(baseDir, 0755)
	logDir := conf.ProxyLogDir(cfg.Id)
	_ = os.MkdirAll(logDir, 0755)

	// 根据出站类型选择正确的SOCKS5端口
	var socks5Port int
	if cfg.OutboundType == "hysteria2" {
		socks5Port = cfg.Hy2Socks5Port // Hysteria2出站：使用本地Hysteria2的SOCKS5端口
	} else if cfg.OutboundType == "socks5" {
		socks5Port = cfg.Socks5Port // SOCKS5出站：使用远端SOCKS5服务器端口
	} else {
		socks5Port = cfg.Hy2Socks5Port // 默认使用Hysteria2端口
	}

	// 生成Xray配置
	xrayConfig := xray.GenerateVLESSRealityConfig(xray.VLESSRealityConfig{
		ProxyID:         cfg.Id,
		Port:            cfg.InboundPort,
		UUID:            cfg.UUID,
		Flow:            cfg.Flow,
		RealityDest:     cfg.RealityDest,
		ServerNames:     []string{cfg.RealityServerName},
		PrivateKey:      cfg.PrivateKey,
		ShortIds:        []string{cfg.ShortId},
		OutboundType:    cfg.OutboundType,
		Socks5Addr:      cfg.Socks5Addr,
		Socks5Port:      socks5Port, // 根据OutboundType选择正确端口
		Socks5User:      cfg.Socks5User,
		Socks5Password:  cfg.Socks5Password,
		VmessServer:     cfg.VmessServer,
		VmessPort:       cfg.VmessPort,
		VmessUUID:       cfg.VmessUUID,
		VmessAlterID:    cfg.VmessAlterID,
		VmessSecurity:   cfg.VmessSecurity,
		VmessNetwork:    cfg.VmessNetwork,
		VmessTLS:        cfg.VmessTLS,
		VmessServerName: cfg.VmessServerName,
		VmessWsPath:     cfg.VmessWsPath,
		VmessWsHost:     cfg.VmessWsHost,
		LogDir:          logDir,
		APIPort:         conf.XrayAPIPort(cfg.Id),
	})

	xrayConfigPath := filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", cfg.Id))
	if err := xrayConfig.SaveConfig(xrayConfigPath); err != nil {
		return fmt.Errorf("保存Xray配置失败: %v", err)
	}

	// 如果出站类型是 Hysteria2，生成 Hysteria2 配置（但不启动）
	if cfg.OutboundType == "hysteria2" {
		hy2Config := hysteria.GenerateHy2Config(hysteria.GenerateHy2ConfigParams(cfg))
		hy2ConfigPath := filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", cfg.Id))
		if err := hy2Config.SaveConfig(hy2ConfigPath); err != nil {
			return fmt.Errorf("保存Hysteria2配置失败: %v", err)
		}
	}

	return nil
}

// StartProxy 启动代理
func (pm *ProxyManager) StartProxy(id int) error {
	cfg := sql.GetProxy(id)
	if cfg.Id == 0 {
		return fmt.Errorf("代理配置不存在")
	}

	// 检查是否已经在运行，避免重复启动
	bridgeManager := GetBridgeManager()
	if existingBridge, exists := bridgeManager.GetBridge(id); exists && existingBridge.IsRunning() {
		fmt.Printf("[Proxy] 代理 %d 已在运行，跳过启动\n", id)
		return nil
	}

	// 创建配置文件
	if err := pm.CreateProxyFromConfig(cfg); err != nil {
		return err
	}

	// 如果是Hysteria2出站，创建并启动Hysteria2实例
	if cfg.OutboundType == "hysteria2" {
		hy2Manager := hysteria.GetGlobalManager()
		if err := hy2Manager.CreateAndStart(id, cfg); err != nil {
			return fmt.Errorf("创建并启动Hysteria2实例失败: %v", err)
		}
	}

	// 获取或创建桥接
	// 重新计算正确的SOCKS5端口
	var socks5Port int
	if cfg.OutboundType == "hysteria2" {
		socks5Port = cfg.Hy2Socks5Port
	} else if cfg.OutboundType == "socks5" {
		socks5Port = cfg.Socks5Port
	} else {
		socks5Port = cfg.Hy2Socks5Port
	}
	bridge := bridgeManager.AddBridge(id, socks5Port, cfg.OutboundType)

	// 再次检查是否已在运行（防止并发情况）
	if bridge.IsRunning() {
		fmt.Printf("[Proxy] 代理 %d 桥接已在运行，跳过启动\n", id)
		return nil
	}

	// 如果是 SOCKS5 或 VMess 出站，只启动 Xray
	if cfg.OutboundType == "socks5" || cfg.OutboundType == "vmess" {
		// 初始化 Xray 管理器
		baseDir := filepath.Join(".", "proxy_configs")
		xrayConfigPath := filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", cfg.Id))

		logDir := filepath.Join(".", "proxy_configs", fmt.Sprintf("logs_%d", cfg.Id))
		bridge.xrayManager = xray.NewManager(xrayConfigPath, logDir)
		if err := bridge.xrayManager.Start(); err != nil {
			return fmt.Errorf("启动Xray失败: %v", err)
		}
		bridge.running = true
	} else {
		// 启动Xray（注意：Hysteria2已经在上面启动过了）
		if err := bridge.Start(); err != nil {
			return fmt.Errorf("启动代理失败: %v", err)
		}
	}

	// 更新状态
	sql.UpdateProxyStatus(id, 0)

	return nil
}

// StopProxy 停止代理（容错处理，即使桥接未运行也正常返回）
func (pm *ProxyManager) StopProxy(id int) error {
	cfg := sql.GetProxy(id)
	if cfg.Id == 0 {
		return fmt.Errorf("代理配置不存在")
	}

	// 检查代理是否正在运行
	bridgeManager := GetBridgeManager()
	bridge, exists := bridgeManager.GetBridge(id)

	// 如果是Hysteria2出站，使用Hysteria2Manager停止
	if cfg.OutboundType == "hysteria2" {
		hy2Manager := hysteria.GetGlobalManager()
		if err := hy2Manager.ForceStop(id); err != nil {
			fmt.Printf("[Proxy] 停止Hysteria2实例失败（可忽略）: %v\n", err)
		}
	}

	// 停止桥接（容错处理）
	if exists {
		if bridge.IsRunning() {
			if err := bridge.Stop(); err != nil {
				fmt.Printf("[Proxy] 停止桥接 %d 失败（可忽略）: %v\n", id, err)
			}
		} else {
			fmt.Printf("[Proxy] 桥接 %d 未运行，跳过停止\n", id)
		}
		// 从管理器中移除
		bridgeManager.RemoveBridge(id)
	} else {
		fmt.Printf("[Proxy] 桥接 %d 不存在，跳过停止\n", id)
	}

	// 更新状态
	sql.UpdateProxyStatus(id, 1)

	return nil
}

// RestartProxy 重启代理（完全重建：停止旧实例→重新从数据库读取配置→创建新实例）
func (pm *ProxyManager) RestartProxy(id int) error {
	cfg := sql.GetProxy(id)
	if cfg.Id == 0 {
		return fmt.Errorf("代理配置不存在")
	}

	// 先停止现有实例（容错处理）
	bridgeManager := GetBridgeManager()
	if bridge, exists := bridgeManager.GetBridge(id); exists {
		if bridge.IsRunning() {
			bridge.Stop()
		}
		bridgeManager.RemoveBridge(id)
	}

	// 如果是Hysteria2出站，先停止再重建
	if cfg.OutboundType == "hysteria2" {
		hy2Manager := hysteria.GetGlobalManager()
		// 先强制停止
		hy2Manager.ForceStop(id)
		// 重新创建并启动
		if err := hy2Manager.CreateAndStart(id, cfg); err != nil {
			return fmt.Errorf("重建Hysteria2实例失败: %v", err)
		}
	}

	// 重新创建配置文件
	if err := pm.CreateProxyFromConfig(cfg); err != nil {
		return fmt.Errorf("创建代理配置失败: %v", err)
	}

	// 创建新的桥接并启动
	socks5Port := cfg.Hy2Socks5Port
	if cfg.OutboundType == "socks5" {
		socks5Port = cfg.Socks5Port
	}
	bridge := bridgeManager.AddBridge(id, socks5Port, cfg.OutboundType)
	if err := bridge.Start(); err != nil {
		return fmt.Errorf("启动桥接失败: %v", err)
	}

	// 更新状态
	sql.UpdateProxyStatus(id, 0)

	return nil
}

// DeleteProxy 删除代理（停止所有运行实例→删除配置文件→删除数据库记录）
func (pm *ProxyManager) DeleteProxy(id int) error {
	cfg := sql.GetProxy(id)
	if cfg.Id == 0 {
		return fmt.Errorf("代理配置不存在")
	}

	// 先停止代理（容错处理）
	pm.StopProxy(id)

	// 如果是Hysteria2出站，使用Hysteria2Manager删除
	if cfg.OutboundType == "hysteria2" {
		hy2Manager := hysteria.GetGlobalManager()
		if err := hy2Manager.Delete(id); err != nil {
			fmt.Printf("[Proxy] 删除Hysteria2实例失败（可忽略）: %v\n", err)
		}
	}

	// 删除配置文件
	baseDir := filepath.Join(".", "proxy_configs")
	xrayConfig := filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", id))
	hy2Config := filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", id))
	logDir := filepath.Join(baseDir, fmt.Sprintf("logs_%d", id))

	if err := os.Remove(xrayConfig); err != nil && !os.IsNotExist(err) {
		fmt.Printf("[Proxy] 删除Xray配置文件失败: %v\n", err)
	}
	if err := os.Remove(hy2Config); err != nil && !os.IsNotExist(err) {
		fmt.Printf("[Proxy] 删除Hysteria2配置文件失败: %v\n", err)
	}
	if err := os.RemoveAll(logDir); err != nil && !os.IsNotExist(err) {
		fmt.Printf("[Proxy] 删除日志目录失败: %v\n", err)
	}

	// 删除数据库记录
	if !sql.DeleteProxy(id) {
		return fmt.Errorf("删除代理配置失败")
	}

	fmt.Printf("[Proxy] 代理 %d 已完全删除\n", id)
	return nil
}

// GetProxyStatus 获取代理状态
func (pm *ProxyManager) GetProxyStatus(id int) map[string]interface{} {
	bridgeManager := GetBridgeManager()
	bridge, exists := bridgeManager.GetBridge(id)
	if !exists {
		return map[string]interface{}{
			"running": false,
		}
	}

	return bridge.GetStatus()
}

// GenerateSubscriptionForProxy 为代理生成订阅
func (pm *ProxyManager) GenerateSubscriptionForProxy(proxyId int, serverIP string) (string, error) {
	cfg := sql.GetProxy(proxyId)
	if cfg.Id == 0 {
		return "", fmt.Errorf("代理配置不存在")
	}

	// 生成访问令牌
	token, err := GenerateAccessToken()
	if err != nil {
		return "", fmt.Errorf("生成令牌失败: %v", err)
	}

	// 保存订阅
	sub := conf.Subscription{
		ProxyId:     proxyId,
		AccessToken: token,
	}
	sql.AddSubscription(sub)

	return token, nil
}

// GetGlobalProxyManager 获取全局代理管理器
var globalProxyManager = &ProxyManager{}

func GetProxyManager() *ProxyManager {
	return globalProxyManager
}
