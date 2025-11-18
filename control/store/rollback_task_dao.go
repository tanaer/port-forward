package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// RollbackTaskRecord 回滚任务记录
type RollbackTaskRecord struct {
	ID            int64  `json:"id"`
	NodeID        string `json:"node_id"`
	ConfigID      int32  `json:"config_id"`
	TargetVersion int32  `json:"target_version"`
	Status        string `json:"status"` // pending, processing, completed, failed
	RetryCount    int    `json:"retry_count"`
	Reason        string `json:"reason"`
	ErrorMessage  string `json:"error_message"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// RollbackTaskDAO 回滚任务数据访问对象
type RollbackTaskDAO struct {
	db *sql.DB
}

// NewRollbackTaskDAO 创建回滚任务DAO
func NewRollbackTaskDAO(db *sql.DB) *RollbackTaskDAO {
	return &RollbackTaskDAO{db: db}
}

// CreateTask 创建回滚任务
func (dao *RollbackTaskDAO) CreateTask(task *RollbackTaskRecord) (int64, error) {
	now := time.Now().Unix()
	if task.CreatedAt == 0 {
		task.CreatedAt = now
	}
	if task.UpdatedAt == 0 {
		task.UpdatedAt = now
	}
	if task.Status == "" {
		task.Status = "pending"
	}

	result, err := dao.db.Exec(`
		INSERT INTO rollback_tasks
		(node_id, config_id, target_version, status, retry_count, reason, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.NodeID, task.ConfigID, task.TargetVersion, task.Status, task.RetryCount,
		task.Reason, task.ErrorMessage, task.CreatedAt, task.UpdatedAt)

	if err != nil {
		return 0, fmt.Errorf("创建回滚任务失败: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取任务ID失败: %v", err)
	}

	log.Printf("[RollbackTaskDAO] 创建任务成功: ID=%d, NodeID=%s, ConfigID=%d, TargetVersion=%d",
		id, task.NodeID, task.ConfigID, task.TargetVersion)

	return id, nil
}

// GetPendingTasksByNode 获取节点的待执行任务
func (dao *RollbackTaskDAO) GetPendingTasksByNode(nodeID string) ([]*RollbackTaskRecord, error) {
	rows, err := dao.db.Query(`
		SELECT id, node_id, config_id, target_version, status, retry_count, reason, error_message, created_at, updated_at
		FROM rollback_tasks
		WHERE node_id = ? AND status = 'pending'
		ORDER BY created_at ASC
	`, nodeID)

	if err != nil {
		return nil, fmt.Errorf("查询待执行任务失败: %v", err)
	}
	defer rows.Close()

	var tasks []*RollbackTaskRecord
	for rows.Next() {
		task := &RollbackTaskRecord{}
		err := rows.Scan(
			&task.ID, &task.NodeID, &task.ConfigID, &task.TargetVersion,
			&task.Status, &task.RetryCount, &task.Reason, &task.ErrorMessage,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描任务记录失败: %v", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// UpdateTaskStatus 更新任务状态
func (dao *RollbackTaskDAO) UpdateTaskStatus(id int64, status string, errorMessage string) error {
	result, err := dao.db.Exec(`
		UPDATE rollback_tasks
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, status, errorMessage, time.Now().Unix(), id)

	if err != nil {
		return fmt.Errorf("更新任务状态失败: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %v", err)
	}

	if affected == 0 {
		return fmt.Errorf("任务ID %d 不存在", id)
	}

	log.Printf("[RollbackTaskDAO] 更新任务状态: ID=%d, Status=%s", id, status)
	return nil
}

// IncrementRetryCount 增加重试计数
func (dao *RollbackTaskDAO) IncrementRetryCount(id int64) error {
	result, err := dao.db.Exec(`
		UPDATE rollback_tasks
		SET retry_count = retry_count + 1, updated_at = ?
		WHERE id = ?
	`, time.Now().Unix(), id)

	if err != nil {
		return fmt.Errorf("增加重试计数失败: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %v", err)
	}

	if affected == 0 {
		return fmt.Errorf("任务ID %d 不存在", id)
	}

	log.Printf("[RollbackTaskDAO] 增加重试计数: ID=%d", id)
	return nil
}

// DeleteTask 删除任务
func (dao *RollbackTaskDAO) DeleteTask(id int64) error {
	result, err := dao.db.Exec("DELETE FROM rollback_tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除任务失败: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %v", err)
	}

	if affected == 0 {
		return fmt.Errorf("任务ID %d 不存在", id)
	}

	log.Printf("[RollbackTaskDAO] 删除任务: ID=%d", id)
	return nil
}

// GetTaskByID 根据ID获取任务
func (dao *RollbackTaskDAO) GetTaskByID(id int64) (*RollbackTaskRecord, error) {
	task := &RollbackTaskRecord{}
	err := dao.db.QueryRow(`
		SELECT id, node_id, config_id, target_version, status, retry_count, reason, error_message, created_at, updated_at
		FROM rollback_tasks
		WHERE id = ?
	`, id).Scan(
		&task.ID, &task.NodeID, &task.ConfigID, &task.TargetVersion,
		&task.Status, &task.RetryCount, &task.Reason, &task.ErrorMessage,
		&task.CreatedAt, &task.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("任务ID %d 不存在", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询任务失败: %v", err)
	}

	return task, nil
}

// GetTasksByNode 获取节点的所有任务
func (dao *RollbackTaskDAO) GetTasksByNode(nodeID string) ([]*RollbackTaskRecord, error) {
	rows, err := dao.db.Query(`
		SELECT id, node_id, config_id, target_version, status, retry_count, reason, error_message, created_at, updated_at
		FROM rollback_tasks
		WHERE node_id = ?
		ORDER BY created_at DESC
	`, nodeID)

	if err != nil {
		return nil, fmt.Errorf("查询节点任务失败: %v", err)
	}
	defer rows.Close()

	var tasks []*RollbackTaskRecord
	for rows.Next() {
		task := &RollbackTaskRecord{}
		err := rows.Scan(
			&task.ID, &task.NodeID, &task.ConfigID, &task.TargetVersion,
			&task.Status, &task.RetryCount, &task.Reason, &task.ErrorMessage,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描任务记录失败: %v", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
