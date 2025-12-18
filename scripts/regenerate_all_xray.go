//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"goForward/conf"
	"goForward/proxy/xray"
	"gorm.io/gorm"
)

func main() {
	dbPath := getDBPath()
	fmt.Printf("使用数据库: %s\n", dbPath)

	proxies, err := loadActiveProxies(dbPath)
	if err != nil {
		log.Fatalf("加载代理失败: %v\n", err)
	}

	if len(proxies) == 0 {
		fmt.Println("未找到 status=0 的活动代理配置，任务结束。")
		return
	}

	baseDir := filepath.Join(".", "proxy_configs")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		log.Fatalf("创建配置目录失败: %v\n", err)
	}

	total := len(proxies)
	success := 0
	var failures []string

	fmt.Printf("共找到 %d 个活动代理，开始重新生成配置...\n", total)

	for idx, proxyCfg := range proxies {
		displayName := proxyCfg.Name
		if displayName == "" {
			displayName = fmt.Sprintf("Proxy-%d", proxyCfg.Id)
		}

		fmt.Printf("[%d/%d] 处理代理 %d (%s)\n", idx+1, total, proxyCfg.Id, displayName)

		outputPath, err := regenerateXrayConfig(baseDir, proxyCfg)
		if err != nil {
			failMsg := fmt.Sprintf("代理 %d (%s): %v", proxyCfg.Id, displayName, err)
			failures = append(failures, failMsg)
			fmt.Printf("    失败: %v\n", err)
			continue
		}

		success++
		fmt.Printf("    已写入 %s\n", outputPath)
	}

	fmt.Printf("任务完成：总计 %d，成功 %d，失败 %d\n", total, success, len(failures))
	if len(failures) > 0 {
		fmt.Println("失败详情：")
		for _, msg := range failures {
			fmt.Printf("  - %s\n", msg)
		}
	}
}

func getDBPath() string {
	if path := os.Getenv("GOFORWARD_DB_PATH"); path != "" {
		return path
	}
	return filepath.Join(".", "goForward.db")
}

func loadActiveProxies(dbPath string) ([]conf.ProxyConfig, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("数据库文件不存在: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层数据库连接失败: %w", err)
	}
	defer sqlDB.Close()

	var proxies []conf.ProxyConfig
	if err := db.Where("status = ?", 0).Order("id ASC").Find(&proxies).Error; err != nil {
		return nil, fmt.Errorf("查询活动代理失败: %w", err)
	}
	return proxies, nil
}

func regenerateXrayConfig(baseDir string, cfg conf.ProxyConfig) (string, error) {
	logDir := conf.ProxyLogDir(cfg.Id)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("创建日志目录失败: %w", err)
	}

	xrayCfg := buildXrayConfig(cfg)
	outputPath := filepath.Join(baseDir, fmt.Sprintf("xray_%d.json", cfg.Id))
	if err := xrayCfg.SaveConfig(outputPath); err != nil {
		return "", fmt.Errorf("写入 Xray 配置失败: %w", err)
	}
	return outputPath, nil
}

func buildXrayConfig(cfg conf.ProxyConfig) *xray.XrayConfig {
	socks5Port := cfg.Hy2Socks5Port
	switch cfg.OutboundType {
	case "socks5":
		socks5Port = cfg.Socks5Port
	case "hysteria2":
		socks5Port = cfg.Hy2Socks5Port
	default:
		socks5Port = cfg.Hy2Socks5Port
	}

	serverNames := []string{}
	if cfg.RealityServerName != "" {
		serverNames = append(serverNames, cfg.RealityServerName)
	}

	shortIds := []string{}
	if cfg.ShortId != "" {
		shortIds = append(shortIds, cfg.ShortId)
	}

	return xray.GenerateVLESSRealityConfig(xray.VLESSRealityConfig{
		ProxyID:         cfg.Id,
		Port:            cfg.InboundPort,
		UUID:            cfg.UUID,
		Flow:            cfg.Flow,
		RealityDest:     cfg.RealityDest,
		ServerNames:     serverNames,
		PrivateKey:      cfg.PrivateKey,
		ShortIds:        shortIds,
		OutboundType:    cfg.OutboundType,
		Socks5Addr:      cfg.Socks5Addr,
		Socks5Port:      socks5Port,
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
		LogDir:          conf.ProxyLogDir(cfg.Id),
		APIPort:         conf.XrayAPIPort(cfg.Id),
	})
}
