//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"goForward/conf"
	"goForward/proxy/xray"
	"gorm.io/gorm"
)

const targetProxyID = 2

func main() {
	dbPath := getDBPath()
	proxyCfg, err := loadProxyConfig(dbPath, targetProxyID)
	if err != nil {
		log.Fatalf("读取代理配置失败: %v", err)
	}

	xrayCfg := buildXrayConfig(proxyCfg)
	outputPath := filepath.Join("proxy_configs", fmt.Sprintf("xray_%d_new.json", proxyCfg.Id))
	if err := xrayCfg.SaveConfig(outputPath); err != nil {
		log.Fatalf("写入Xray配置失败: %v", err)
	}

	data, err := json.MarshalIndent(xrayCfg, "", "  ")
	if err != nil {
		log.Fatalf("格式化配置失败: %v", err)
	}

	fmt.Println(string(data))
}

func getDBPath() string {
	if path := os.Getenv("GOFORWARD_DB_PATH"); path != "" {
		return path
	}
	return filepath.Join(".", "goForward.db")
}

func loadProxyConfig(dbPath string, id int) (conf.ProxyConfig, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return conf.ProxyConfig{}, fmt.Errorf("数据库文件不存在: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return conf.ProxyConfig{}, fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return conf.ProxyConfig{}, fmt.Errorf("获取底层数据库连接失败: %w", err)
	}
	defer sqlDB.Close()

	var proxyCfg conf.ProxyConfig
	if err := db.Where("id = ?", id).First(&proxyCfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conf.ProxyConfig{}, fmt.Errorf("未找到ID=%d的代理配置", id)
		}
		return conf.ProxyConfig{}, fmt.Errorf("查询代理配置失败: %w", err)
	}
	return proxyCfg, nil
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
