package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"goForward/conf"
	"goForward/sql"
)

// ImportConfig 导入配置
type ImportConfig struct {
	// 转发表配置
	Forwards []ForwardConfig `json:"forwards,omitempty" yaml:"forwards,omitempty"`

	// 代理配置
	Proxies []ProxyConfigImport `json:"proxies,omitempty" yaml:"proxies,omitempty"`
}

// ForwardConfig 转发表配置
type ForwardConfig struct {
	LocalPort     string `json:"localPort" yaml:"localPort"`
	RemoteAddr    string `json:"remoteAddr" yaml:"remoteAddr"`
	RemotePort    string `json:"remotePort" yaml:"remotePort"`
	OutTime       int    `json:"outTime" yaml:"outTime"`
	Protocol      string `json:"protocol" yaml:"protocol"`
	Whitelist     string `json:"whitelist" yaml:"whitelist"`
	Blacklist     string `json:"blacklist" yaml:"blacklist"`
	Remark        string `json:"remark" yaml:"remark"`
}

// ProxyConfigImport 代理配置导入
type ProxyConfigImport struct {
	Name      string `json:"name" yaml:"name"`
	Status    int    `json:"status" yaml:"status"`
	Remark    string `json:"remark" yaml:"remark"`

	// 入站配置
	InboundPort int `json:"inboundPort" yaml:"inboundPort"`

	// 出站类型
	OutboundType string `json:"outboundType" yaml:"outboundType"`

	// SOCKS5出站配置
	Socks5Addr     string `json:"socks5Addr" yaml:"socks5Addr"`
	Socks5Port     int    `json:"socks5Port" yaml:"socks5Port"`
	Socks5User     string `json:"socks5User" yaml:"socks5User"`
	Socks5Password string `json:"socks5Password" yaml:"socks5Password"`
}

// ImportFromFile 从文件导入配置
func ImportFromFile(filePath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", filePath)
	}

	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析文件格式
	var config ImportConfig
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".json" {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("JSON格式错误: %v", err)
		}
	} else if ext == ".yaml" || ext == ".yml" {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("YAML格式错误: %v", err)
		}
	} else {
		return fmt.Errorf("不支持的文件格式: %s，支持格式: json, yaml, yml", ext)
	}

	// 导入转发表配置
	if len(config.Forwards) > 0 {
		if err := importForwards(config.Forwards); err != nil {
			return fmt.Errorf("导入转发表配置失败: %v", err)
		}
	}

	// 导入代理配置
	if len(config.Proxies) > 0 {
		if err := importProxies(config.Proxies); err != nil {
			return fmt.Errorf("导入代理配置失败: %v", err)
		}
	}

	return nil
}

// importForwards 导入转发表配置
func importForwards(forwards []ForwardConfig) error {
	successCount := 0
	failCount := 0

	fmt.Println("=== 开始导入转发表配置 ===")
	for i, fwd := range forwards {
		// 验证配置
		if err := validateForwardConfig(&fwd); err != nil {
			fmt.Printf("[%d/%d] 跳过无效配置: %v\n", i+1, len(forwards), err)
			failCount++
			continue
		}

		// 创建ConnectionStats
		connectionStats := conf.ConnectionStats{
			LocalPort:  fwd.LocalPort,
			RemoteAddr: fwd.RemoteAddr,
			RemotePort: fwd.RemotePort,
			OutTime:    fwd.OutTime,
			Protocol:   fwd.Protocol,
			Whitelist:  fwd.Whitelist,
			Blacklist:  fwd.Blacklist,
			Remark:     fwd.Remark,
			Status:     0, // 默认启用
		}

		// 添加到数据库
		if id := sql.AddForward(connectionStats); id == 0 {
			fmt.Printf("[%d/%d] 导入失败: %s:%s -> %s:%s\n",
				i+1, len(forwards), fwd.Protocol, fwd.LocalPort, fwd.RemoteAddr, fwd.RemotePort)
			failCount++
		} else {
			fmt.Printf("[%d/%d] 导入成功: %s:%s -> %s:%s - %s\n",
				i+1, len(forwards), fwd.Protocol, fwd.LocalPort, fwd.RemoteAddr, fwd.RemotePort, fwd.Remark)
			successCount++
		}
	}

	fmt.Printf("\n转发表配置导入完成: 成功 %d 个，失败 %d 个\n", successCount, failCount)
	return nil
}

