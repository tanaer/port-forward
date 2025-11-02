package sql

import (
	"log"
)

// UpdateForwardDirect 直接更新单个转发配置（回退路径）
// 场景：批量更新失败时，逐个重试
func UpdateForwardDirect(id int, bytes uint64, gb uint64) error {
	tx := db.Begin()
	if err := tx.Exec("UPDATE connection_stats SET total_bytes = ?, total_gigabyte = ? WHERE id = ?",
		bytes, gb, id).Error; err != nil {
		log.Printf("[UpdateForwardDirect] Update failed for ID %d: %v", id, err)
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
