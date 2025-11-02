package forward

import (
	"log"
	"sync"
	"time"

	"goForward/sql"
)

// StatsAggregator 流量统计聚合器
// 解决数据库写入风暴问题：每5秒×N次 → 每30秒×1次（批量）
type StatsAggregator struct {
	mu      sync.RWMutex
	pending map[int]*sql.PendingStats
	ticker  *time.Ticker
	quit    chan bool
}

// NewStatsAggregator 创建新的统计聚合器
func NewStatsAggregator() *StatsAggregator {
	sa := &StatsAggregator{
		pending: make(map[int]*sql.PendingStats),
		quit:    make(chan bool),
	}

	// 每30秒执行一次批量刷新
	sa.ticker = time.NewTicker(30 * time.Second)
	go sa.flushLoop()

	return sa
}

// Add 添加统计数据（累加逻辑）
func (sa *StatsAggregator) Add(id int, bytes uint64, gb uint64) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if existing, ok := sa.pending[id]; ok {
		// 累加现有值
		existing.Bytes += bytes
		existing.GB += gb
	} else {
		// 新建条目
		sa.pending[id] = &sql.PendingStats{Bytes: bytes, GB: gb}
	}
}

// flush 批量刷新到数据库（带错误处理和回退机制）
func (sa *StatsAggregator) flush() {
	sa.mu.Lock()
	pending := sa.pending
	sa.pending = make(map[int]*sql.PendingStats)
	sa.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	log.Printf("[StatsAggregator] Flushing %d stats to database", len(pending))

	// 优先尝试批量更新
	if err := sql.BatchUpdateStats(pending); err != nil {
		log.Printf("[StatsAggregator] Batch update failed: %v, falling back to individual updates", err)
		// 回退到逐个更新
		for id, stats := range pending {
			if err := sql.UpdateForwardDirect(id, stats.Bytes, stats.GB); err != nil {
				log.Printf("[StatsAggregator] Direct update failed for ID %d: %v", id, err)
				// 重新加入pending（下次重试）
				sa.mu.Lock()
				if existing, ok := sa.pending[id]; ok {
					existing.Bytes += stats.Bytes
					existing.GB += stats.GB
				} else {
					sa.pending[id] = &sql.PendingStats{Bytes: stats.Bytes, GB: stats.GB}
				}
				sa.mu.Unlock()
			}
		}
	} else {
		log.Printf("[StatsAggregator] Batch update successful: %d records", len(pending))
	}
}

// flushLoop 后台定期刷新循环
func (sa *StatsAggregator) flushLoop() {
	for {
		select {
		case <-sa.ticker.C:
			sa.flush()
		case <-sa.quit:
			log.Printf("[StatsAggregator] Shutting down flush loop")
			return
		}
	}
}

// Stop 停止聚合器
func (sa *StatsAggregator) Stop() {
	sa.ticker.Stop()
	close(sa.quit)
	// 执行最后一次刷新
	sa.flush()
}
