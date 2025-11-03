package metrics

import (
	"sync"
	"time"
)

// Metrics 性能监控指标
type Metrics struct {
	mu sync.RWMutex

	// 流量统计
	TotalBytesSent     uint64
	TotalBytesReceived uint64
	TotalPacketsSent   uint64
	TotalPacketsRecv   uint64

	// 连接统计
	ActiveConnections int
	TotalConnections  uint64
	FailedConnections uint64

	// 端口检查统计
	PortChecksTotal   uint64
	PortChecksCached  uint64
	CacheHitRate      float64

	// 性能统计
	DBWriteDuration    time.Duration
	PortCheckDuration time.Duration

	// 时间戳
	LastUpdate time.Time
}

// GlobalMetrics 全局指标实例
var GlobalMetrics *Metrics

// InitMetrics 初始化全局指标
func InitMetrics() {
	GlobalMetrics = &Metrics{
		LastUpdate: time.Now(),
	}
}

// UpdateBytes 更新字节数统计
func (m *Metrics) UpdateBytes(sent, recv uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalBytesSent += sent
	m.TotalBytesReceived += recv
	m.LastUpdate = time.Now()
}

// UpdateConnections 更新连接统计
func (m *Metrics) UpdateConnections(active int, total uint64, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ActiveConnections = active
	if !failed {
		m.TotalConnections++
	} else {
		m.FailedConnections++
	}
	m.LastUpdate = time.Now()
}

// UpdatePortChecks 更新端口检查统计
func (m *Metrics) UpdatePortChecks(cached bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PortChecksTotal++
	if cached {
		m.PortChecksCached++
		// 计算缓存命中率
		if m.PortChecksTotal > 0 {
			m.CacheHitRate = float64(m.PortChecksCached) / float64(m.PortChecksTotal) * 100
		}
	}
	m.PortCheckDuration = duration
	m.LastUpdate = time.Now()
}

// GetMetrics 获取当前指标
func (m *Metrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"traffic": map[string]interface{}{
			"bytes_sent":     m.TotalBytesSent,
			"bytes_recv":     m.TotalBytesReceived,
			"packets_sent":   m.TotalPacketsSent,
			"packets_recv":   m.TotalPacketsRecv,
		},
		"connections": map[string]interface{}{
			"active":    m.ActiveConnections,
			"total":     m.TotalConnections,
			"failed":    m.FailedConnections,
			"success_rate": float64(m.TotalConnections) / float64(m.TotalConnections+m.FailedConnections) * 100,
		},
		"performance": map[string]interface{}{
			"cache_hit_rate":    m.CacheHitRate,
			"port_check_avg_ms": float64(m.PortCheckDuration.Nanoseconds()) / 1000000,
			"db_write_avg_ms":   float64(m.DBWriteDuration.Nanoseconds()) / 1000000,
		},
		"last_update": m.LastUpdate.Format("2006-01-02 15:04:05"),
	}
}
