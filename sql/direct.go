package sql

import (
	"log"
)

// UpdateForwardDirect 直接更新单个转发配置（回退路径）
// 场景：批量更新失败时，逐个重试
func UpdateForwardDirect(id int, bytes uint64, gb uint64) error {
	tx := db.Begin()
	// 修复：使用累加而非覆盖，确保增量正确累积到总量
	if err := tx.Exec("UPDATE connection_stats SET total_bytes = total_bytes + ?, total_gigabyte = total_gigabyte + ? WHERE id = ?",
		bytes, gb, id).Error; err != nil {
		log.Printf("[UpdateForwardDirect] Update failed for ID %d: %v", id, err)
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
