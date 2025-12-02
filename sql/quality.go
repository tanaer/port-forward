package sql

import (
	"strings"
	"time"

	"goForward/conf"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// qualityMonitorSettingDB 对应数据库中的质量监控配置
type qualityMonitorSettingDB struct {
	ID                     int     `gorm:"primaryKey"`
	Enabled                bool    `gorm:"default:false"`
	ProxySpec              string  `gorm:"size:200"`
	Target                 string  `gorm:"size:200"`
	IntervalSeconds        int     `gorm:"default:60"`
	ProbeCount             int     `gorm:"default:3"`
	WarnLatencyMs          int     `gorm:"default:180"`
	WarnLossPercent        float64 `gorm:"default:3"`
	WarnConsecutiveFailure int     `gorm:"default:3"`
	RetentionDays          int     `gorm:"default:7"`
	MaxConcurrent          int     `gorm:"default:4"`
	UpdatedAt              time.Time
}

func (qualityMonitorSettingDB) TableName() string {
	return "quality_monitor_settings"
}

// InitQualityMonitorSettings 初始化表
func InitQualityMonitorSettings() {
	db.AutoMigrate(&qualityMonitorSettingDB{})
	db.AutoMigrate(&ProxyQualitySample{})
	db.AutoMigrate(&ProxyTrafficSample{})
	db.AutoMigrate(&ProxyTrafficHourly{})
	db.AutoMigrate(&ProxyTrafficDaily{})
	db.AutoMigrate(&ProxyQualityHourly{})
	db.AutoMigrate(&ProxyQualityDaily{})
}

// ProxyQualitySample 每分钟延迟样本
type ProxyQualitySample struct {
	ID        int       `gorm:"primaryKey"`
	ProxyID   int       `gorm:"uniqueIndex:idx_proxy_quality_unique"`
	Minute    time.Time `gorm:"uniqueIndex:idx_proxy_quality_unique"`
	LatencyMs int
	JitterMs  int
	LossPct   float64
	Status    string `gorm:"size:20"`
}

// ProxyTrafficSample 每分钟流量样本
type ProxyTrafficSample struct {
	ID        int       `gorm:"primaryKey"`
	ProxyID   int       `gorm:"uniqueIndex:idx_proxy_traffic_minute"`
	Minute    time.Time `gorm:"uniqueIndex:idx_proxy_traffic_minute"`
	BytesUp   uint64
	BytesDown uint64
}

// ProxyTrafficHourly 每小时聚合样本
type ProxyTrafficHourly struct {
	ID          int       `gorm:"primaryKey"`
	ProxyID     int       `gorm:"uniqueIndex:idx_proxy_traffic_hourly"`
	HourStart   time.Time `gorm:"uniqueIndex:idx_proxy_traffic_hourly"`
	BytesUp     uint64
	BytesDown   uint64
	SampleCount int
	UpdatedAt   time.Time
}

func (ProxyTrafficHourly) TableName() string {
	return "proxy_traffic_hourly"
}

// ProxyTrafficDaily 每天聚合样本
type ProxyTrafficDaily struct {
	ID          int       `gorm:"primaryKey"`
	ProxyID     int       `gorm:"uniqueIndex:idx_proxy_traffic_daily"`
	DayStart    time.Time `gorm:"uniqueIndex:idx_proxy_traffic_daily"`
	BytesUp     uint64
	BytesDown   uint64
	SampleCount int
	UpdatedAt   time.Time
}

func (ProxyTrafficDaily) TableName() string {
	return "proxy_traffic_daily"
}

// ProxyQualityHourly 每小时聚合的延迟样本
type ProxyQualityHourly struct {
	ID          int       `gorm:"primaryKey"`
	ProxyID     int       `gorm:"uniqueIndex:idx_proxy_quality_hourly"`
	HourStart   time.Time `gorm:"uniqueIndex:idx_proxy_quality_hourly"`
	LatencyMs   int       // 平均延迟
	JitterMs    int       // 平均抖动
	LossPct     float64   // 平均丢包率
	SampleCount int
	UpdatedAt   time.Time
}

func (ProxyQualityHourly) TableName() string {
	return "proxy_quality_hourly"
}

// ProxyQualityDaily 每天聚合的延迟样本
type ProxyQualityDaily struct {
	ID          int       `gorm:"primaryKey"`
	ProxyID     int       `gorm:"uniqueIndex:idx_proxy_quality_daily"`
	DayStart    time.Time `gorm:"uniqueIndex:idx_proxy_quality_daily"`
	LatencyMs   int       // 平均延迟
	JitterMs    int       // 平均抖动
	LossPct     float64   // 平均丢包率
	SampleCount int
	UpdatedAt   time.Time
}

func (ProxyQualityDaily) TableName() string {
	return "proxy_quality_daily"
}

// SaveQualityMonitorSetting 持久化配置
func SaveQualityMonitorSetting(cfg conf.QualityMonitorConfig) error {
	setting := qualityMonitorSettingDB{
		ID:                     1,
		Enabled:                cfg.Enabled,
		ProxySpec:              conf.FormatProxyIDs(cfg.ProxyIDs),
		Target:                 cfg.TestTarget,
		IntervalSeconds:        int(cfg.Interval / time.Second),
		ProbeCount:             cfg.ProbeCount,
		WarnLatencyMs:          cfg.WarnLatencyMs,
		WarnLossPercent:        cfg.WarnLossPercent,
		WarnConsecutiveFailure: cfg.WarnConsecutiveFailure,
		RetentionDays:          cfg.RetentionDays,
		MaxConcurrent:          cfg.MaxConcurrent,
		UpdatedAt:              time.Now(),
	}

	if setting.IntervalSeconds <= 0 {
		setting.IntervalSeconds = int(conf.DefaultQualityMonitorConfig.Interval / time.Second)
	}
	if setting.ProbeCount <= 0 {
		setting.ProbeCount = conf.DefaultQualityMonitorConfig.ProbeCount
	}
	if setting.MaxConcurrent <= 0 {
		setting.MaxConcurrent = conf.DefaultQualityMonitorConfig.MaxConcurrent
	}
	if setting.RetentionDays < 0 {
		setting.RetentionDays = conf.DefaultQualityMonitorConfig.RetentionDays
	}
	if setting.Target == "" {
		setting.Target = conf.DefaultQualityMonitorConfig.TestTarget
	}

	var existing qualityMonitorSettingDB
	result := db.First(&existing, 1)
	if result.Error != nil || result.RowsAffected == 0 {
		return db.Save(&setting).Error
	}

	return db.Model(&qualityMonitorSettingDB{}).Where("id = ?", 1).Updates(setting).Error
}

// GetQualityMonitorSetting 获取当前配置
func GetQualityMonitorSetting() (conf.QualityMonitorConfig, bool) {
	var setting qualityMonitorSettingDB
	result := db.First(&setting, 1)
	if result.Error != nil || result.RowsAffected == 0 {
		return conf.QualityMonitorConfig{}, false
	}

	return qualitySettingToConfig(setting), true
}

// EnsureQualityMonitorSetting 返回现有配置，没有则创建默认值
func EnsureQualityMonitorSetting(defaultCfg conf.QualityMonitorConfig) conf.QualityMonitorConfig {
	if cfg, ok := GetQualityMonitorSetting(); ok {
		return cfg
	}
	_ = SaveQualityMonitorSetting(defaultCfg)
	return defaultCfg
}

func qualitySettingToConfig(s qualityMonitorSettingDB) conf.QualityMonitorConfig {
	cfg := conf.QualityMonitorConfig{
		Enabled:                s.Enabled,
		ProxyIDs:               conf.ParseProxyIDSpec(s.ProxySpec),
		TestTarget:             s.Target,
		ProbeCount:             s.ProbeCount,
		MaxConcurrent:          s.MaxConcurrent,
		WarnLatencyMs:          s.WarnLatencyMs,
		WarnLossPercent:        s.WarnLossPercent,
		WarnConsecutiveFailure: s.WarnConsecutiveFailure,
		RetentionDays:          s.RetentionDays,
	}

	if s.IntervalSeconds <= 0 {
		cfg.Interval = conf.DefaultQualityMonitorConfig.Interval
	} else {
		cfg.Interval = time.Duration(s.IntervalSeconds) * time.Second
	}
	if cfg.ProbeCount <= 0 {
		cfg.ProbeCount = conf.DefaultQualityMonitorConfig.ProbeCount
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = conf.DefaultQualityMonitorConfig.MaxConcurrent
	}
	if cfg.RetentionDays < 0 {
		cfg.RetentionDays = conf.DefaultQualityMonitorConfig.RetentionDays
	}
	if cfg.TestTarget == "" {
		cfg.TestTarget = conf.DefaultQualityMonitorConfig.TestTarget
	}

	return cfg
}

// AddQualitySample 写入或更新每分钟延迟样本
func AddQualitySample(proxyID int, ts time.Time, latency, jitter int, lossPct float64, status string) {
	minute := ts.Truncate(time.Minute)
	sample := ProxyQualitySample{
		ProxyID:   proxyID,
		Minute:    minute,
		LatencyMs: latency,
		JitterMs:  jitter,
		LossPct:   lossPct,
		Status:    status,
	}

	db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "proxy_id"}, {Name: "minute"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"latency_ms": latency,
			"jitter_ms":  jitter,
			"loss_pct":   lossPct,
			"status":     status,
		}),
	}).Create(&sample)

	// 同时更新小时级和天级聚合
	addQualityAggregates(proxyID, minute, latency, jitter, lossPct)
}

