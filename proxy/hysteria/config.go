package hysteria

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"goForward/conf"
)

// Hy2Config Hysteria2配置
type Hy2Config struct {
	Server    string           `yaml:"server"`
	Auth      string           `yaml:"auth,omitempty"`
	Password  string           `yaml:"password,omitempty"`
	Obfs      *ObfsConfig      `yaml:"obfs,omitempty"`
	TLS       *TLSConfig       `yaml:"tls,omitempty"`
	Bandwidth *BandwidthConfig `yaml:"bandwidth,omitempty"`
	Socks5    *Socks5Config    `yaml:"socks5,omitempty"`
	HTTP      *HTTPConfig      `yaml:"http,omitempty"`
}

// ObfsConfig 混淆配置
type ObfsConfig struct {
	Type     string `yaml:"type"`
	Password string `yaml:"password"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	SNI      string `yaml:"sni,omitempty"`
	Insecure bool   `yaml:"insecure,omitempty"`
}

// BandwidthConfig 带宽配置
type BandwidthConfig struct {
	Up   string `yaml:"up"`
	Down string `yaml:"down"`
}

// Socks5Config SOCKS5配置
type Socks5Config struct {
	Listen string `yaml:"listen"`
}

// ListenPort 从Listen字段提取端口号
func (s *Socks5Config) ListenPort() int {
	if s == nil || s.Listen == "" {
		return 10808 // 默认端口
	}
	parts := strings.Split(s.Listen, ":")
	if len(parts) == 2 {
		port, err := strconv.Atoi(parts[1])
		if err == nil {
			return port
		}
	}
	return 10808 // 默认端口
}

// HTTPConfig HTTP代理配置
type HTTPConfig struct {
	Listen string `yaml:"listen"`
}

// Hy2ConfigParams Hysteria2配置参数
type Hy2ConfigParams struct {
	Server     string
	Port       string
	Password   string
	Obfs       string
	ObfsPass   string
	SNI        string
	Insecure   bool
	UpMbps     int
	DownMbps   int
	Socks5Port int
}

// GenerateHy2Config 生成Hysteria2配置
func GenerateHy2Config(params Hy2ConfigParams) *Hy2Config {
	config := &Hy2Config{
		Server: fmt.Sprintf("%s:%s", params.Server, params.Port),
		Bandwidth: &BandwidthConfig{
			Up:   fmt.Sprintf("%d mbps", params.UpMbps),
			Down: fmt.Sprintf("%d mbps", params.DownMbps),
		},
		Socks5: &Socks5Config{
			Listen: fmt.Sprintf("127.0.0.1:%d", params.Socks5Port),
		},
	}

	// 设置认证
	if params.Password != "" {
		config.Auth = params.Password
	}

	// 设置混淆
	if params.Obfs != "" && params.ObfsPass != "" {
		config.Obfs = &ObfsConfig{
			Type:     params.Obfs,
			Password: params.ObfsPass,
		}
	}

	// 设置TLS
	if params.SNI != "" || params.Insecure {
		config.TLS = &TLSConfig{
			SNI:      params.SNI,
			Insecure: params.Insecure,
		}
	}

	return config
}

// SaveConfig 保存配置到文件
func (c *Hy2Config) SaveConfig(filename string) error {
	data, err := yaml.Marshal(c)
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
func LoadConfig(filename string) (*Hy2Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Hy2Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &config, nil
}

// parseBandwidth 解析带宽字符串 (如 "100 mbps")
func parseBandwidth(bw string) (int, error) {
	bw = strings.ToLower(strings.TrimSpace(bw))
	bw = strings.ReplaceAll(bw, " ", "")

	// 移除单位
	bw = strings.TrimSuffix(bw, "mbps")
	bw = strings.TrimSuffix(bw, "mb")
	bw = strings.TrimSuffix(bw, "m")

	return strconv.Atoi(bw)
}

// GenerateHy2ConfigParams 从ProxyConfig生成Hysteria2配置参数
func GenerateHy2ConfigParams(cfg conf.ProxyConfig) Hy2ConfigParams {
	return Hy2ConfigParams{
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
	}
}
