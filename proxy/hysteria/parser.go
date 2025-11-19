package hysteria

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ParseSubscription 解析Hysteria2订阅链接
func ParseSubscription(subUrl string) (*Hy2Config, error) {
	// 判断是HTTP订阅还是hysteria2://协议
	if strings.HasPrefix(subUrl, "http://") || strings.HasPrefix(subUrl, "https://") {
		return parseHTTPSubscription(subUrl)
	} else if strings.HasPrefix(subUrl, "hysteria2://") || strings.HasPrefix(subUrl, "hy2://") {
		return parseHysteria2URI(subUrl)
	}

	return nil, fmt.Errorf("不支持的订阅格式")
}

// parseHTTPSubscription 解析HTTP订阅链接
func parseHTTPSubscription(subUrl string) (*Hy2Config, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(subUrl)
	if err != nil {
		return nil, fmt.Errorf("获取订阅失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取订阅内容失败: %v", err)
	}

	// 尝试base64解码
	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		// 如果不是base64，直接使用原始内容
		decoded = body
	}

	content := string(decoded)
	lines := strings.Split(content, "\n")

	// 查找第一个hysteria2链接
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "hysteria2://") || strings.HasPrefix(line, "hy2://") {
			return parseHysteria2URI(line)
		}
	}

	return nil, fmt.Errorf("订阅中未找到有效的Hysteria2节点")
}

// parseHysteria2URI 解析hysteria2://协议链接
func parseHysteria2URI(uri string) (*Hy2Config, error) {
	// hysteria2://password@server:port?obfs=salamander&obfs-password=xxx&sni=example.com&insecure=1#name

	// 移除协议前缀
	uri = strings.TrimPrefix(uri, "hysteria2://")
	uri = strings.TrimPrefix(uri, "hy2://")

	// 分离备注
	parts := strings.SplitN(uri, "#", 2)
	uri = parts[0]

	// 分离参数
	parts = strings.SplitN(uri, "?", 2)
	authAndServer := parts[0]
	var params url.Values
	if len(parts) == 2 {
		var err error
		params, err = url.ParseQuery(parts[1])
		if err != nil {
			return nil, fmt.Errorf("解析参数失败: %v", err)
		}
	}

	// 解析认证和服务器
	parts = strings.SplitN(authAndServer, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("URI格式错误")
	}

	password := parts[0]
	serverAddr := parts[1]

	// 解析服务器地址和端口
	host, port, err := parseServerAddr(serverAddr)
	if err != nil {
		return nil, err
	}

	config := &Hy2Config{
		Server: fmt.Sprintf("%s:%s", host, port),
		Auth:   password,
		Bandwidth: &BandwidthConfig{
			Up:   "100 mbps",
			Down: "100 mbps",
		},
		// SOCKS5端口会在web端处理时动态分配，这里暂时设置为0
	}

	// 解析可选参数
	if params != nil {
		// 混淆
		if obfsType := params.Get("obfs"); obfsType != "" {
			obfsPass := params.Get("obfs-password")
			if obfsPass == "" {
				obfsPass = params.Get("obfsPassword")
			}
			if obfsPass != "" {
				config.Obfs = &ObfsConfig{
					Type:     obfsType,
					Password: obfsPass,
				}
			}
		}

		// TLS配置
		sni := params.Get("sni")
		insecure := params.Get("insecure") == "1"
		if sni != "" || insecure {
			config.TLS = &TLSConfig{
				SNI:      sni,
				Insecure: insecure,
			}
		}

		// 带宽配置
		if up := params.Get("up"); up != "" {
			if config.Bandwidth == nil {
				config.Bandwidth = &BandwidthConfig{}
			}
			config.Bandwidth.Up = up
		}
		if down := params.Get("down"); down != "" {
			if config.Bandwidth == nil {
				config.Bandwidth = &BandwidthConfig{}
			}
			config.Bandwidth.Down = down
		}
	}

	return config, nil
}

// parseServerAddr 解析服务器地址
func parseServerAddr(addr string) (host, port string, err error) {
	// 处理IPv6地址
	if strings.HasPrefix(addr, "[") {
		endIdx := strings.Index(addr, "]")
		if endIdx == -1 {
			return "", "", fmt.Errorf("无效的IPv6地址")
		}
		host = addr[1:endIdx]
		if len(addr) > endIdx+1 && addr[endIdx+1] == ':' {
			port = addr[endIdx+2:]
		} else {
			port = "443"
		}
		return host, port, nil
	}

	// 处理IPv4或域名
	parts := strings.Split(addr, ":")
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	} else if len(parts) == 1 {
		return parts[0], "443", nil
	}

	return "", "", fmt.Errorf("无效的服务器地址")
}

// ParseHysteria2JSON 解析JSON格式的Hysteria2配置
func ParseHysteria2JSON(jsonStr string) (*Hy2Config, error) {
	var config Hy2Config
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("解析JSON配置失败: %v", err)
	}
	return &config, nil
}

// Hy2ConfigToParams 将Hy2Config转换为参数结构
func Hy2ConfigToParams(config *Hy2Config) Hy2ConfigParams {
	params := Hy2ConfigParams{
		UpMbps:     100,
		DownMbps:   100,
		Socks5Port: 0, // 不设置默认值，由web端动态分配
	}

	// 解析服务器地址
	if config.Server != "" {
		parts := strings.Split(config.Server, ":")
		if len(parts) >= 1 {
			params.Server = parts[0]
		}
		if len(parts) >= 2 {
			params.Port = parts[1]
		}
	}

	// 密码
	if config.Auth != "" {
		params.Password = config.Auth
	} else if config.Password != "" {
		params.Password = config.Password
	}

	// 混淆
	if config.Obfs != nil {
		params.Obfs = config.Obfs.Type
		params.ObfsPass = config.Obfs.Password
	}

	// TLS
	if config.TLS != nil {
		params.SNI = config.TLS.SNI
		params.Insecure = config.TLS.Insecure
	}

	// 带宽
	if config.Bandwidth != nil {
		if up, err := parseBandwidth(config.Bandwidth.Up); err == nil {
			params.UpMbps = up
		}
		if down, err := parseBandwidth(config.Bandwidth.Down); err == nil {
			params.DownMbps = down
		}
	}

	// SOCKS5端口
	if config.Socks5 != nil && config.Socks5.Listen != "" {
		parts := strings.Split(config.Socks5.Listen, ":")
		if len(parts) == 2 {
			if port, err := strconv.Atoi(parts[1]); err == nil {
				params.Socks5Port = port
			}
		}
	}

	return params
}
