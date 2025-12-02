package quality

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"goForward/conf"
	"goForward/proxy"
	"goForward/sql"
)

// Monitor 周期性检测代理线路质量
type Monitor struct {
	cfg            conf.QualityMonitorConfig
	ticker         *time.Ticker
	stop           chan struct{}
	running        bool
	mu             sync.Mutex
	failureCounter map[int]int
}

var (
	globalMonitor *Monitor
	globalMu      sync.Mutex
)

// InitGlobalMonitor 初始化全局质量监控
func InitGlobalMonitor(cfg conf.QualityMonitorConfig) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalMonitor == nil {
		globalMonitor = NewMonitor(cfg)
	} else {
		globalMonitor.UpdateConfig(cfg)
	}
}

// StartGlobalMonitor 启动全局监控
func StartGlobalMonitor() {
	globalMu.Lock()
	monitor := globalMonitor
	globalMu.Unlock()

	if monitor != nil {
		monitor.Start()
	}
}

// StopGlobalMonitor 停止全局监控
func StopGlobalMonitor() {
	globalMu.Lock()
	monitor := globalMonitor
	globalMu.Unlock()

	if monitor != nil {
		monitor.Stop()
	}
}

// UpdateGlobalMonitorConfig 更新运行配置并根据需要重启
func UpdateGlobalMonitorConfig(cfg conf.QualityMonitorConfig) {
	globalMu.Lock()
	conf.QualityMonitor = cfg

	if globalMonitor == nil {
		globalMonitor = NewMonitor(cfg)
		globalMu.Unlock()
		if cfg.Enabled {
			globalMonitor.Start()
		}
		return
	}

	globalMonitor.Stop()
	globalMonitor.UpdateConfig(cfg)
	monitor := globalMonitor
	globalMu.Unlock()

	if cfg.Enabled {
		monitor.Start()
	}
}

// IsRunning 返回监控是否正在运行
func IsRunning() bool {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalMonitor == nil {
		return false
	}
	return globalMonitor.IsRunning()
}

// NewMonitor 创建监控器
func NewMonitor(cfg conf.QualityMonitorConfig) *Monitor {
	return &Monitor{
		cfg:            cfg,
		failureCounter: make(map[int]int),
	}
}

// UpdateConfig 更新配置
func (m *Monitor) UpdateConfig(cfg conf.QualityMonitorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	m.failureCounter = make(map[int]int)
}

// Start 启动监控
func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	if !m.cfg.Enabled {
		fmt.Println("[QualityMonitor] 未启用，跳过启动")
		return
	}

	interval := m.cfg.Interval
	if interval < 15*time.Second {
		interval = 15 * time.Second
	}

	m.ticker = time.NewTicker(interval)
	m.stop = make(chan struct{})
	m.running = true

	fmt.Printf("[QualityMonitor] 已启动，检测间隔 %s，目标 %s\n", interval, m.effectiveTarget())

	go m.loop()
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
	if m.ticker != nil {
		m.ticker.Stop()
		m.ticker = nil
	}

	m.running = false
	fmt.Println("[QualityMonitor] 已停止")
}

func (m *Monitor) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *Monitor) loop() {
	// 立即执行一次
	m.runOnce()

	for {
		select {
		case <-m.ticker.C:
			m.runOnce()
		case <-m.stop:
			return
		}
	}
}

func (m *Monitor) runOnce() {
	proxies := m.pickTargetProxies()
	if len(proxies) == 0 {
		return
	}

	maxConcurrent := m.cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for _, p := range proxies {
		proxyConfig := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			entry := m.evaluateProxy(proxyConfig)
			if err := sql.InsertProxyQualityLog(entry); err != nil {
				fmt.Printf("[QualityMonitor] 写入记录失败: %v\n", err)
			}
			sql.AddQualitySample(entry.ProxyId, entry.CreatedAt, entry.LatencyAvgMs, entry.JitterMs, entry.LossPercent, entry.Status)

			if entry.Status != "healthy" {
				fmt.Printf("[QualityMonitor] 代理ID=%d 状态=%s RTT=%dms 丢包=%.1f%% (%s)\n",
					entry.ProxyId, entry.Status, entry.LatencyAvgMs, entry.LossPercent, entry.Message)
			}
		}()
	}

	wg.Wait()

	if m.cfg.RetentionDays > 0 {
		cutoff := time.Now().Add(-time.Duration(m.cfg.RetentionDays) * 24 * time.Hour)
		sql.CleanupProxyQualityLogs(cutoff)
		sql.CleanupProxyQualitySamples(cutoff)
		sql.CleanupProxyTrafficSamples(cutoff)
		sql.CleanupProxyTrafficHourly(cutoff)
		sql.CleanupProxyTrafficDaily(cutoff)
	}
}