func addQualityAggregates(proxyID int, ts time.Time, latency, jitter int, lossPct float64) {
	updateQualityHourly(proxyID, ts, latency, jitter, lossPct)
	updateQualityDaily(proxyID, ts, latency, jitter, lossPct)
}

func updateQualityHourly(proxyID int, ts time.Time, latency, jitter int, lossPct float64) {
	now := time.Now()
	hourStart := ts.Truncate(time.Hour)

	// 使用加权平均更新：new_avg = (old_avg * old_count + new_value) / (old_count + 1)
	entry := ProxyQualityHourly{
		ProxyID:     proxyID,
		HourStart:   hourStart,
		LatencyMs:   latency,
		JitterMs:    jitter,
		LossPct:     lossPct,
		SampleCount: 1,
		UpdatedAt:   now,
	}

	db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "proxy_id"}, {Name: "hour_start"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"latency_ms":   gorm.Expr("(latency_ms * sample_count + ?) / (sample_count + 1)", latency),
			"jitter_ms":    gorm.Expr("(jitter_ms * sample_count + ?) / (sample_count + 1)", jitter),
			"loss_pct":     gorm.Expr("(loss_pct * sample_count + ?) / (sample_count + 1)", lossPct),
			"sample_count": gorm.Expr("sample_count + 1"),
			"updated_at":   now,
		}),
	}).Create(&entry)
}

