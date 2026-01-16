package utils

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"goForward/conf"
	"goForward/forward"
	"goForward/sql"
)

func spawnForwardInstance(f conf.ConnectionStats) {
	stats := &forward.ConnectionStats{
		ConnectionStats: conf.ConnectionStats{
			Id:         f.Id,
			LocalPort:  f.LocalPort,
			RemotePort: f.RemotePort,
			RemoteAddr: f.RemoteAddr,
			Blacklist:  f.Blacklist,
			Whitelist:  f.Whitelist,
			Protocol:   f.Protocol,
			OutTime:    f.OutTime,
			Remark:     f.Remark,
			TotalBytes: f.TotalBytes,
		},
		TotalBytesOld:  f.TotalBytes,
		TotalBytesLock: sync.Mutex{},
		TCPConnections: make(map[string]*forward.IPStruct),
	}
	conf.Wg.Add(1)
	go forward.Run(stats, &conf.Wg)
}

func sanitizeConnectionInput(f conf.ConnectionStats) conf.ConnectionStats {
	f.LocalPort = strings.ReplaceAll(f.LocalPort, " ", "")
	f.RemotePort = strings.ReplaceAll(f.RemotePort, " ", "")
	f.RemoteAddr = strings.ReplaceAll(f.RemoteAddr, " ", "")
	f.Blacklist = strings.ReplaceAll(f.Blacklist, " ", "")
	f.Whitelist = strings.ReplaceAll(f.Whitelist, " ", "")
	f.Remark = strings.TrimSpace(f.Remark)
	if f.OutTime < 0 {
		f.OutTime = conf.DefaultOutTime
	}
	return f
}

// 增加转发并开启
func AddForward(newF conf.ConnectionStats) bool {
	if newF.Protocol != "udp" {
		newF.Protocol = "tcp"
	}
	if newF.LocalPort == conf.WebPort && newF.Protocol == "tcp" {
		return false
	}
	id := sql.AddForward(newF)
	if id > 0 {
		if strings.Contains(newF.LocalPort, ",") {
			localPorts := strings.Split(newF.LocalPort, ",")
			for _, localPort := range localPorts {
				cfg := conf.ConnectionStats{
					Id:         id,
					LocalPort:  localPort,
					RemotePort: localPort,
					RemoteAddr: newF.RemoteAddr,
					Blacklist:  newF.Blacklist,
					Whitelist:  newF.Whitelist,
					Protocol:   newF.Protocol,
					OutTime:    newF.OutTime,
					Remark:     newF.Remark,
					TotalBytes: 0,
				}
				spawnForwardInstance(cfg)
			}
		} else {
			cfg := conf.ConnectionStats{
				Id:         id,
				LocalPort:  newF.LocalPort,
				RemotePort: newF.RemotePort,
				RemoteAddr: newF.RemoteAddr,
				Blacklist:  newF.Blacklist,
				Whitelist:  newF.Whitelist,
				Protocol:   newF.Protocol,
				OutTime:    newF.OutTime,
				Remark:     newF.Remark,
				TotalBytes: 0,
			}
			spawnForwardInstance(cfg)
		}
		return true
	}
	return false
}

// 删除并关闭指定转发
func DelForward(f conf.ConnectionStats) bool {
	sql.DelForward(f.Id)
	if strings.Contains(f.LocalPort, ",") {
		localPorts := strings.Split(f.LocalPort, ",")
		for _, localPort := range localPorts {
			conf.Ch <- localPort + f.Protocol
		}
	} else {
		conf.Ch <- f.LocalPort + f.Protocol
	}

	return true
}

