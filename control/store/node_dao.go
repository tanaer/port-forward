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