func updateQualityDaily(proxyID int, ts time.Time, latency, jitter int, lossPct float64) {
	now := time.Now()
	dayStart := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location())

	entry := ProxyQualityDaily{
		ProxyID:     proxyID,
		DayStart:    dayStart,
		LatencyMs:   latency,
		JitterMs:    jitter,
		LossPct:     lossPct,
		SampleCount: 1,
		UpdatedAt:   now,
	}

	db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "proxy_id"}, {Name: "day_start"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"latency_ms":   gorm.Expr("(latency_ms * sample_count + ?) / (sample_count + 1)", latency),
			"jitter_ms":    gorm.Expr("(jitter_ms * sample_count + ?) / (sample_count + 1)", jitter),
			"loss_pct":     gorm.Expr("(loss_pct * sample_count + ?) / (sample_count + 1)", lossPct),
			"sample_count": gorm.Expr("sample_count + 1"),
			"updated_at":   now,
		}),
	}).Create(&entry)
}

// AddTrafficSample 写入每分钟流量样本（累加）
func AddTrafficSample(proxyID int, ts time.Time, bytesUp, bytesDown uint64) {
	if bytesUp == 0 && bytesDown == 0 {
		return
	}
	minute := ts.Truncate(time.Minute)
	sample := ProxyTrafficSample{
		ProxyID:   proxyID,
		Minute:    minute,
		BytesUp:   bytesUp,
		BytesDown: bytesDown,
	}

	db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "proxy_id"}, {Name: "minute"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"bytes_up":   gorm.Expr("bytes_up + ?", bytesUp),
			"bytes_down": gorm.Expr("bytes_down + ?", bytesDown),
		}),
	}).Create(&sample)

	addTrafficAggregates(proxyID, minute, bytesUp, bytesDown)
}

