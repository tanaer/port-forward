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
		Port:            cfg.InboundPort,
		UUID:            cfg.UUID,
		Flow:            cfg.Flow,
		RealityDest:     cfg.RealityDest,
		ServerNames:     []string{cfg.RealityServerName},
		PrivateKey:      cfg.PrivateKey,
		ShortIds:        []string{cfg.ShortId},
		OutboundType:    cfg.OutboundType,
		Socks5Addr:      cfg.Socks5Addr,
		Socks5Port:      socks5Port,  // 根据OutboundType选择正确端口
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
	bridgeManager := GetBridgeManager()
	// 重新计算正确的SOCKS5端口
	var socks5Port int
	if cfg.OutboundType == "hysteria2" {
		socks5Port = cfg.Hy2Socks5Port
	} else if cfg.OutboundType == "socks5" {
		socks5Port = cfg.Socks5Port
	} else {
		socks5Port = cfg.Hy2Socks5Port
	}
	bridge := bridgeManager.AddBridge(id, socks5Port)

	// 如果是 SOCKS5 或 VMess 出站，只启动 Xray
	if cfg.OutboundType == "socks5" || cfg.OutboundType == "vmess" {
		// 初始化 Xray 管理器
		baseDir := filepath.Join(".", "proxy_configs")
		xrayConfigPath := filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", cfg.Id))

		bridge.xrayManager = xray.NewManager(xrayConfigPath)
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

// StopProxy 停止代理
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
		if hy2Manager.IsRunning(id) {
			if err := hy2Manager.Stop(id); err != nil {
				fmt.Printf("[Proxy] 停止Hysteria2实例失败: %v\n", err)
			}
		} else {
			fmt.Printf("[Proxy] Hysteria2实例 %d 未运行，跳过停止\n", id)
		}
	}

	// 停止桥接
	if exists {
		if err := bridge.Stop(); err != nil {
			return fmt.Errorf("停止代理失败: %v", err)
		}
	} else {
		fmt.Printf("[Proxy] 桥接 %d 未运行，跳过停止\n", id)
	}

	// 更新状态
	sql.UpdateProxyStatus(id, 1)

	return nil
}

// RestartProxy 重启代理
func (pm *ProxyManager) RestartProxy(id int) error {
	cfg := sql.GetProxy(id)
	if cfg.Id == 0 {
		return fmt.Errorf("代理配置不存在")
	}

	// 如果是Hysteria2出站，使用Hysteria2Manager重启
	if cfg.OutboundType == "hysteria2" {
		hy2Manager := hysteria.GetGlobalManager()
		if err := hy2Manager.UpdateAndRestart(id, cfg); err != nil {
			return fmt.Errorf("重启Hysteria2实例失败: %v", err)
		}
	}

	// 重新创建配置文件和Xray配置
	if err := pm.CreateProxyFromConfig(cfg); err != nil {
		return err
	}

	// 根据出站类型选择重启方式
	if cfg.OutboundType == "socks5" || cfg.OutboundType == "vmess" {
		// SOCKS5/VMess 只需重启 Xray
		bridgeManager := GetBridgeManager()
		bridge, exists := bridgeManager.GetBridge(id)
		if exists && bridge.xrayManager != nil {
			if err := bridge.xrayManager.Restart(); err != nil {
				return fmt.Errorf("重启Xray失败: %v", err)
			}
		}
	} else {
		// Hysteria2 重启桥接（包含Xray）
		bridgeManager := GetBridgeManager()
		bridge, exists := bridgeManager.GetBridge(id)
		if exists {
			if err := bridge.Restart(); err != nil {
				return fmt.Errorf("重启桥接失败: %v", err)
			}
		}
	}

	// 更新状态
	sql.UpdateProxyStatus(id, 0)

	return nil
}

// DeleteProxy 删除代理
func (pm *ProxyManager) DeleteProxy(id int) error {
	cfg := sql.GetProxy(id)
	if cfg.Id == 0 {
		return fmt.Errorf("代理配置不存在")
	}

	// 如果是Hysteria2出站，使用Hysteria2Manager删除
	if cfg.OutboundType == "hysteria2" {
		hy2Manager := hysteria.GetGlobalManager()
		if err := hy2Manager.Delete(id); err != nil {
			fmt.Printf("[Proxy] 删除Hysteria2实例失败: %v\n", err)
		}
	}

	// 停止桥接
	bridgeManager := GetBridgeManager()
	if bridge, exists := bridgeManager.GetBridge(id); exists {
		bridge.Stop()
		bridgeManager.RemoveBridge(id)
	}

	// 删除配置文件
	baseDir := filepath.Join(".", "proxy_configs")
	os.Remove(filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", id)))

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
