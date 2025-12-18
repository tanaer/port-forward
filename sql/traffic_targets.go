package sql

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProxyTargetSample 记录每分钟访问目标
type ProxyTargetSample struct {
	ID         int       `gorm:"primaryKey"`
	ProxyID    int       `gorm:"uniqueIndex:idx_proxy_target_unique"`
	Minute     time.Time `gorm:"uniqueIndex:idx_proxy_target_unique"`
	Target     string    `gorm:"size:256;uniqueIndex:idx_proxy_target_unique"`
	Count      int
	ErrorCount int
}

func (ProxyTargetSample) TableName() string {
	return "proxy_target_samples"
}

// InitTrafficTargets 初始化目标统计表
func InitTrafficTargets() {
	db.AutoMigrate(&ProxyTargetSample{})
}

// AddTargetSample 累加目标访问统计
func AddTargetSample(proxyID int, minute time.Time, target string, success bool) {
	if proxyID == 0 || target == "" {
		return
	}
	record := ProxyTargetSample{
		ProxyID: proxyID,
		Minute:  minute.Truncate(time.Minute),
		Target:  target,
	}

	errIncrement := 0
	if !success {
		errIncrement = 1
	}

	db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "proxy_id"},
			{Name: "minute"},
			{Name: "target"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":       gorm.Expr("count + 1"),
			"error_count": gorm.Expr("error_count + ?", errIncrement),
		}),
	}).Create(&record)
}

// GetRecentTargetSamples 返回最近的目标访问记录
func GetRecentTargetSamples(proxyID int, limit int) []ProxyTargetSample {
	if limit <= 0 {
		limit = 20
	}
	var samples []ProxyTargetSample
	db.Model(&ProxyTargetSample{}).
		Where("proxy_id = ?", proxyID).
		Order("minute desc").
		Limit(limit).
		Find(&samples)
	return samples
}