func addTrafficAggregates(proxyID int, ts time.Time, bytesUp, bytesDown uint64) {
	if bytesUp == 0 && bytesDown == 0 {
		return
	}
	updateTrafficHourly(proxyID, ts, bytesUp, bytesDown)
	updateTrafficDaily(proxyID, ts, bytesUp, bytesDown)
}

func updateTrafficHourly(proxyID int, ts time.Time, bytesUp, bytesDown uint64) {
	now := time.Now()
	hourStart := ts.Truncate(time.Hour)
	entry := ProxyTrafficHourly{
		ProxyID:     proxyID,
		HourStart:   hourStart,
		BytesUp:     bytesUp,
		BytesDown:   bytesDown,
		SampleCount: 1,
		UpdatedAt:   now,
	}

	db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "proxy_id"}, {Name: "hour_start"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"bytes_up":     gorm.Expr("bytes_up + ?", bytesUp),
			"bytes_down":   gorm.Expr("bytes_down + ?", bytesDown),
			"sample_count": gorm.Expr("sample_count + 1"),
			"updated_at":   now,
		}),
	}).Create(&entry)
}

func updateTrafficDaily(proxyID int, ts time.Time, bytesUp, bytesDown uint64) {
	now := time.Now()
	dayStart := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location())
	entry := ProxyTrafficDaily{
		ProxyID:     proxyID,
		DayStart:    dayStart,
		BytesUp:     bytesUp,
		BytesDown:   bytesDown,
		SampleCount: 1,
		UpdatedAt:   now,
	}

	db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "proxy_id"}, {Name: "day_start"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"bytes_up":     gorm.Expr("bytes_up + ?", bytesUp),
			"bytes_down":   gorm.Expr("bytes_down + ?", bytesDown),
			"sample_count": gorm.Expr("sample_count + 1"),
			"updated_at":   now,
		}),
	}).Create(&entry)
}

// CleanupProxyQualitySamples 清理过期的延迟样本
func CleanupProxyQualitySamples(before time.Time) {
	db.Where("minute < ?", before).Delete(&ProxyQualitySample{})
}

// CleanupProxyTrafficSamples 清理过期的流量样本
func CleanupProxyTrafficSamples(before time.Time) {
	db.Where("minute < ?", before).Delete(&ProxyTrafficSample{})
}

// CleanupProxyTrafficHourly 清理过期的小时聚合数据
func CleanupProxyTrafficHourly(before time.Time) {
	db.Where("hour_start < ?", before.Truncate(time.Hour)).Delete(&ProxyTrafficHourly{})
}

// CleanupProxyTrafficDaily 清理过期的天聚合数据
func CleanupProxyTrafficDaily(before time.Time) {
	cutoff := time.Date(before.Year(), before.Month(), before.Day(), 0, 0, 0, 0, before.Location())
	db.Where("day_start < ?", cutoff).Delete(&ProxyTrafficDaily{})
}

