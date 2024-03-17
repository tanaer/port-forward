package utils

import (
	"sync"

	"goForward/conf"
	"goForward/forward"
	"goForward/sql"
)

// 增加转发并开启
func AddForward(newF conf.ConnectionStats) bool {
	if newF.LocalPort == conf.WebPort && newF.Protocol == "tcp" {
		return false
	}
	id := sql.AddForward(newF)
	if id > 0 {
		stats := &forward.ConnectionStats{
			ConnectionStats: conf.ConnectionStats{
				Id:         id,
				LocalPort:  newF.LocalPort,
				RemotePort: newF.RemotePort,
				RemoteAddr: newF.RemoteAddr,
				Protocol:   newF.Protocol,
				OutTime:    newF.OutTime,
				TotalBytes: 0,
			},
			TotalBytesOld:  0,
			TotalBytesLock: sync.Mutex{},
			TCPConnections: make(map[string]*forward.IPStruct),
		}
		conf.Wg.Add(1)
		go forward.Run(stats, &conf.Wg)
		return true
	}
	return false
}

// 删除并关闭指定转发
func DelForward(f conf.ConnectionStats) bool {
	sql.DelForward(f.Id)
	conf.Ch <- f.LocalPort + f.Protocol
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
			stats := &forward.ConnectionStats{
				ConnectionStats: conf.ConnectionStats{
					Id:         f.Id,
					LocalPort:  f.LocalPort,
					RemotePort: f.RemotePort,
					RemoteAddr: f.RemoteAddr,
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
			return true
		} else {
			conf.Ch <- f.LocalPort + f.Protocol
			return true
		}
	}

	return false
}