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
	execPath, _ := os.Executable()
	baseDir := filepath.Join(filepath.Dir(execPath), "proxy_configs")
	os.MkdirAll(baseDir, 0755)

	// 生成Xray配置
	xrayConfig := xray.GenerateVLESSRealityConfig(xray.VLESSRealityConfig{
		Port:            cfg.InboundPort,
		UUID:            cfg.UUID,
		Flow:            cfg.Flow,
		RealityDest:     cfg.RealityDest,
		ServerNames:     []string{cfg.RealityServerName},
		PrivateKey:      cfg.PrivateKey,
		ShortIds:        []string{cfg.ShortId},
		OutboundType:    cfg.OutboundType,
		Socks5Addr:      cfg.Socks5Addr,
		Socks5Port:      cfg.Hy2Socks5Port,
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
	})

	xrayConfigPath := filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", cfg.Id))
	if err := xrayConfig.SaveConfig(xrayConfigPath); err != nil {
		return fmt.Errorf("保存Xray配置失败: %v", err)
	}

	// 如果出站类型是 Hysteria2，生成 Hysteria2 配置
	if cfg.OutboundType == "hysteria2" {
		hy2Config := hysteria.GenerateHy2Config(hysteria.Hy2ConfigParams{
		Server:     cfg.Hy2Server,
		Port:       cfg.Hy2Port,
		Password:   cfg.Hy2Password,
		Obfs:       cfg.Hy2Obfs,
		ObfsPass:   cfg.Hy2ObfsPassword,
		SNI:        cfg.Hy2SNI,
		Insecure:   cfg.Hy2Insecure,
		UpMbps:     cfg.Hy2UpMbps,
		DownMbps:   cfg.Hy2DownMbps,
		Socks5Port: cfg.Hy2Socks5Port,
	})

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

	// 创建配置文件
	if err := pm.CreateProxyFromConfig(cfg); err != nil {
		return err
	}

	// 获取或创建桥接
	bridgeManager := GetBridgeManager()
	bridge := bridgeManager.AddBridge(id, cfg.Hy2Socks5Port)

	// 如果是 SOCKS5 或 VMess 出站，只启动 Xray
	if cfg.OutboundType == "socks5" || cfg.OutboundType == "vmess" {
		// 初始化 Xray 管理器
		execPath, _ := os.Executable()
		baseDir := filepath.Join(filepath.Dir(execPath), "proxy_configs")
		xrayConfigPath := filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", cfg.Id))

		bridge.xrayManager = xray.NewManager(xrayConfigPath)
		if err := bridge.xrayManager.Start(); err != nil {
			return fmt.Errorf("启动Xray失败: %v", err)
		}
		bridge.running = true
	} else {
		// 启动完整桥接（Xray + Hysteria2）
		if err := bridge.Start(); err != nil {
			return fmt.Errorf("启动代理失败: %v", err)
		}
	}

	// 更新状态
	sql.UpdateProxyStatus(id, 0)

	return nil
}

// StopProxy 停止代理
func (pm *ProxyManager) StopProxy(id int) error {
	bridgeManager := GetBridgeManager()
	bridge, exists := bridgeManager.GetBridge(id)
	if !exists {
		// 如果 bridge 不存在，尝试直接停止 Xray 进程
		fmt.Printf("[Proxy] Bridge 不存在，尝试清理进程...\n")
		// 更新状态为已停止
		sql.UpdateProxyStatus(id, 1)
		return nil
	}

	if err := bridge.Stop(); err != nil {
		return fmt.Errorf("停止代理失败: %v", err)
	}

	// 更新状态
	sql.UpdateProxyStatus(id, 1)

	return nil
}

// RestartProxy 重启代理
func (pm *ProxyManager) RestartProxy(id int) error {
	if err := pm.StopProxy(id); err != nil {
		fmt.Printf("停止代理失败: %v\n", err)
	}

	return pm.StartProxy(id)
}

// DeleteProxy 删除代理
func (pm *ProxyManager) DeleteProxy(id int) error {
	// 先停止代理
	bridgeManager := GetBridgeManager()
	if bridge, exists := bridgeManager.GetBridge(id); exists {
		bridge.Stop()
		bridgeManager.RemoveBridge(id)
	}

	// 删除配置文件
	execPath, _ := os.Executable()
	baseDir := filepath.Join(filepath.Dir(execPath), "proxy_configs")
	os.Remove(filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", id)))
	os.Remove(filepath.Join(baseDir, fmt.Sprintf("hy2_%d.yaml", id)))

	// 删除数据库记录
	if !sql.DeleteProxy(id) {
		return fmt.Errorf("删除代理配置失败")
	}

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