// GetQualitySamples 获取指定代理的质量样本
func GetQualitySamples(proxyID int, start, end time.Time, limit int) []ProxyQualitySample {
	var samples []ProxyQualitySample
	query := db.Model(&ProxyQualitySample{}).Where("proxy_id = ?", proxyID)
	if !start.IsZero() {
		query = query.Where("minute >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("minute <= ?", end)
	}
	if limit > 0 {
		query = query.Order("minute desc").Limit(limit)
	} else {
		query = query.Order("minute desc")
	}
	query.Find(&samples)
	// reverse to chronological order
	for i, j := 0, len(samples)-1; i < j; i, j = i+1, j-1 {
		samples[i], samples[j] = samples[j], samples[i]
	}
	return samples
}

// GetTrafficSamples 获取指定代理的流量样本
func GetTrafficSamples(proxyID int, start, end time.Time, limit int) []ProxyTrafficSample {
	var samples []ProxyTrafficSample
	query := db.Model(&ProxyTrafficSample{}).Where("proxy_id = ?", proxyID)
	if !start.IsZero() {
		query = query.Where("minute >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("minute <= ?", end)
	}
	if limit > 0 {
		query = query.Order("minute desc").Limit(limit)
	} else {
		query = query.Order("minute desc")
	}
	query.Find(&samples)
	for i, j := 0, len(samples)-1; i < j; i, j = i+1, j-1 {
		samples[i], samples[j] = samples[j], samples[i]
	}
	return samples
}

// GetTrafficSamplesWithResolution 根据聚合级别返回样本（minute/hour/day）
func GetTrafficSamplesWithResolution(proxyID int, resolution string, start, end time.Time, limit int) []ProxyTrafficSample {
	switch strings.ToLower(resolution) {
	case "hour", "hourly":
		return getHourlyTrafficSamples(proxyID, start, end, limit)
	case "day", "daily":
		return getDailyTrafficSamples(proxyID, start, end, limit)
	default:
		return GetTrafficSamples(proxyID, start, end, limit)
	}
}

func getHourlyTrafficSamples(proxyID int, start, end time.Time, limit int) []ProxyTrafficSample {
	var rows []ProxyTrafficHourly
	query := db.Model(&ProxyTrafficHourly{}).Where("proxy_id = ?", proxyID)
	if !start.IsZero() {
		query = query.Where("hour_start >= ?", start.Truncate(time.Hour))
	}
	if !end.IsZero() {
		query = query.Where("hour_start <= ?", end.Truncate(time.Hour))
	}
	if limit > 0 {
		query = query.Order("hour_start desc").Limit(limit)
	} else {
		query = query.Order("hour_start desc")
	}
	query.Find(&rows)
	if len(rows) == 0 {
		return nil
	}
	samples := make([]ProxyTrafficSample, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		samples = append(samples, ProxyTrafficSample{
			ProxyID:   row.ProxyID,
			Minute:    row.HourStart,
			BytesUp:   row.BytesUp,
			BytesDown: row.BytesDown,
		})
	}
	return samples
}

func getDailyTrafficSamples(proxyID int, start, end time.Time, limit int) []ProxyTrafficSample {
	var rows []ProxyTrafficDaily
	query := db.Model(&ProxyTrafficDaily{}).Where("proxy_id = ?", proxyID)
	if !start.IsZero() {
		cutoff := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		query = query.Where("day_start >= ?", cutoff)
	}
	if !end.IsZero() {
		cutoff := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
		query = query.Where("day_start <= ?", cutoff)
	}
	if limit > 0 {
		query = query.Order("day_start desc").Limit(limit)
	} else {
		query = query.Order("day_start desc")
	}
	query.Find(&rows)
	if len(rows) == 0 {
		return nil
	}
	samples := make([]ProxyTrafficSample, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		samples = append(samples, ProxyTrafficSample{
			ProxyID:   row.ProxyID,
			Minute:    row.DayStart,
			BytesUp:   row.BytesUp,
			BytesDown: row.BytesDown,
		})
	}
	return samples
}
