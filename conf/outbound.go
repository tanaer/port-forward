package conf

import (
	"time"
)

// OutboundConfig 出站配置（独立管理）
type OutboundConfig struct {
	Id        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`           // 出站配置名称
	Type      string    `gorm:"size:20;default:'hysteria2'" json:"type"` // "hysteria2", "socks5", "vmess"
	Status    int       `gorm:"default:1" json:"status"`                 // 0=启用, 1=停用（只能有一个启用）
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`

	// ========== Hysteria2 配置 ==========
	Hy2Server       string `gorm:"size:100" json:"hy2Server"`
	Hy2Port         string `gorm:"size:10" json:"hy2Port"`
	Hy2Password     string `gorm:"size:200" json:"hy2Password"`
	Hy2Subscription string `gorm:"size:500" json:"hy2Subscription"`
	Hy2Obfs         string `gorm:"size:50" json:"hy2Obfs"`
	Hy2ObfsPassword string `gorm:"size:200" json:"hy2ObfsPassword"`
	Hy2SNI          string `gorm:"size:100" json:"hy2SNI"`
	Hy2Insecure     bool   `gorm:"default:false" json:"hy2Insecure"`
	Hy2UpMbps       int    `gorm:"default:100" json:"hy2UpMbps"`
	Hy2DownMbps     int    `gorm:"default:100" json:"hy2DownMbps"`

	// ========== SOCKS5 配置 ==========
	Socks5Addr     string `gorm:"size:100" json:"socks5Addr"`
	Socks5Port     int    `gorm:"default:1080" json:"socks5Port"`
	Socks5User     string `gorm:"size:100" json:"socks5User"`
	Socks5Password string `gorm:"size:200" json:"socks5Password"`

	// ========== VMess 配置 ==========
	VmessServer       string `gorm:"size:100" json:"vmessServer"`
	VmessPort         int    `gorm:"default:443" json:"vmessPort"`
	VmessUUID         string `gorm:"size:36" json:"vmessUUID"`
	VmessAlterID      int    `gorm:"default:0" json:"vmessAlterID"`
	VmessSecurity     string `gorm:"size:50" json:"vmessSecurity"`
	VmessNetwork      string `gorm:"size:20" json:"vmessNetwork"`
	VmessTLS          bool   `gorm:"default:false" json:"vmessTLS"`
	VmessServerName   string `gorm:"size:100" json:"vmessServerName"`
	VmessWsPath       string `gorm:"size:200" json:"vmessWsPath"`
	VmessWsHost       string `gorm:"size:100" json:"vmessWsHost"`
	VmessSubscription string `gorm:"size:500" json:"vmessSubscription"`
}

// TableName 指定表名
func (OutboundConfig) TableName() string {
	return "outbound_configs"
}

// GetDisplayName 获取显示名称（带类型标识）
func (o *OutboundConfig) GetDisplayName() string {
	typeLabel := ""
	switch o.Type {
	case "hysteria2":
		typeLabel = "HY2"
	case "socks5":
		typeLabel = "SOCKS5"
	case "vmess":
		typeLabel = "VMess"
	}
	return "[" + typeLabel + "] " + o.Name
}

// GetServerInfo 获取服务器信息摘要
func (o *OutboundConfig) GetServerInfo() string {
	switch o.Type {
	case "hysteria2":
		return o.Hy2Server + ":" + o.Hy2Port
	case "socks5":
		if o.Socks5Port > 0 {
			return o.Socks5Addr + ":" + string(rune(o.Socks5Port))
		}
		return o.Socks5Addr
	case "vmess":
		return o.VmessServer
	}
	return ""
}