// 改变转发状态
func ExStatus(f conf.ConnectionStats) bool {
	if sql.FreeForward(f.LocalPort, f.Protocol) {
		return false
	}
	if sql.UpdateForwardStatus(f.Id, f.Status) {
		// 启用转发
		if f.Status == 0 {
			if strings.Contains(f.LocalPort, ",") {
				localPorts := strings.Split(f.LocalPort, ",")
				for _, localPort := range localPorts {
					cfg := f
					cfg.LocalPort = localPort
					cfg.RemotePort = localPort
					spawnForwardInstance(cfg)
				}
			} else {
				spawnForwardInstance(f)
			}
			return true
		}
		if strings.Contains(f.LocalPort, ",") {
			localPorts := strings.Split(f.LocalPort, ",")
			for _, localPort := range localPorts {
				conf.Ch <- localPort + f.Protocol
			}
		} else {
			conf.Ch <- f.LocalPort + f.Protocol
		}
		return true
	}

	return false
}

// 更新转发配置
func UpdateForward(newF conf.ConnectionStats) (bool, string) {
	existing := sql.GetForward(newF.Id)
	if existing.Id == 0 {
		return false, "转发不存在"
	}

	newF = sanitizeConnectionInput(newF)

	if newF.Protocol != "udp" {
		newF.Protocol = "tcp"
	}
	if newF.LocalPort == conf.WebPort && newF.Protocol == "tcp" {
		return false, "不可占用面板端口"
	}

	skipPortCheck := newF.LocalPort == existing.LocalPort && newF.Protocol == existing.Protocol
	if !sql.FreeForwardExclude(newF.LocalPort, newF.Protocol, newF.Id, skipPortCheck) {
		return false, "端口已被占用"
	}

	newF.TotalBytes = existing.TotalBytes
	newF.TotalGigabyte = existing.TotalGigabyte
	newF.Status = existing.Status

	if !sql.UpdateForwardConfig(newF) {
		return false, "更新失败"
	}

	if existing.Status == 0 {
		conf.Ch <- existing.LocalPort + existing.Protocol
		time.Sleep(200 * time.Millisecond)
		refreshed := sql.GetForward(newF.Id)
		refreshed.TotalBytes = existing.TotalBytes
		refreshed.TotalGigabyte = existing.TotalGigabyte
		spawnForwardInstance(refreshed)
	}

	return true, ""
}

// 批量更新协议集合
func UpdateForwardGroup(base conf.ConnectionStats, protocols []string) (bool, string) {
	existing := sql.GetForward(base.Id)
	if existing.Id == 0 {
		return false, "转发不存在"
	}

	protoSet := map[string]struct{}{}
	order := []string{"tcp", "udp"}
	for _, proto := range protocols {
		p := strings.ToLower(strings.TrimSpace(proto))
		if p != "tcp" && p != "udp" {
			continue
		}
		if _, ok := protoSet[p]; !ok {
			protoSet[p] = struct{}{}
		}
	}
	if len(protoSet) == 0 {
		return false, "协议类型不正确"
	}

	base = sanitizeConnectionInput(base)
	base.TotalBytes = existing.TotalBytes
	base.TotalGigabyte = existing.TotalGigabyte

	existingEntries := map[string]conf.ConnectionStats{}
	for _, proto := range order {
		entry := sql.GetForwardByPortAndProtocol(existing.LocalPort, proto)
		if entry.Id != 0 {
			existingEntries[proto] = entry
		}
	}

	for _, proto := range order {
		if _, ok := protoSet[proto]; !ok {
			continue
		}
		if entry, ok := existingEntries[proto]; ok {
			update := base
			update.Id = entry.Id
			update.Protocol = proto
			if ok, msg := UpdateForward(update); !ok {
				return false, msg
			}
		} else {
			if !sql.FreeForward(base.LocalPort, proto) {
				return false, fmt.Sprintf("%s 协议端口已占用", strings.ToUpper(proto))
			}
			newEntry := base
			newEntry.Id = 0
			newEntry.Protocol = proto
			newEntry.Status = existing.Status
			newEntry.TotalBytes = 0
			newEntry.TotalGigabyte = 0
			newID := sql.AddForward(newEntry)
			if newID == 0 {
				return false, fmt.Sprintf("%s 协议保存失败", strings.ToUpper(proto))
			}
			if existing.Status == 0 {
				newEntry.Id = newID
				spawnForwardInstance(newEntry)
			}
		}
	}

	for proto, entry := range existingEntries {
		if _, ok := protoSet[proto]; ok {
			continue
		}
		DelForward(entry)
	}

	return true, ""
}

