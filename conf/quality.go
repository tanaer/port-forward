package conf

import "time"

// QualityMonitorConfig 控制线路质量监控的运行行为
type QualityMonitorConfig struct {
	Enabled                bool
	Interval               time.Duration
	ProxyIDs               []int
	TestTarget             string
	ProbeCount             int
	MaxConcurrent          int
	WarnLatencyMs          int
	WarnLossPercent        float64
	WarnConsecutiveFailure int
	RetentionDays          int
}

// DefaultQualityMonitorConfig 提供默认配置
var DefaultQualityMonitorConfig = QualityMonitorConfig{
	Enabled:                false,
	Interval:               60 * time.Second,
	TestTarget:             "rtmp.tiktok.com:1935",
	ProbeCount:             3,
	MaxConcurrent:          4,
	WarnLatencyMs:          180,
	WarnLossPercent:        3,
	WarnConsecutiveFailure: 3,
	RetentionDays:          7,
}

// QualityMonitor 保存运行期配置（可由 CLI 或配置文件覆盖）
var QualityMonitor = DefaultQualityMonitorConfig

// ProxyQualityLog 记录线路质量检测结果
type ProxyQualityLog struct {
	Id             int    `gorm:"primaryKey;autoIncrement"`
	ProxyId        int    `gorm:"index"`
	Target         string `gorm:"size:200"`
	ProbeCount     int
	SuccessCount   int
	FailureCount   int
	LatencyAvgMs   int
	JitterMs       int
	LossPercent    float64
	ThroughputMbps float64
	Status         string `gorm:"size:20"`
	Message        string `gorm:"size:500"`
	CreatedAt      time.Time
}

// TableName 指定线路质量日志的表名
func (ProxyQualityLog) TableName() string {
	return "proxy_quality_logs"
}
