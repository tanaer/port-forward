package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// XrayConfig Xray完整配置
type XrayConfig struct {
	Log       LogConfig        `json:"log"`
	Inbounds  []InboundConfig  `json:"inbounds"`
	Outbounds []OutboundConfig `json:"outbounds"`
	Routing   *RoutingConfig   `json:"routing,omitempty"`
}

// RoutingConfig 路由配置
type RoutingConfig struct {
	DomainStrategy string        `json:"domainStrategy"`
	Rules          []RoutingRule `json:"rules"`
}

// RoutingRule 路由规则
type RoutingRule struct {
	Type        string   `json:"type"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
}

// LogConfig 日志配置
type LogConfig struct {
	Loglevel string `json:"loglevel"`
}

// InboundConfig 入站配置
type InboundConfig struct {
	Listen         string                 `json:"listen"`
	Port           int                    `json:"port"`
	Protocol       string                 `json:"protocol"`
	Settings       map[string]interface{} `json:"settings"`
	StreamSettings StreamSettings         `json:"streamSettings"`
	Tag            string                 `json:"tag"`
}

// OutboundConfig 出站配置
type OutboundConfig struct {
	Protocol       string                 `json:"protocol"`
	Settings       map[string]interface{} `json:"settings"`
	StreamSettings map[string]interface{} `json:"streamSettings,omitempty"`
	Tag            string                 `json:"tag"`
}

// StreamSettings 传输配置
type StreamSettings struct {
	Network         string                 `json:"network"`
	Security        string                 `json:"security"`
	RealitySettings map[string]interface{} `json:"realitySettings,omitempty"`
}

// VLESSRealityConfig VLESS+Reality配置参数
type VLESSRealityConfig struct {
	Port           int
	UUID           string
	Flow           string
	RealityDest    string
	ServerNames    []string
	PrivateKey     string
	ShortIds       []string
	OutboundType   string // "hysteria2", "socks5", 或 "vmess"
	Socks5Addr     string // SOCKS5 地址
	Socks5Port     int    // SOCKS5 端口
	Socks5User     string // SOCKS5 用户名
	Socks5Password string // SOCKS5 密码
	// VMess 出站参数
	VmessServer     string
	VmessPort       int
	VmessUUID       string
	VmessAlterID    int
	VmessSecurity   string
	VmessNetwork    string
	VmessTLS        bool
	VmessServerName string
	VmessWsPath     string
	VmessWsHost     string
}

// GenerateVLESSRealityConfig 生成VLESS+Reality配置
func GenerateVLESSRealityConfig(cfg VLESSRealityConfig) *XrayConfig {
	return &XrayConfig{
		Log: LogConfig{
			Loglevel: "debug",
		},
		Inbounds: []InboundConfig{
			{
				Listen:   "0.0.0.0",
				Port:     cfg.Port,
				Protocol: "vless",
				Tag:      "vless-in",
				Settings: map[string]interface{}{
					"clients": []map[string]interface{}{
						{
							"id":   cfg.UUID,
							"flow": cfg.Flow,
						},
					},
					"decryption": "none",
				},
				StreamSettings: StreamSettings{
					Network:  "tcp",
					Security: "reality",
					RealitySettings: map[string]interface{}{
						"show":        false,
						"dest":        cfg.RealityDest,
						"serverNames": cfg.ServerNames,
						"privateKey":  cfg.PrivateKey,
						"shortIds":    cfg.ShortIds,
					},
				},
			},
		},
		Outbounds: generateOutbounds(cfg),
		Routing: &RoutingConfig{
			DomainStrategy: "AsIs",
			Rules: []RoutingRule{
				{
					Type:        "field",
					InboundTag:  []string{"vless-in"},
					OutboundTag: getOutboundTag(cfg.OutboundType),
				},
			},
		},
	}
}

// getOutboundTag 根据出站类型获取标签
func getOutboundTag(outboundType string) string {
	switch outboundType {
	case "vmess":
		return "vmess-out"
	case "socks5":
		return "socks-out"
	default:
		return "socks-out"
	}
}

// SaveConfig 保存配置到文件
func (c *XrayConfig) SaveConfig(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*XrayConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config XrayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &config, nil
}

// generateOutbounds 生成出站配置
func generateOutbounds(cfg VLESSRealityConfig) []OutboundConfig {
	outbounds := []OutboundConfig{}

	if cfg.OutboundType == "socks5" {
		// 直接使用 SOCKS5 出站
		serverConfig := map[string]interface{}{
			"address": cfg.Socks5Addr,
			"port":    cfg.Socks5Port,
		}

		// 如果提供了用户名和密码，添加认证信息
		if cfg.Socks5User != "" || cfg.Socks5Password != "" {
			serverConfig["users"] = []map[string]interface{}{
				{
					"user": cfg.Socks5User,
					"pass": cfg.Socks5Password,
				},
			}
		}

		outbounds = append(outbounds, OutboundConfig{
			Protocol: "socks",
			Tag:      "socks-out",
			Settings: map[string]interface{}{
				"servers": []map[string]interface{}{serverConfig},
			},
		})
	} else if cfg.OutboundType == "vmess" {
		// 使用 VMess 出站
		// 构建 streamSettings
		streamSettings := map[string]interface{}{
			"network": cfg.VmessNetwork,
		}

		// WebSocket 配置
		if cfg.VmessNetwork == "ws" {
			wsSettings := map[string]interface{}{}
			if cfg.VmessWsPath != "" {
				wsSettings["path"] = cfg.VmessWsPath
			}
			if cfg.VmessWsHost != "" {
				wsSettings["headers"] = map[string]interface{}{
					"Host": cfg.VmessWsHost,
				}
			}
			if len(wsSettings) > 0 {
				streamSettings["wsSettings"] = wsSettings
			}
		}

		// TLS 配置
		if cfg.VmessTLS {
			streamSettings["security"] = "tls"
			tlsSettings := map[string]interface{}{}
			if cfg.VmessServerName != "" {
				tlsSettings["serverName"] = cfg.VmessServerName
			}
			streamSettings["tlsSettings"] = tlsSettings
		}

		vmessOutbound := OutboundConfig{
			Protocol: "vmess",
			Tag:      "vmess-out",
			Settings: map[string]interface{}{
				"vnext": []map[string]interface{}{
					{
						"address": cfg.VmessServer,
						"port":    cfg.VmessPort,
						"users": []map[string]interface{}{
							{
								"id":       cfg.VmessUUID,
								"alterId":  cfg.VmessAlterID,
								"security": cfg.VmessSecurity,
							},
						},
					},
				},
			},
			StreamSettings: streamSettings,
		}

		outbounds = append(outbounds, vmessOutbound)
	} else {
		// 使用 Hysteria2 (通过本地 SOCKS5)
		outbounds = append(outbounds, OutboundConfig{
			Protocol: "socks",
			Tag:      "socks-out",
			Settings: map[string]interface{}{
				"servers": []map[string]interface{}{
					{
						"address": "127.0.0.1",
						"port":    cfg.Socks5Port,
					},
				},
			},
		})
	}

	// 添加 direct 出站
	outbounds = append(outbounds, OutboundConfig{
		Protocol: "freedom",
		Tag:      "direct",
		Settings: map[string]interface{}{},
	})

	return outbounds
}
