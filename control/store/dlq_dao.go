package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// DLQTask 死信队列中的任务
type DLQTask struct {
	ID               int64
	OriginalTaskID   int64
	NodeID           string
	ConfigID         int32
	TargetVersion    int32
	Status           string
	FailureReason    string
	RetryCount       int
	MovedToDLQAt     int64
	DLQExpiryAt      int64
	Metadata         sql.NullString
}

// DLQDAO 死信队列数据访问对象
type DLQDAO struct {
	db *sql.DB
}

// NewDLQDAO 创建 DLQ DAO
func NewDLQDAO(db *sql.DB) *DLQDAO {
	return &DLQDAO{db: db}
}

// MoveToDLQ 将失败任务移动到死信队列
func (dao *DLQDAO) MoveToDLQ(originalTaskID int64, nodeID string, configID int32,
	targetVersion int32, status string, failureReason string, retryCount int) (int64, error) {

	now := time.Now().Unix()
	// 30天后过期
	expiryAt := now + (30 * 24 * 60 * 60)

	result, err := dao.db.Exec(`
		INSERT INTO rollback_tasks_dlq
		(original_task_id, node_id, config_id, target_version, status, failure_reason, retry_count, moved_to_dlq_at, dlq_expiry_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, originalTaskID, nodeID, configID, targetVersion, status, failureReason, retryCount, now, expiryAt)

	if err != nil {
		return 0, fmt.Errorf("移动到DLQ失败: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取DLQ任务ID失败: %v", err)
	}

	log.Printf("[DLQDAO] 任务已移动到DLQ: OriginalTaskID=%d, DLQTaskID=%d", originalTaskID, id)
	return id, nil
}

// ListDLQTasks 列出DLQ中的所有任务
func (dao *DLQDAO) ListDLQTasks(limit int) ([]*DLQTask, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := dao.db.Query(`
		SELECT id, original_task_id, node_id, config_id, target_version, status,
		       failure_reason, retry_count, moved_to_dlq_at, dlq_expiry_at, metadata
		FROM rollback_tasks_dlq
		WHERE dlq_expiry_at > ?
		ORDER BY moved_to_dlq_at DESC
		LIMIT ?
	`, time.Now().Unix(), limit)

	if err != nil {
		return nil, fmt.Errorf("查询DLQ任务失败: %v", err)
	}
	defer rows.Close()

	var tasks []*DLQTask
	for rows.Next() {
		task := &DLQTask{}
		err := rows.Scan(
			&task.ID, &task.OriginalTaskID, &task.NodeID, &task.ConfigID,
			&task.TargetVersion, &task.Status, &task.FailureReason,
			&task.RetryCount, &task.MovedToDLQAt, &task.DLQExpiryAt, &task.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描DLQ任务失败: %v", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetDLQTaskByID 根据DLQ任务ID获取任务详情
func (dao *DLQDAO) GetDLQTaskByID(id int64) (*DLQTask, error) {
	task := &DLQTask{}
	err := dao.db.QueryRow(`
		SELECT id, original_task_id, node_id, config_id, target_version, status,
		       failure_reason, retry_count, moved_to_dlq_at, dlq_expiry_at, metadata
		FROM rollback_tasks_dlq
		WHERE id = ?
	`, id).Scan(
		&task.ID, &task.OriginalTaskID, &task.NodeID, &task.ConfigID,
		&task.TargetVersion, &task.Status, &task.FailureReason,
		&task.RetryCount, &task.MovedToDLQAt, &task.DLQExpiryAt, &task.Metadata,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("DLQ任务ID %d 不存在", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询DLQ任务失败: %v", err)
	}

	return task, nil
}

// ReplayFromDLQ 从DLQ重放任务到原队列
func (dao *DLQDAO) ReplayFromDLQ(dlqID int64) error {
	// 获取DLQ中的任务
	dlqTask, err := dao.GetDLQTaskByID(dlqID)
	if err != nil {
		return err
	}

	// 将任务重新插入到 rollback_tasks 表
	now := time.Now().Unix()
	_, err = dao.db.Exec(`
		INSERT INTO rollback_tasks
		(node_id, config_id, target_version, status, retry_count, reason, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, dlqTask.NodeID, dlqTask.ConfigID, dlqTask.TargetVersion, "pending", 0,
	   dlqTask.FailureReason, "", now, now)

	if err != nil {
		return fmt.Errorf("从DLQ重放任务失败: %v", err)
	}

	// 从DLQ中删除
	_, err = dao.db.Exec("DELETE FROM rollback_tasks_dlq WHERE id = ?", dlqID)
	if err != nil {
		return fmt.Errorf("删除DLQ任务失败: %v", err)
	}

	log.Printf("[DLQDAO] 任务已从DLQ重放: DLQTaskID=%d", dlqID)
	return nil
}

// DeleteFromDLQ 从DLQ中删除任务
func (dao *DLQDAO) DeleteFromDLQ(dlqID int64) error {
	result, err := dao.db.Exec("DELETE FROM rollback_tasks_dlq WHERE id = ?", dlqID)
	if err != nil {
		return fmt.Errorf("删除DLQ任务失败: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %v", err)
	}

	if affected == 0 {
		return fmt.Errorf("DLQ任务ID %d 不存在", dlqID)
	}

	log.Printf("[DLQDAO] 任务已从DLQ删除: DLQTaskID=%d", dlqID)
	return nil
}

// CleanupExpiredDLQTasks 清理过期的DLQ任务
func (dao *DLQDAO) CleanupExpiredDLQTasks() (int64, error) {
	result, err := dao.db.Exec(`
		DELETE FROM rollback_tasks_dlq WHERE dlq_expiry_at <= ?
	`, time.Now().Unix())

	if err != nil {
		return 0, fmt.Errorf("清理过期DLQ任务失败: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("获取影响行数失败: %v", err)
	}

	if affected > 0 {
		log.Printf("[DLQDAO] 清理过期DLQ任务: 删除 %d 条", affected)
	}

	return affected, nil
}

// GetDLQTaskCount 获取DLQ中的任务总数（未过期）
func (dao *DLQDAO) GetDLQTaskCount() (int64, error) {
	var count int64
	err := dao.db.QueryRow(`
		SELECT COUNT(*) FROM rollback_tasks_dlq WHERE dlq_expiry_at > ?
	`, time.Now().Unix()).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("查询DLQ任务计数失败: %v", err)
	}

	return count, nil
}
