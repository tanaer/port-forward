package store

import (
	"database/sql"
	"fmt"
	"time"
)

// ProxyConfigDAO 代理配置数据访问对象
type ProxyConfigDAO struct {
	db *sql.DB
}

// NewProxyConfigDAO 创建代理配置DAO
func NewProxyConfigDAO(db *sql.DB) *ProxyConfigDAO {
	return &ProxyConfigDAO{db: db}
}

// CreateConfig 创建代理配置
func (dao *ProxyConfigDAO) CreateConfig(config *ProxyConfigRecord) error {
	now := time.Now().Unix()
	config.CreatedAt = now
	config.UpdatedAt = now
	if config.Version == 0 {
		config.Version = 1
	}

	query := `
		INSERT INTO proxy_configs (node_id, name, outbound_type, config_json, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := dao.db.Exec(query,
		config.NodeID,
		config.Name,
		config.OutboundType,
		config.ConfigJSON,
		config.Version,
		config.CreatedAt,
		config.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("创建代理配置失败: %v", err)
	}

	fmt.Printf("[DAO] 代理配置已创建: %s (节点: %s)\n", config.Name, config.NodeID)
	return nil
}

// GetConfigByID 根据ID获取配置
func (dao *ProxyConfigDAO) GetConfigByID(id int32) (*ProxyConfigRecord, error) {
	query := `
		SELECT id, node_id, name, outbound_type, config_json, version, created_at, updated_at
		FROM proxy_configs WHERE id = ?
	`

	var config ProxyConfigRecord
	err := dao.db.QueryRow(query, id).Scan(
		&config.ID,
		&config.NodeID,
		&config.Name,
		&config.OutboundType,
		&config.ConfigJSON,
		&config.Version,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询代理配置失败: %v", err)
	}

	return &config, nil
}

// GetAllConfigs 获取所有配置
func (dao *ProxyConfigDAO) GetAllConfigs() ([]*ProxyConfigRecord, error) {
	query := `
		SELECT id, node_id, name, outbound_type, config_json, version, created_at, updated_at
		FROM proxy_configs ORDER BY created_at DESC
	`

	rows, err := dao.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询代理配置列表失败: %v", err)
	}
	defer rows.Close()

	var configs []*ProxyConfigRecord
	for rows.Next() {
		var config ProxyConfigRecord
		err := rows.Scan(
			&config.ID,
			&config.NodeID,
			&config.Name,
			&config.OutboundType,
			&config.ConfigJSON,
			&config.Version,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描代理配置记录失败: %v", err)
		}
		configs = append(configs, &config)
	}

	return configs, nil
}

// GetConfigsByNodeID 根据节点ID获取配置
func (dao *ProxyConfigDAO) GetConfigsByNodeID(nodeID string) ([]*ProxyConfigRecord, error) {
	query := `
		SELECT id, node_id, name, outbound_type, config_json, version, created_at, updated_at
		FROM proxy_configs WHERE node_id = ? ORDER BY created_at DESC
	`

	rows, err := dao.db.Query(query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("根据节点ID查询配置失败: %v", err)
	}
	defer rows.Close()

	var configs []*ProxyConfigRecord
	for rows.Next() {
		var config ProxyConfigRecord
		err := rows.Scan(
			&config.ID,
			&config.NodeID,
			&config.Name,
			&config.OutboundType,
			&config.ConfigJSON,
			&config.Version,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描代理配置记录失败: %v", err)
		}
		configs = append(configs, &config)
	}

	return configs, nil
}

// UpdateConfig 更新代理配置
func (dao *ProxyConfigDAO) UpdateConfig(id int32, configJSON string, version int32) error {
	now := time.Now().Unix()

	query := `
		UPDATE proxy_configs
		SET config_json = ?, version = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := dao.db.Exec(query, configJSON, version, now, id)
	if err != nil {
		return fmt.Errorf("更新代理配置失败: %v", err)
	}

	fmt.Printf("[DAO] 代理配置已更新: %d\n", id)
	return nil
}

// DeleteConfig 删除代理配置
func (dao *ProxyConfigDAO) DeleteConfig(id int32) error {
	query := "DELETE FROM proxy_configs WHERE id = ?"
	_, err := dao.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("删除代理配置失败: %v", err)
	}

	fmt.Printf("[DAO] 代理配置已删除: %d\n", id)
	return nil
}

// DeleteConfigsByNodeID 根据节点ID删除配置
func (dao *ProxyConfigDAO) DeleteConfigsByNodeID(nodeID string) error {
	query := "DELETE FROM proxy_configs WHERE node_id = ?"
	_, err := dao.db.Exec(query, nodeID)
	if err != nil {
		return fmt.Errorf("删除节点配置失败: %v", err)
	}

	fmt.Printf("[DAO] 节点配置已删除: %s\n", nodeID)
	return nil
}

// CountConfigs 统计配置数量
func (dao *ProxyConfigDAO) CountConfigs() (int, error) {
	query := "SELECT COUNT(*) FROM proxy_configs"
	var count int
	err := dao.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计配置数量失败: %v", err)
	}
	return count, nil
}

// GetConfigsByOutboundType 根据出站类型获取配置
func (dao *ProxyConfigDAO) GetConfigsByOutboundType(outboundType string) ([]*ProxyConfigRecord, error) {
	query := `
		SELECT id, node_id, name, outbound_type, config_json, version, created_at, updated_at
		FROM proxy_configs WHERE outbound_type = ? ORDER BY created_at DESC
	`

	rows, err := dao.db.Query(query, outboundType)
	if err != nil {
		return nil, fmt.Errorf("根据出站类型查询配置失败: %v", err)
	}
	defer rows.Close()

	var configs []*ProxyConfigRecord
	for rows.Next() {
		var config ProxyConfigRecord
		err := rows.Scan(
			&config.ID,
			&config.NodeID,
			&config.Name,
			&config.OutboundType,
			&config.ConfigJSON,
			&config.Version,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描代理配置记录失败: %v", err)
		}
		configs = append(configs, &config)
	}

	return configs, nil
}
