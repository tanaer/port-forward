package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// NodeEventDAO 节点事件数据访问对象
type NodeEventDAO struct {
	db *sql.DB
}

// NewNodeEventDAO 创建节点事件DAO
func NewNodeEventDAO(db *sql.DB) *NodeEventDAO {
	return &NodeEventDAO{db: db}
}

// NodeEventRecord 节点事件记录
type NodeEventRecord struct {
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	EventType string `json:"event_type"`
	EventData string `json:"event_data"`
	CreatedAt int64  `json:"created_at"`
}

// CreateEvent 创建节点事件
func (dao *NodeEventDAO) CreateEvent(nodeID string, eventType string, eventData string) error {
	now := time.Now().Unix()

	query := `
		INSERT INTO node_events (node_id, event_type, event_data, created_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := dao.db.Exec(query, nodeID, eventType, eventData, now)
	if err != nil {
		return fmt.Errorf("创建节点事件失败: %v", err)
	}

	log.Printf("[DAO] 节点事件已创建: %s - %s", nodeID, eventType)
	return nil
}

// GetEventsByNodeID 根据节点ID获取事件
func (dao *NodeEventDAO) GetEventsByNodeID(nodeID string, limit int) ([]*NodeEventRecord, error) {
	query := `
		SELECT id, node_id, event_type, event_data, created_at
		FROM node_events
		WHERE node_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := dao.db.Query(query, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询节点事件失败: %v", err)
	}
	defer rows.Close()

	var events []*NodeEventRecord
	for rows.Next() {
		var event NodeEventRecord
		err := rows.Scan(
			&event.ID,
			&event.NodeID,
			&event.EventType,
			&event.EventData,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描节点事件记录失败: %v", err)
		}
		events = append(events, &event)
	}

	return events, nil
}

// GetEventsByType 根据事件类型获取事件
func (dao *NodeEventDAO) GetEventsByType(eventType string, limit int) ([]*NodeEventRecord, error) {
	query := `
		SELECT id, node_id, event_type, event_data, created_at
		FROM node_events
		WHERE event_type = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := dao.db.Query(query, eventType, limit)
	if err != nil {
		return nil, fmt.Errorf("查询节点事件失败: %v", err)
	}
	defer rows.Close()

	var events []*NodeEventRecord
	for rows.Next() {
		var event NodeEventRecord
		err := rows.Scan(
			&event.ID,
			&event.NodeID,
			&event.EventType,
			&event.EventData,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描节点事件记录失败: %v", err)
		}
		events = append(events, &event)
	}

	return events, nil
}

// GetRecentEvents 获取最近的事件
func (dao *NodeEventDAO) GetRecentEvents(limit int) ([]*NodeEventRecord, error) {
	query := `
		SELECT id, node_id, event_type, event_data, created_at
		FROM node_events
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := dao.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询最近事件失败: %v", err)
	}
	defer rows.Close()

	var events []*NodeEventRecord
	for rows.Next() {
		var event NodeEventRecord
		err := rows.Scan(
			&event.ID,
			&event.NodeID,
			&event.EventType,
			&event.EventData,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描节点事件记录失败: %v", err)
		}
		events = append(events, &event)
	}

	return events, nil
}

// DeleteEventsByNodeID 根据节点ID删除事件
func (dao *NodeEventDAO) DeleteEventsByNodeID(nodeID string) error {
	query := "DELETE FROM node_events WHERE node_id = ?"
	_, err := dao.db.Exec(query, nodeID)
	if err != nil {
		return fmt.Errorf("删除节点事件失败: %v", err)
	}

	log.Printf("[DAO] 节点事件已删除: %s", nodeID)
	return nil
}

// DeleteOldEvents 删除旧事件（保留最近N天）
func (dao *NodeEventDAO) DeleteOldEvents(days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days).Unix()

	query := "DELETE FROM node_events WHERE created_at < ?"
	result, err := dao.db.Exec(query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("删除旧事件失败: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("获取影响行数失败: %v", err)
	}

	log.Printf("[DAO] 删除 %d 天前的事件，影响行数: %d", days, affected)
	return affected, nil
}
