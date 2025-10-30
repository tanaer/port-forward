package conf

import (
	"time"
)

// ProxyConfig 代理配置
type ProxyConfig struct {
	Id        int       `gorm:"primaryKey;autoIncrement"`
	Name      string    `gorm:"size:100"`
	Status    int       `gorm:"default:0"` // 0=启用, 1=停用
	Remark    string    `gorm:"size:500"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// VLESS+Reality 入站配置
	InboundPort       int    `gorm:"default:443"`
	UUID              string `gorm:"size:36"`
	Flow              string `gorm:"size:50;default:'xtls-rprx-vision'"`
	RealityDest       string `gorm:"size:100"` // 回落域名
	RealityServerName string `gorm:"size:100"` // SNI
	PrivateKey        string `gorm:"size:100"` // Reality 私钥
	PublicKey         string `gorm:"size:100"` // Reality 公钥
	ShortId           string `gorm:"size:20"`  // Reality Short ID

	// 出站配置
	OutboundType string `gorm:"size:20;default:'hysteria2'"` // "hysteria2" 或 "socks5"

	// Hysteria2 出站配置
	Hy2Server       string `gorm:"size:100"` // Hysteria2 服务器地址
	Hy2Port         string `gorm:"size:10"`  // Hysteria2 端口
	Hy2Password     string `gorm:"size:200"` // Hysteria2 密码
	Hy2Subscription string `gorm:"size:500"` // Hysteria2 订阅链接
	Hy2Obfs         string `gorm:"size:50"`  // 混淆类型
	Hy2ObfsPassword string `gorm:"size:200"` // 混淆密码
	Hy2SNI          string `gorm:"size:100"` // SNI
	Hy2Insecure     bool   `gorm:"default:false"`
	Hy2UpMbps       int    `gorm:"default:100"`   // 上行带宽
	Hy2DownMbps     int    `gorm:"default:100"`   // 下行带宽
	Hy2Socks5Port   int    `gorm:"default:10808"` // SOCKS5端口

	// SOCKS5 出站配置
	Socks5Addr     string `gorm:"size:100"`     // SOCKS5 服务器地址
	Socks5Port     int    `gorm:"default:1080"` // SOCKS5 端口
	Socks5User     string `gorm:"size:100"`     // SOCKS5 用户名（可选）
	Socks5Password string `gorm:"size:200"`     // SOCKS5 密码（可选）

	// VMess 出站配置
	VmessServer       string `gorm:"size:100"`            // VMess 服务器地址
	VmessPort         int    `gorm:"default:443"`         // VMess 端口
	VmessUUID         string `gorm:"size:36"`             // VMess UUID
	VmessAlterID      int    `gorm:"default:0"`           // VMess alterID (通常为0)
	VmessSecurity     string `gorm:"size:50"`             // 加密方式: auto, aes-128-gcm, chacha20-poly1305, none
	VmessNetwork      string `gorm:"size:20"`             // 传输协议: tcp, ws, h2, grpc
	VmessTLS          bool   `gorm:"default:false"`       // 是否启用 TLS
	VmessServerName   string `gorm:"size:100"`            // TLS SNI
	VmessWsPath       string `gorm:"size:200"`            // WebSocket 路径
	VmessWsHost       string `gorm:"size:100"`            // WebSocket Host
	VmessSubscription string `gorm:"size:500"`            // VMess 订阅链接

	// 流量统计
	TotalBytes    uint64 `gorm:"default:0"`
	TotalGigabyte uint64 `gorm:"default:0"`
}

// Subscription 订阅配置
type Subscription struct {
	Id           int       `gorm:"primaryKey;autoIncrement"`
	ProxyId      int       `gorm:"index"`
	AccessToken  string    `gorm:"size:50;uniqueIndex"`
	ClientCount  int       `gorm:"default:0"`
	TotalTraffic uint64    `gorm:"default:0"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ProxyConfig) TableName() string {
	return "proxy_configs"
}

// TableName 指定表名
func (Subscription) TableName() string {
	return "subscriptions"
}
