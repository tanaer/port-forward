package proxy

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

// SubscriptionGenerator 订阅生成器
type SubscriptionGenerator struct {
	ServerIP  string
	Port      int
	UUID      string
	PublicKey string
	ShortId   string
	SNI       string
	Remark    string
}

// GenerateVLESSLink 生成VLESS订阅链接
func (sg *SubscriptionGenerator) GenerateVLESSLink() string {
	// vless://UUID@SERVER:PORT/?fp=chrome&security=reality&pbk=PUBLIC_KEY&sid=SHORT_ID&sni=SNI&type=tcp&flow=xtls-rprx-vision#REMARK
	// 参考标准格式：vless://uuid@server:port/?fp=chrome&security=reality&pbk=xxx&sid=xxx&sni=xxx&type=tcp&flow=xtls-rprx-vision&encryption=none#name

	remark := sg.Remark
	if remark == "" {
		remark = "VLESS-Reality"
	}

	// 按照 v2rayN 和 Reality 标准格式构建链接
	// 注意：使用 /? 而不是 ? （兼容性更好）
	// 参数顺序：fp → security → pbk → sid → sni → type → flow
	// encryption=none 对于 Reality 可以省略，因为 VLESS 本身无加密
	link := fmt.Sprintf("vless://%s@%s:%d?type=tcp&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&flow=xtls-rprx-vision#%s",
		sg.UUID,
		sg.ServerIP,
		sg.Port,
		url.QueryEscape(sg.PublicKey),
		url.QueryEscape(sg.SNI),
		url.QueryEscape(sg.ShortId),
		url.QueryEscape(remark),
	)

	return link
}

// GenerateSubscriptionContent 生成订阅内容（base64编码）
func (sg *SubscriptionGenerator) GenerateSubscriptionContent() string {
	link := sg.GenerateVLESSLink()
	return base64.StdEncoding.EncodeToString([]byte(link))
}

// GenerateClashConfig 生成Clash配置
func (sg *SubscriptionGenerator) GenerateClashConfig() string {
	config := fmt.Sprintf(`proxies:
  - name: %s
    type: vless
    server: %s
    port: %d
    uuid: %s
    network: tcp
    tls: true
    udp: true
    flow: xtls-rprx-vision
    servername: %s
    reality-opts:
      public-key: %s
      short-id: %s
    client-fingerprint: chrome
`,
		sg.Remark,
		sg.ServerIP,
		sg.Port,
		sg.UUID,
		sg.SNI,
		sg.PublicKey,
		sg.ShortId,
	)

	return config
}

// GenerateAccessToken 生成访问令牌
func GenerateAccessToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// GenerateUUID 生成UUID
func GenerateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	// 设置版本(4)和变体位
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ParseVLESSLink 解析VLESS链接
func ParseVLESSLink(link string) (map[string]string, error) {
	if !strings.HasPrefix(link, "vless://") {
		return nil, fmt.Errorf("不是有效的VLESS链接")
	}

	link = strings.TrimPrefix(link, "vless://")

	// 分离备注
	parts := strings.SplitN(link, "#", 2)
	link = parts[0]
	remark := ""
	if len(parts) == 2 {
		remark, _ = url.QueryUnescape(parts[1])
	}

	// 分离参数
	parts = strings.SplitN(link, "?", 2)
	userAndServer := parts[0]
	var params url.Values
	if len(parts) == 2 {
		var err error
		params, err = url.ParseQuery(parts[1])
		if err != nil {
			return nil, fmt.Errorf("解析参数失败: %v", err)
		}
	}

	// 解析用户和服务器
	parts = strings.SplitN(userAndServer, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("链接格式错误")
	}

	uuid := parts[0]
	serverAndPort := parts[1]

	// 解析服务器和端口
	parts = strings.SplitN(serverAndPort, ":", 2)
	server := parts[0]
	port := "443"
	if len(parts) == 2 {
		port = parts[1]
	}

	result := map[string]string{
		"uuid":   uuid,
		"server": server,
		"port":   port,
		"remark": remark,
	}

	if params != nil {
		for key, values := range params {
			if len(values) > 0 {
				result[key] = values[0]
			}
		}
	}

	return result, nil
}
