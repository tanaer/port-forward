package sql

import (
	"log"
)

// BatchUpdateStats 批量更新统计数据（优化路径）
// 优势：单个事务，减少数据库压力
// 失败处理：遇到错误立即回滚
func BatchUpdateStats(stats map[int]*PendingStats) error {
	if len(stats) == 0 {
		return nil
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[BatchUpdateStats] Recovered from panic: %v", r)
			tx.Rollback()
		}
	}()

	for id, ps := range stats {
		if err := tx.Exec("UPDATE connection_stats SET total_bytes = ?, total_gigabyte = ? WHERE id = ?",
			ps.Bytes, ps.GB, id).Error; err != nil {
			log.Printf("[BatchUpdateStats] Update failed for ID %d: %v", id, err)
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}
