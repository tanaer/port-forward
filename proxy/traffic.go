package proxy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"goForward/conf"
	"goForward/sql"
)

// TrafficStats 流量统计结构
type TrafficStats struct {
	BytesUp   uint64
	BytesDown uint64
}

// TrafficMonitor 流量监控器
type TrafficMonitor struct {
	lastPositions map[string]int64 // 记录每个日志文件的最后读取位置
	ticker        *time.Ticker
	stopChan      chan bool
}

// NewTrafficMonitor 创建流量监控器
func NewTrafficMonitor() *TrafficMonitor {
	return &TrafficMonitor{
		lastPositions: make(map[string]int64),
		stopChan:      make(chan bool),
	}
}

// Start 启动流量监控
func (tm *TrafficMonitor) Start() {
	tm.ticker = time.NewTicker(30 * time.Second) // 每30秒检查一次
	go func() {
		for {
			select {
			case <-tm.ticker.C:
				tm.collectTrafficStats()
			case <-tm.stopChan:
				return
			}
		}
	}()
	fmt.Println("[TrafficMonitor] 流量监控已启动")
}

// Stop 停止流量监控
func (tm *TrafficMonitor) Stop() {
	if tm.ticker != nil {
		tm.ticker.Stop()
	}
	// 安全关闭通道，避免重复关闭
	select {
	case <-tm.stopChan:
		// 通道已关闭
	default:
		close(tm.stopChan)
	}
	fmt.Println("[TrafficMonitor] 流量监控已停止")
}

// collectTrafficStats 收集所有代理的流量统计
func (tm *TrafficMonitor) collectTrafficStats() {
	// 获取所有活动的代理
	activeProxies := sql.GetActiveProxies()

	for _, proxy := range activeProxies {
		tm.updateProxyTraffic(&proxy)
	}
}

// updateProxyTraffic 更新单个代理的流量统计
func (tm *TrafficMonitor) updateProxyTraffic(proxy *conf.ProxyConfig) {
	var totalUp, totalDown uint64

	// 根据代理类型获取流量
	switch proxy.OutboundType {
	case "hysteria2":
		up, down := tm.parseHysteria2Traffic(proxy.Id)
		totalUp += up
		totalDown += down
	case "socks5":
		// SOCKS5代理的流量统计需要额外实现
		up, down := tm.parseSocks5Traffic(proxy.Id)
		totalUp += up
		totalDown += down
	case "vmess":
		up, down := tm.parseVMessTraffic(proxy.Id)
		totalUp += up
		totalDown += down
	}

	// 如果有流量变化，更新数据库
	if totalUp > 0 || totalDown > 0 {
		// 更新字节数
		sql.UpdateProxyTraffic(proxy.Id, totalUp+totalDown)

		// 计算并更新GB数
		totalBytes := totalUp + totalDown
		if totalBytes >= 1073741824 { // 1GB
			gb := totalBytes / 1073741824
			sql.UpdateProxyTrafficGB(proxy.Id, gb)
		}
	}
}

// parseHysteria2Traffic 解析Hysteria2流量统计
func (tm *TrafficMonitor) parseHysteria2Traffic(proxyID int) (uint64, uint64) {
	execPath, _ := os.Executable()
	logDir := filepath.Join(filepath.Dir(execPath), "proxy_configs", fmt.Sprintf("logs_%d", proxyID))
	hysteriaLog := filepath.Join(logDir, "hysteria2.log")

	up, down, err := tm.parseTrafficFromLog(hysteriaLog, `(\d+)\s+bytes.*(up|down)`)
	if err != nil {
		return 0, 0
	}

	return up, down
}

// parseSocks5Traffic 解析SOCKS5流量统计
func (tm *TrafficMonitor) parseSocks5Traffic(proxyID int) (uint64, uint64) {
	execPath, _ := os.Executable()
	logDir := filepath.Join(filepath.Dir(execPath), "proxy_configs", fmt.Sprintf("logs_%d", proxyID))
	socks5Log := filepath.Join(logDir, "socks5.log")

	up, down, err := tm.parseTrafficFromLog(socks5Log, `up:\s*(\d+)\s*bytes.*down:\s*(\d+)\s*bytes`)
	if err != nil {
		return 0, 0
	}

	return up, down
}

// parseVMessTraffic 解析VMess流量统计
func (tm *TrafficMonitor) parseVMessTraffic(proxyID int) (uint64, uint64) {
	execPath, _ := os.Executable()
	logDir := filepath.Join(filepath.Dir(execPath), "proxy_configs", fmt.Sprintf("logs_%d", proxyID))
	vmessLog := filepath.Join(logDir, "vmess.log")

	up, down, err := tm.parseTrafficFromLog(vmessLog, `traffic:\s*up=(\d+).*down=(\d+)`)
	if err != nil {
		return 0, 0
	}

	return up, down
}

// parseTrafficFromLog 从日志文件解析流量统计
func (tm *TrafficMonitor) parseTrafficFromLog(logPath, pattern string) (uint64, uint64, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	// 获取文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, 0, err
	}
	fileSize := fileInfo.Size()

	// 检查是否有新内容
	lastPos, exists := tm.lastPositions[logPath]
	if !exists {
		lastPos = 0
	}
	if lastPos >= fileSize {
		return 0, 0, nil // 没有新内容
	}

	// 跳到上次读取的位置
	file.Seek(lastPos, 0)

	var totalUp, totalDown uint64
	re := regexp.MustCompile(pattern)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 3 {
			// 根据正则表达式提取流量数据
			bytes, _ := strconv.ParseUint(matches[1], 10, 64)
			direction := matches[2]

			if direction == "up" {
				totalUp += bytes
			} else if direction == "down" {
				totalDown += bytes
			}
		} else if len(matches) >= 3 {
			// 处理其他格式
			up, _ := strconv.ParseUint(matches[1], 10, 64)
			down, _ := strconv.ParseUint(matches[2], 10, 64)
			totalUp += up
			totalDown += down
		}
	}

	// 更新最后读取位置
	tm.lastPositions[logPath] = fileSize

	return totalUp, totalDown, nil
}