type ImportDefinition struct {
	LocalPort  string `json:"localPort"`
	RemotePort string `json:"remotePort"`
	RemoteAddr string `json:"remoteAddr"`
	Protocol   string `json:"protocol"`
	OutTime    int    `json:"outTime"`
	Whitelist  string `json:"whitelist"`
	Blacklist  string `json:"blacklist"`
	Remark     string `json:"remark"`
}

type ImportSummary struct {
	Added   int      `json:"added"`
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Failed  []string `json:"failed"`
}

func (s ImportSummary) Message() string {
	return fmt.Sprintf("导入完成，新建%d条，更新%d条，跳过重复%d条，失败%d条", s.Added, s.Updated, s.Skipped, len(s.Failed))
}

func ImportForwardDefinitions(defs []ImportDefinition) ImportSummary {
	summary := ImportSummary{}
	for idx, def := range defs {
		proto := strings.ToLower(strings.TrimSpace(def.Protocol))
		if proto != "udp" {
			proto = "tcp"
		}
		cfg := conf.ConnectionStats{
			LocalPort:  def.LocalPort,
			RemotePort: def.RemotePort,
			RemoteAddr: def.RemoteAddr,
			Whitelist:  def.Whitelist,
			Blacklist:  def.Blacklist,
			Remark:     def.Remark,
			OutTime:    def.OutTime,
			Protocol:   proto,
		}
		cfg = sanitizeConnectionInput(cfg)
		cfg.Protocol = proto
		if cfg.LocalPort == "" || cfg.RemoteAddr == "" {
			summary.Failed = append(summary.Failed, fmt.Sprintf("第%d条本地或远程地址为空", idx+1))
			continue
		}
		if cfg.RemotePort == "" {
			cfg.RemotePort = cfg.LocalPort
		}
		existing := sql.GetForwardByPortAndProtocol(cfg.LocalPort, cfg.Protocol)
		if existing.Id != 0 {
			existingSan := sanitizeConnectionInput(existing)
			if existingSan.RemotePort == cfg.RemotePort &&
				existingSan.RemoteAddr == cfg.RemoteAddr &&
				existingSan.OutTime == cfg.OutTime &&
				existingSan.Whitelist == cfg.Whitelist &&
				existingSan.Blacklist == cfg.Blacklist &&
				existingSan.Remark == cfg.Remark {
				summary.Skipped++
				continue
			}
			cfg.Id = existing.Id
			cfg.TotalBytes = existing.TotalBytes
			cfg.TotalGigabyte = existing.TotalGigabyte
			cfg.Status = existing.Status
			if ok, msg := UpdateForward(cfg); !ok {
				summary.Failed = append(summary.Failed, fmt.Sprintf("第%d条更新失败:%s", idx+1, msg))
				continue
			}
			summary.Updated++
			continue
		}
		if !AddForward(cfg) {
			summary.Failed = append(summary.Failed, fmt.Sprintf("第%d条添加失败，端口可能占用", idx+1))
			continue
		}
		summary.Added++
	}
	return summary
}

// RestartForward 重启指定转发（用于配置热更新）
// 先停止旧的转发器，然后重新启动新的配置
func RestartForward(f conf.ConnectionStats) bool {
	// 先停止旧的转发器
	if strings.Contains(f.LocalPort, ",") {
		localPorts := strings.Split(f.LocalPort, ",")
		for _, localPort := range localPorts {
			conf.Ch <- localPort + f.Protocol
		}
	} else {
		conf.Ch <- f.LocalPort + f.Protocol
	}

	// 等待转发器完全停止
	time.Sleep(100 * time.Millisecond)

	// 重新启动转发器
	return AddForward(f)
}
