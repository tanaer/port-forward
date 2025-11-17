package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// NodeDAO 节点数据访问对象
type NodeDAO struct {
	db *sql.DB
}

// NewNodeDAO 创建节点DAO
func NewNodeDAO(db *sql.DB) *NodeDAO {
	return &NodeDAO{db: db}
}

// CreateNode 创建节点
func (dao *NodeDAO) CreateNode(node *NodeRecord) error {
	now := time.Now().Unix()
	node.CreatedAt = now
	node.UpdatedAt = now
	if node.Status == "" {
		node.Status = "unknown"
	}

	query := `
		INSERT INTO nodes (node_id, hostname, ip_address, version, status, control_token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := dao.db.Exec(query,
		node.NodeID,
		node.Hostname,
		node.IPAddress,
		node.Version,
		node.Status,
		node.ControlToken,
		node.CreatedAt,
		node.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("创建节点失败: %v", err)
	}

	fmt.Printf("[DAO] 节点已创建: %s\n", node.NodeID)
	return nil
}

// GetNodeByID 根据节点ID获取节点
func (dao *NodeDAO) GetNodeByID(nodeID string) (*NodeRecord, error) {
	query := `
		SELECT id, node_id, hostname, ip_address, version, status, control_token, created_at, updated_at
		FROM nodes WHERE node_id = ?
	`

	var node NodeRecord
	err := dao.db.QueryRow(query, nodeID).Scan(
		&node.ID,
		&node.NodeID,
		&node.Hostname,
		&node.IPAddress,
		&node.Version,
		&node.Status,
		&node.ControlToken,
		&node.CreatedAt,
		&node.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询节点失败: %v", err)
	}

	return &node, nil
}

// GetAllNodes 获取所有节点
func (dao *NodeDAO) GetAllNodes() ([]*NodeRecord, error) {
	query := `
		SELECT id, node_id, hostname, ip_address, version, status, control_token, created_at, updated_at
		FROM nodes ORDER BY created_at DESC
	`

	rows, err := dao.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询节点列表失败: %v", err)
	}
	defer rows.Close()

	var nodes []*NodeRecord
	for rows.Next() {
		var node NodeRecord
		err := rows.Scan(
			&node.ID,
			&node.NodeID,
			&node.Hostname,
			&node.IPAddress,
			&node.Version,
			&node.Status,
			&node.ControlToken,
			&node.CreatedAt,
			&node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描节点记录失败: %v", err)
		}
		nodes = append(nodes, &node)
	}

	return nodes, nil
}

// UpdateNode 更新节点
func (dao *NodeDAO) UpdateNode(nodeID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	now := time.Now().Unix()
	updates["updated_at"] = now

	// 构建动态更新SQL
	setParts := []string{}
	args := []interface{}{}
	for key, value := range updates {
		setParts = append(setParts, fmt.Sprintf("%s = ?", key))
		args = append(args, value)
	}
	args = append(args, nodeID)

	query := "UPDATE nodes SET " + strings.Join(setParts, ", ") + " WHERE node_id = ?"

	_, err := dao.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("更新节点失败: %v", err)
	}

	fmt.Printf("[DAO] 节点已更新: %s\n", nodeID)
	return nil
}

// UpdateNodeStatus 更新节点状态
func (dao *NodeDAO) UpdateNodeStatus(nodeID, status string) error {
	return dao.UpdateNode(nodeID, map[string]interface{}{
		"status": status,
	})
}

// UpdateNodeHealth 更新节点健康信息
func (dao *NodeDAO) UpdateNodeHealth(nodeID string, healthJSON string) error {
	// 健康信息存储在日志表中
	// 这里可以调用LogDAO来记录日志
	// 暂时简化处理
	return nil
}

// DeleteNode 删除节点
func (dao *NodeDAO) DeleteNode(nodeID string) error {
	query := "DELETE FROM nodes WHERE node_id = ?"
	_, err := dao.db.Exec(query, nodeID)
	if err != nil {
		return fmt.Errorf("删除节点失败: %v", err)
	}

	fmt.Printf("[DAO] 节点已删除: %s\n", nodeID)
	return nil
}

// CountNodes 统计节点数量
func (dao *NodeDAO) CountNodes() (int, error) {
	query := "SELECT COUNT(*) FROM nodes"
	var count int
	err := dao.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计节点数量失败: %v", err)
	}
	return count, nil
}

// GetNodesByGroup 根据分组获取节点
func (dao *NodeDAO) GetNodesByGroup(group string) ([]*NodeRecord, error) {
	query := `
		SELECT id, node_id, hostname, ip_address, version, status, control_token, node_group, tags, created_at, updated_at
		FROM nodes
		WHERE node_group = ?
		ORDER BY created_at DESC
	`
	rows, err := dao.db.Query(query, group)
	if err != nil {
		return nil, fmt.Errorf("根据分组查询节点失败: %v", err)
	}
	defer rows.Close()

	var nodes []*NodeRecord
	for rows.Next() {
		var node NodeRecord
		var nodeGroup sql.NullString
		var tags sql.NullString
		if err := rows.Scan(
			&node.ID,
			&node.NodeID,
			&node.Hostname,
			&node.IPAddress,
			&node.Version,
			&node.Status,
			&node.ControlToken,
			&nodeGroup,
			&tags,
			&node.CreatedAt,
			&node.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描节点记录失败: %v", err)
		}
		if nodeGroup.Valid {
			node.NodeGroup = nodeGroup.String
		}
		if tags.Valid {
			node.Tags = tags.String
		}
		nodes = append(nodes, &node)
	}
	return nodes, nil
}

// GetNodesByStatus 根据状态获取节点
func (dao *NodeDAO) GetNodesByStatus(status string) ([]*NodeRecord, error) {
	query := `
		SELECT id, node_id, hostname, ip_address, version, status, control_token, created_at, updated_at
		FROM nodes WHERE status = ? ORDER BY created_at DESC
	`

	rows, err := dao.db.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("根据状态查询节点失败: %v", err)
	}
	defer rows.Close()

	var nodes []*NodeRecord
	for rows.Next() {
		var node NodeRecord
		err := rows.Scan(
			&node.ID,
			&node.NodeID,
			&node.Hostname,
			&node.IPAddress,
			&node.Version,
			&node.Status,
			&node.ControlToken,
			&node.CreatedAt,
			&node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描节点记录失败: %v", err)
		}
		nodes = append(nodes, &node)
	}

	return nodes, nil
}

// BatchGetNodesByIDs 批量获取节点
func (dao *NodeDAO) BatchGetNodesByIDs(nodeIDs []string) ([]*NodeRecord, error) {
	if len(nodeIDs) == 0 {
		return []*NodeRecord{}, nil
	}

	query := `
		SELECT id, node_id, hostname, ip_address, version, status, control_token, node_group, tags, created_at, updated_at
		FROM nodes WHERE node_id IN (
	`

	// 构建 IN 子句
	params := make([]interface{}, 0, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		params = append(params, nodeID)
	}

	query += ") ORDER BY created_at DESC"

	rows, err := dao.db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("批量查询节点失败: %v", err)
	}
	defer rows.Close()

	var nodes []*NodeRecord
	for rows.Next() {
		var node NodeRecord
		var nodeGroup sql.NullString
		var tags sql.NullString
		err := rows.Scan(
			&node.ID,
			&node.NodeID,
			&node.Hostname,
			&node.IPAddress,
			&node.Version,
			&node.Status,
			&node.ControlToken,
			&nodeGroup,
			&tags,
			&node.CreatedAt,
			&node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描节点记录失败: %v", err)
		}

		// 处理NULL值
		if nodeGroup.Valid {
			node.NodeGroup = nodeGroup.String
		}
		if tags.Valid {
			node.Tags = tags.String
		}

		nodes = append(nodes, &node)
	}

	return nodes, nil
}

// BatchUpdateNodesStatus 批量更新节点状态
func (dao *NodeDAO) BatchUpdateNodesStatus(nodeIDs []string, status string) (int64, error) {
	if len(nodeIDs) == 0 {
		return 0, fmt.Errorf("节点ID列表为空")
	}

	query := `
		UPDATE nodes SET status = ?, updated_at = ? WHERE node_id IN (
	`

	now := time.Now().Unix()
	params := []interface{}{status, now}

	for i, nodeID := range nodeIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		params = append(params, nodeID)
	}

	query += ")"

	result, err := dao.db.Exec(query, params...)
	if err != nil {
		return 0, fmt.Errorf("批量更新节点状态失败: %v", err)
	}

	updated, _ := result.RowsAffected()
	fmt.Printf("[DAO] 批量更新节点状态完成，更新 %d 个节点\n", updated)
	return updated, nil
}

// IsolateNode 隔离节点
func (dao *NodeDAO) IsolateNode(nodeID, reason string) error {
	_, err := dao.db.Exec(
		"UPDATE nodes SET isolated = 1, isolated_at = ?, isolated_reason = ? WHERE node_id = ?",
		time.Now().Unix(), reason, nodeID,
	)
	if err != nil {
		return fmt.Errorf("隔离节点失败: %v", err)
	}
	return nil
}

// RecoverNode 恢复节点
func (dao *NodeDAO) RecoverNode(nodeID string) error {
	_, err := dao.db.Exec(
		"UPDATE nodes SET isolated = 0, isolated_at = NULL, isolated_reason = NULL WHERE node_id = ?",
		nodeID,
	)
	if err != nil {
		return fmt.Errorf("恢复节点失败: %v", err)
	}
	return nil
}

// IsNodeIsolated 检查节点是否被隔离
func (dao *NodeDAO) IsNodeIsolated(nodeID string) (bool, error) {
	var isolated int
	err := dao.db.QueryRow("SELECT isolated FROM nodes WHERE node_id = ?", nodeID).Scan(&isolated)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("节点 %s 不存在", nodeID)
		}
		return false, fmt.Errorf("查询节点隔离状态失败: %v", err)
	}
	return isolated == 1, nil
}