func (m *Monitor) pickTargetProxies() []conf.ProxyConfig {
	if len(m.cfg.ProxyIDs) == 0 {
		return sql.GetActiveProxies()
	}

	var res []conf.ProxyConfig
	for _, id := range m.cfg.ProxyIDs {
		proxyConfig := sql.GetProxy(id)
		if proxyConfig.Id == 0 {
			continue
		}
		// 仅检测启用中的线路
		if proxyConfig.Status == 0 {
			res = append(res, proxyConfig)
		}
	}
	return res
}

func (m *Monitor) evaluateProxy(proxyConfig conf.ProxyConfig) conf.ProxyQualityLog {
	probeCount := m.cfg.ProbeCount
	if probeCount <= 0 {
		probeCount = 1
	}

	target := m.effectiveTarget()
	var latencies []float64
	var lastErr string
	success := 0

	for i := 0; i < probeCount; i++ {
		result := m.runSingleProbe(proxyConfig, target)
		if result == nil {
			lastErr = "探测失败：无结果"
			continue
		}

		if result.Success {
			success++
			latencies = append(latencies, float64(result.Latency.Milliseconds()))
		} else {
			lastErr = result.Message
		}

		time.Sleep(200 * time.Millisecond)
	}

	failures := probeCount - success
	loss := 0.0
	if probeCount > 0 {
		loss = (float64(failures) / float64(probeCount)) * 100
	}

	avgLatency := average(latencies)
	jitter := jitter(latencies)
	status := m.deriveStatus(proxyConfig.Id, success, probeCount, avgLatency, loss)

	message := fmt.Sprintf("成功 %d/%d，平均延迟 %.0fms", success, probeCount, avgLatency)
	if success == 0 && lastErr != "" {
		message = lastErr
	} else if lastErr != "" {
		message = fmt.Sprintf("%s；最近错误: %s", message, lastErr)
	}

	sampleTime := time.Now()
	return conf.ProxyQualityLog{
		ProxyId:      proxyConfig.Id,
		Target:       target,
		ProbeCount:   probeCount,
		SuccessCount: success,
		FailureCount: failures,
		LatencyAvgMs: int(math.Round(avgLatency)),
		JitterMs:     int(math.Round(jitter)),
		LossPercent:  math.Round(loss*10) / 10,
		Status:       status,
		Message:      truncateMessage(message),
		CreatedAt:    sampleTime,
	}
}

func (m *Monitor) deriveStatus(proxyID int, success, total int, avgLatency, loss float64) string {
	status := "healthy"

	if success == 0 {
		status = "critical"
	} else if avgLatency > float64(m.cfg.WarnLatencyMs) || loss > m.cfg.WarnLossPercent {
		status = "warning"
	}

	failuresOnly := success == 0
	counter := m.updateFailureCounter(proxyID, failuresOnly)
	if counter >= m.cfg.WarnConsecutiveFailure && failuresOnly {
		status = "critical"
	}

	if !failuresOnly && counter > 0 {
		m.updateFailureCounter(proxyID, false)
	}

	return status
}

func (m *Monitor) updateFailureCounter(proxyID int, failed bool) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !failed {
		m.failureCounter[proxyID] = 0
		return 0
	}

	m.failureCounter[proxyID]++
	return m.failureCounter[proxyID]
}

func (m *Monitor) runSingleProbe(proxyConfig conf.ProxyConfig, target string) *proxy.TestResult {
	switch strings.ToLower(proxyConfig.OutboundType) {
	case "socks5":
		// 通过SOCKS5代理测试到目标地址的连通性（完整链路）
		return proxy.TestSOCKS5WithTarget(
			proxyConfig.Socks5Addr,
			proxyConfig.Socks5Port,
			proxyConfig.Socks5User,
			proxyConfig.Socks5Password,
			target,
		)
	case "vmess":
		// VMess暂时只测试TCP连接（完整链路测试需要实现VMess协议）
		return proxy.TestVMessConnection(proxyConfig.VmessServer, proxyConfig.VmessPort)
	default:
		// Hysteria2：通过本地SOCKS5端口测试到目标地址的连通性
		port := proxyConfig.Hy2Socks5Port
		if port == 0 {
			port = 10808
		}
		return proxy.TestHysteria2ConnectionWithTarget("127.0.0.1", port, target)
	}
}

func (m *Monitor) effectiveTarget() string {
	target := m.cfg.TestTarget
	if strings.TrimSpace(target) == "" {
		return conf.DefaultQualityMonitorConfig.TestTarget
	}
	return target
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func jitter(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sum := 0.0
	count := 0.0
	for i := 1; i < len(values); i++ {
		sum += math.Abs(values[i] - values[i-1])
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / count
}

func truncateMessage(msg string) string {
	if len(msg) <= 480 {
		return msg
	}
	return msg[:480]
}
