package store

import (
	"database/sql"
	"fmt"
	"time"
)

// NodeLogDAO 节点日志数据访问对象
type NodeLogDAO struct {
	db *sql.DB
}

// NewNodeLogDAO 创建节点日志DAO
func NewNodeLogDAO(db *sql.DB) *NodeLogDAO {
	return &NodeLogDAO{db: db}
}

// CreateLog 创建日志记录
func (dao *NodeLogDAO) CreateLog(log *NodeLogRecord) error {
	if log.CreatedAt == 0 {
		log.CreatedAt = time.Now().Unix()
	}

	query := `
		INSERT INTO node_logs (node_id, log_type, message, data, created_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := dao.db.Exec(query,
		log.NodeID,
		log.LogType,
		log.Message,
		log.Data,
		log.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("创建日志记录失败: %v", err)
	}

	return nil
}

// GetLogsByNodeID 根据节点ID获取日志
func (dao *NodeLogDAO) GetLogsByNodeID(nodeID string, limit int) ([]*NodeLogRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, node_id, log_type, message, data, created_at
		FROM node_logs WHERE node_id = ?
		ORDER BY created_at DESC LIMIT ?
	`

	rows, err := dao.db.Query(query, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询节点日志失败: %v", err)
	}
	defer rows.Close()

	var logs []*NodeLogRecord
	for rows.Next() {
		var log NodeLogRecord
		err := rows.Scan(
			&log.ID,
			&log.NodeID,
			&log.LogType,
			&log.Message,
			&log.Data,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描日志记录失败: %v", err)
		}
		logs = append(logs, &log)
	}

	return logs, nil
}

// GetLogsByType 根据日志类型获取记录
func (dao *NodeLogDAO) GetLogsByType(logType string, limit int) ([]*NodeLogRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, node_id, log_type, message, data, created_at
		FROM node_logs WHERE log_type = ?
		ORDER BY created_at DESC LIMIT ?
	`

	rows, err := dao.db.Query(query, logType, limit)
	if err != nil {
		return nil, fmt.Errorf("根据类型查询日志失败: %v", err)
	}
	defer rows.Close()

	var logs []*NodeLogRecord
	for rows.Next() {
		var log NodeLogRecord
		err := rows.Scan(
			&log.ID,
			&log.NodeID,
			&log.LogType,
			&log.Message,
			&log.Data,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描日志记录失败: %v", err)
		}
		logs = append(logs, &log)
	}

	return logs, nil
}

// GetRecentLogs 获取最近日志
func (dao *NodeLogDAO) GetRecentLogs(limit int) ([]*NodeLogRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, node_id, log_type, message, data, created_at
		FROM node_logs
		ORDER BY created_at DESC LIMIT ?
	`

	rows, err := dao.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询最近日志失败: %v", err)
	}
	defer rows.Close()

	var logs []*NodeLogRecord
	for rows.Next() {
		var log NodeLogRecord
		err := rows.Scan(
			&log.ID,
			&log.NodeID,
			&log.LogType,
			&log.Message,
			&log.Data,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描日志记录失败: %v", err)
		}
		logs = append(logs, &log)
	}

	return logs, nil
}

// GetLogsInTimeRange 获取时间范围内的日志
func (dao *NodeLogDAO) GetLogsInTimeRange(startTime, endTime int64) ([]*NodeLogRecord, error) {
	query := `
		SELECT id, node_id, log_type, message, data, created_at
		FROM node_logs
		WHERE created_at BETWEEN ? AND ?
		ORDER BY created_at DESC
	`

	rows, err := dao.db.Query(query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询时间范围日志失败: %v", err)
	}
	defer rows.Close()

	var logs []*NodeLogRecord
	for rows.Next() {
		var log NodeLogRecord
		err := rows.Scan(
			&log.ID,
			&log.NodeID,
			&log.LogType,
			&log.Message,
			&log.Data,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描日志记录失败: %v", err)
		}
		logs = append(logs, &log)
	}

	return logs, nil
}

// DeleteOldLogs 删除旧日志
func (dao *NodeLogDAO) DeleteOldLogs(days int) (int64, error) {
	cutoffTime := time.Now().Unix() - int64(days*24*60*60)
	query := "DELETE FROM node_logs WHERE created_at < ?"
	result, err := dao.db.Exec(query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("删除旧日志失败: %v", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// CountLogs 统计日志数量
func (dao *NodeLogDAO) CountLogs() (int, error) {
	query := "SELECT COUNT(*) FROM node_logs"
	var count int
	err := dao.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计日志数量失败: %v", err)
	}
	return count, nil
}