// importProxies 导入代理配置
func importProxies(proxies []ProxyConfigImport) error {
	successCount := 0
	failCount := 0

	fmt.Println("=== 开始导入代理配置 ===")
	for i, proxy := range proxies {
		// 验证配置
		if err := validateProxyConfig(&proxy); err != nil {
			fmt.Printf("[%d/%d] 跳过无效配置: %v\n", i+1, len(proxies), err)
			failCount++
			continue
		}

		// 创建ProxyConfig
		proxyConfig := conf.ProxyConfig{
			Name:          proxy.Name,
			Status:        proxy.Status,
			Remark:        proxy.Remark,
			InboundPort:   proxy.InboundPort,
			OutboundType:  proxy.OutboundType,
			Socks5Addr:    proxy.Socks5Addr,
			Socks5Port:    proxy.Socks5Port,
			Socks5User:    proxy.Socks5User,
			Socks5Password: proxy.Socks5Password,
		}

		// 添加到数据库
		if err := sql.AddProxyConfig(proxyConfig); err != nil {
			fmt.Printf("[%d/%d] 导入失败: %s - %v\n", i+1, len(proxies), proxy.Name, err)
			failCount++
		} else {
			fmt.Printf("[%d/%d] 导入成功: %s (入站:%d, 出站:%s)\n",
				i+1, len(proxies), proxy.Name, proxy.InboundPort, proxy.OutboundType)
			successCount++
		}
	}

	fmt.Printf("\n代理配置导入完成: 成功 %d 个，失败 %d 个\n", successCount, failCount)
	return nil
}

// validateForwardConfig 验证转发表配置
func validateForwardConfig(fwd *ForwardConfig) error {
	if fwd.LocalPort == "" {
		return fmt.Errorf("localPort不能为空")
	}
	if fwd.RemoteAddr == "" {
		return fmt.Errorf("remoteAddr不能为空")
	}
	if fwd.RemotePort == "" {
		return fmt.Errorf("remotePort不能为空")
	}
	if fwd.Protocol != "tcp" && fwd.Protocol != "udp" {
		return fmt.Errorf("protocol必须是tcp或udp")
	}
	if fwd.OutTime <= 0 {
		fwd.OutTime = 30 // 默认30秒
	}
	return nil
}

// validateProxyConfig 验证代理配置
func validateProxyConfig(proxy *ProxyConfigImport) error {
	if proxy.Name == "" {
		return fmt.Errorf("name不能为空")
	}
	if proxy.InboundPort <= 0 || proxy.InboundPort > 65535 {
		return fmt.Errorf("inboundPort必须在1-65535范围内")
	}
	if proxy.OutboundType == "" {
		proxy.OutboundType = "hysteria2" // 默认
	}
	if proxy.OutboundType == "socks5" {
		if proxy.Socks5Addr == "" {
			return fmt.Errorf("SOCKS5出站时，socks5Addr不能为空")
		}
		if proxy.Socks5Port <= 0 || proxy.Socks5Port > 65535 {
			return fmt.Errorf("socks5Port必须在1-65535范围内")
		}
	}
	return nil
}

// CreateExampleConfig 创建示例配置文件
func CreateExampleConfig(filePath string) error {
	example := ImportConfig{
		Forwards: []ForwardConfig{
			{
				LocalPort:  "9999",
				RemoteAddr: "127.0.0.1",
				RemotePort: "9999",
				OutTime:    30,
				Protocol:   "tcp",
				Whitelist:  "",
				Blacklist:  "",
				Remark:     "示例TCP转发",
			},
		},
		Proxies: []ProxyConfigImport{
			{
				Name:        "SOCKS5代理示例",
				Status:      0,
				Remark:      "示例SOCKS5代理配置",
				InboundPort: 8443,
				OutboundType: "socks5",
				Socks5Addr: "103.129.162.224",
				Socks5Port: 9860,
				Socks5User: "username",
				Socks5Password: "password",
			},
		},
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	var data []byte
	var err error

	if ext == ".json" {
		data, err = json.MarshalIndent(example, "", "  ")
		if err != nil {
			return err
		}
	} else if ext == ".yaml" || ext == ".yml" {
		data, err = yaml.Marshal(example)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("不支持的文件格式: %s，支持格式: json, yaml, yml", ext)
	}

	// 确保目录存在
	if dir := filepath.Dir(filePath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(filePath, data, 0644)
}
