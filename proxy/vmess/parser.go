package vmess

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// VmessConfig VMess配置
type VmessConfig struct {
	Server     string
	Port       int
	UUID       string
	AlterID    int
	Security   string
	Network    string
	TLS        bool
	ServerName string
	WsPath     string
	WsHost     string
}

// VmessJSON VMess JSON 格式（vmess:// 链接解析后的格式）
type VmessJSON struct {
	V    string      `json:"v"`    // 版本
	PS   string      `json:"ps"`   // 备注
	Add  string      `json:"add"`  // 服务器地址
	Port interface{} `json:"port"` // 端口（可能是字符串或整数）
	ID   string      `json:"id"`   // UUID
	Aid  interface{} `json:"aid"`  // alterID（可能是字符串或整数）
	Scy  string      `json:"scy"`  // 加密方式
	Net  string      `json:"net"`  // 传输协议
	Type string      `json:"type"` // 伪装类型
	Host string      `json:"host"` // WebSocket Host
	Path string      `json:"path"` // WebSocket 路径
	TLS  string      `json:"tls"`  // TLS
	SNI  string      `json:"sni"`  // Server Name
}

// ParseSubscription 解析 VMess 订阅
func ParseSubscription(subUrl string) (*VmessConfig, error) {
	if strings.HasPrefix(subUrl, "http://") || strings.HasPrefix(subUrl, "https://") {
		return parseHTTPSubscription(subUrl)
	} else if strings.HasPrefix(subUrl, "vmess://") {
		return parseVmessURI(subUrl)
	}
	return nil, fmt.Errorf("不支持的订阅格式")
}

// parseHTTPSubscription 解析 HTTP 订阅
func parseHTTPSubscription(subUrl string) (*VmessConfig, error) {
	resp, err := http.Get(subUrl)
	if err != nil {
		return nil, fmt.Errorf("请求订阅失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 订阅内容可能是 base64 编码的
	content := strings.TrimSpace(string(body))
	if decoded, err := base64.StdEncoding.DecodeString(content); err == nil {
		content = string(decoded)
	}

	// 解析第一个 vmess:// 链接
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "vmess://") {
			return parseVmessURI(line)
		}
	}

	return nil, fmt.Errorf("未找到有效的 VMess 配置")
}

// parseVmessURI 解析 vmess:// 链接
func parseVmessURI(uri string) (*VmessConfig, error) {
	// 先移除前缀和空格
	uri = strings.TrimSpace(uri)
	encoded := strings.TrimPrefix(uri, "vmess://")
	encoded = strings.TrimSpace(encoded)

	// 检查是否是 URI 格式（包含 @ 符号且不是 base64）
	// URI 格式: security:uuid@host:port?params
	// Base64 格式: 一长串 base64 字符
	if strings.Contains(encoded, "@") && strings.Contains(encoded, ":") {
		// 检查 @ 前面是否有冒号（URI 格式特征）
		atIndex := strings.Index(encoded, "@")
		if atIndex > 0 {
			beforeAt := encoded[:atIndex]
			if strings.Contains(beforeAt, ":") {
				// 这是 URI 格式
				return parseVmessURIFormat(uri)
			}
		}
	}

	// 否则是 JSON base64 格式
	return parseVmessBase64Format(encoded)
}

// parseVmessURIFormat 解析 URI 格式的 VMess 链接
// 格式: vmess://security:uuid@host:port?params#remarks
func parseVmessURIFormat(uri string) (*VmessConfig, error) {
	// 移除前缀
	uri = strings.TrimPrefix(uri, "vmess://")

	// 分离 remarks (# 后面的部分)
	parts := strings.SplitN(uri, "#", 2)
	mainPart := parts[0]

	// 分离参数 (? 后面的部分)
	parts = strings.SplitN(mainPart, "?", 2)
	userAndHost := parts[0]
	params := ""
	if len(parts) > 1 {
		params = parts[1]
	}

	// 分离用户信息和主机信息 (@分隔)
	parts = strings.SplitN(userAndHost, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的 URI 格式")
	}

	userInfo := parts[0]
	hostPort := parts[1]

	// 解析用户信息 (security:uuid)
	userParts := strings.SplitN(userInfo, ":", 2)
	if len(userParts) != 2 {
		return nil, fmt.Errorf("无效的用户信息格式")
	}

	security := userParts[0]
	uuid := userParts[1]

	// 解析主机和端口
	hostParts := strings.SplitN(hostPort, ":", 2)
	if len(hostParts) != 2 {
		return nil, fmt.Errorf("无效的主机:端口格式")
	}

	host := hostParts[0]
	portStr := hostParts[1]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("端口格式错误: %v", err)
	}

	config := &VmessConfig{
		Server:   host,
		Port:     port,
		UUID:     uuid,
		Security: security,
		Network:  "tcp",
		AlterID:  0,
	}

	// 解析参数
	if params != "" {
		paramMap := make(map[string]string)
		for _, param := range strings.Split(params, "&") {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) == 2 {
				paramMap[kv[0]] = kv[1]
			}
		}

		// 提取有用的参数
		if alterID, ok := paramMap["alterId"]; ok {
			if aid, err := strconv.Atoi(alterID); err == nil {
				config.AlterID = aid
			}
		}

		if obfs, ok := paramMap["obfs"]; ok && obfs != "none" {
			config.Network = obfs
		}
	}

	return config, nil
}

// parseVmessBase64Format 解析 base64 JSON 格式的 VMess 链接
func parseVmessBase64Format(encoded string) (*VmessConfig, error) {
	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// 尝试使用 RawStdEncoding
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			// 尝试使用 URLEncoding
			decoded, err = base64.URLEncoding.DecodeString(encoded)
			if err != nil {
				return nil, fmt.Errorf("Base64 解码失败: %v", err)
			}
		}
	}

	// JSON 解析
	var vmessJSON VmessJSON
	if err := json.Unmarshal(decoded, &vmessJSON); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}

	// 转换为 VmessConfig
	config := &VmessConfig{
		Server:     vmessJSON.Add,
		UUID:       vmessJSON.ID,
		Security:   vmessJSON.Scy,
		Network:    vmessJSON.Net,
		TLS:        vmessJSON.TLS == "tls",
		ServerName: vmessJSON.SNI,
		WsHost:     vmessJSON.Host,
		WsPath:     vmessJSON.Path,
	}

	// 处理端口（可能是字符串或整数）
	switch v := vmessJSON.Port.(type) {
	case string:
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("端口格式错误: %v", err)
		}
		config.Port = port
	case float64:
		config.Port = int(v)
	case int:
		config.Port = v
	default:
		return nil, fmt.Errorf("不支持的端口类型: %T", v)
	}

	// 处理 alterID（可能是字符串或整数）
	switch v := vmessJSON.Aid.(type) {
	case string:
		aid, err := strconv.Atoi(v)
		if err != nil {
			config.AlterID = 0
		} else {
			config.AlterID = aid
		}
	case float64:
		config.AlterID = int(v)
	case int:
		config.AlterID = v
	default:
		config.AlterID = 0
	}

	// 默认值
	if config.Security == "" {
		config.Security = "auto"
	}
	if config.Network == "" {
		config.Network = "tcp"
	}

	return config, nil
}
