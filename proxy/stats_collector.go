package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"goForward/conf"
	"goForward/sql"
)

const (
	inboundTrafficUplinkName   = "inbound>>>vless-in>>>traffic>>>uplink"
	inboundTrafficDownlinkName = "inbound>>>vless-in>>>traffic>>>downlink"
)

type xrayStatsCollector struct {
	interval    time.Duration
	stop        chan struct{}
	wg          sync.WaitGroup
	lastTraffic map[int]trafficSnapshot

	logOffsets   map[int]int64
	logOffsetsMu sync.Mutex
}

type trafficSnapshot struct {
	uplink   uint64
	downlink uint64
}

var globalStatsCollector *xrayStatsCollector

// StartStatsCollector 启动统一的 Xray 统计采集器
func StartStatsCollector() {
	if globalStatsCollector != nil {
		return
	}
	globalStatsCollector = &xrayStatsCollector{
		interval:    time.Minute,
		stop:        make(chan struct{}),
		lastTraffic: make(map[int]trafficSnapshot),
		logOffsets:  make(map[int]int64),
	}
	globalStatsCollector.wg.Add(1)
	go globalStatsCollector.loop()
}

// StopStatsCollector 停止统计采集器
func StopStatsCollector() {
	if globalStatsCollector == nil {
		return
	}
	close(globalStatsCollector.stop)
	globalStatsCollector.wg.Wait()
	globalStatsCollector = nil
}

func (c *xrayStatsCollector) loop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	c.collect()
	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.stop:
			return
		}
	}
}

func (c *xrayStatsCollector) collect() {
	proxies := sql.GetActiveProxies()
	if len(proxies) == 0 {
		return
	}
	minute := time.Now().Truncate(time.Minute)
	for _, proxyCfg := range proxies {
		c.collectTraffic(proxyCfg, minute)
		c.collectTargets(proxyCfg)
	}
}

func (c *xrayStatsCollector) collectTraffic(proxyCfg conf.ProxyConfig, minute time.Time) {
	port := conf.XrayAPIPort(proxyCfg.Id)
	uplink, err := queryXrayStat(port, inboundTrafficUplinkName)
	if err != nil {
		return
	}
	downlink, err := queryXrayStat(port, inboundTrafficDownlinkName)
	if err != nil {
		return
	}

	prev := c.lastTraffic[proxyCfg.Id]
	var deltaUp, deltaDown uint64
	if uplink >= prev.uplink {
		deltaUp = uplink - prev.uplink
	} else {
		deltaUp = uplink
	}
	if downlink >= prev.downlink {
		deltaDown = downlink - prev.downlink
	} else {
		deltaDown = downlink
	}
	c.lastTraffic[proxyCfg.Id] = trafficSnapshot{uplink: uplink, downlink: downlink}

	if deltaUp == 0 && deltaDown == 0 {
		return
	}
	sql.AddTrafficSample(proxyCfg.Id, minute, deltaUp, deltaDown)
}

func queryXrayStat(port int, name string) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	binPath := filepath.Join(".", "bin", "xray")
	cmd := exec.CommandContext(ctx, binPath, "api", "statsquery",
		fmt.Sprintf("--server=127.0.0.1:%d", port),
		fmt.Sprintf("--pattern=%s", name),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("stats command failed: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	return parseXrayStatsOutput(string(output), name)
}

func parseXrayStatsOutput(out, name string) (uint64, error) {
	var resp struct {
		Stat []struct {
			Name  string      `json:"name"`
			Value json.Number `json:"value"`
		} `json:"stat"`
	}

	if err := json.Unmarshal([]byte(out), &resp); err == nil && len(resp.Stat) > 0 {
		parseValue := func(n json.Number) (uint64, error) {
			return strconv.ParseUint(n.String(), 10, 64)
		}

		if name != "" {
			for _, stat := range resp.Stat {
				if stat.Name != name {
					continue
				}
				if val, err := parseValue(stat.Value); err == nil {
					return val, nil
				}
			}
		}
		for _, stat := range resp.Stat {
			if val, err := parseValue(stat.Value); err == nil {
				return val, nil
			}
		}
		// If JSON parsed but no usable value, fall through to legacy parsing.
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 支持旧格式: "value: 12345"
		if strings.HasPrefix(line, "value:") {
			valueStr := strings.TrimSpace(strings.TrimPrefix(line, "value:"))
			if valueStr == "" {
				continue
			}
			v, err := strconv.ParseUint(valueStr, 10, 64)
			if err != nil {
				return 0, err
			}
			return v, nil
		}
		// 支持新格式: "\"value\": 12345" (JSON中的value字段)
		if strings.Contains(line, "\"value\"") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				valueStr := strings.TrimSpace(parts[len(parts)-1])
				valueStr = strings.TrimSuffix(valueStr, ",")
				if valueStr == "" {
					continue
				}
				v, err := strconv.ParseUint(valueStr, 10, 64)
				if err != nil {
					return 0, err
				}
				return v, nil
			}
		}
	}
	// 如果没有找到value字段，说明统计项存在但未初始化，返回0（不是错误）
	return 0, nil
}

func (c *xrayStatsCollector) collectTargets(proxyCfg conf.ProxyConfig) {
	path := conf.XrayAccessLogPath(proxyCfg.Id)
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return
	}

	c.logOffsetsMu.Lock()
	offset, ok := c.logOffsets[proxyCfg.Id]
	if !ok {
		offset = info.Size()
		c.logOffsets[proxyCfg.Id] = offset
		c.logOffsetsMu.Unlock()
		return
	}
	if info.Size() < offset {
		offset = 0
	}
	c.logOffsetsMu.Unlock()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		offset = 0
		file.Seek(0, io.SeekStart)
	}

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return
		}
		offset += int64(len(line))
		ts, target, success, ok := parseAccessLogLine(line)
		if !ok {
			continue
		}
		sql.AddTargetSample(proxyCfg.Id, ts.Truncate(time.Minute), target, success)
	}

	c.logOffsetsMu.Lock()
	c.logOffsets[proxyCfg.Id] = offset
	c.logOffsetsMu.Unlock()
}

func parseAccessLogLine(line string) (time.Time, string, bool, bool) {
	line = strings.TrimSpace(line)
	if len(line) < 20 {
		return time.Now(), "", false, false
	}

	tsStr := line[:19]
	ts, err := time.ParseInLocation("2006/01/02 15:04:05", tsStr, time.Local)
	if err != nil {
		ts = time.Now()
	}
	payload := line[20:]

	var success bool
	var marker string
	if idx := strings.Index(payload, " accepted "); idx != -1 {
		success = true
		marker = " accepted "
		payload = payload[idx+len(marker):]
	} else if idx := strings.Index(payload, " rejected "); idx != -1 {
		success = false
		marker = " rejected "
		payload = payload[idx+len(marker):]
	} else if idx := strings.Index(payload, " failed "); idx != -1 {
		success = false
		marker = " failed "
		payload = payload[idx+len(marker):]
	} else {
		return ts, "", false, false
	}

	fields := strings.Fields(payload)
	if len(fields) == 0 {
		return ts, "", success, false
	}

	target := fields[0]
	target = extractTargetHost(target)
	if target == "" {
		return ts, "", success, false
	}
	return ts, target, success, true
}

func extractTargetHost(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	parts := strings.SplitN(token, ":", 3)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return strings.ToLower(parts[0])
	}
	if len(parts) >= 2 {
		return strings.ToLower(parts[1])
	}
	return ""
}
