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
	if f.OutTime <= 0 {
		f.OutTime = 5
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
