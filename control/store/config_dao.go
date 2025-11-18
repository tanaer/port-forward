package store

import (
	"database/sql"
	"fmt"
	"strings"
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
		SELECT id, node_id, name, outbound_type, config_json, inbound_port, config_group, version, created_at, updated_at
		FROM proxy_configs WHERE id = ?
	`

	var config ProxyConfigRecord
	err := dao.db.QueryRow(query, id).Scan(
		&config.ID,
		&config.NodeID,
		&config.Name,
		&config.OutboundType,
		&config.ConfigJSON,
		&config.InboundPort,
		&config.ConfigGroup,
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
		SELECT id, node_id, name, outbound_type, config_json, inbound_port, config_group, version, created_at, updated_at
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
			&config.InboundPort,
			&config.ConfigGroup,
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
		SELECT id, node_id, name, outbound_type, config_json, inbound_port, config_group, version, created_at, updated_at
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
			&config.InboundPort,
			&config.ConfigGroup,
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
		SELECT id, node_id, name, outbound_type, config_json, inbound_port, config_group, version, created_at, updated_at
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
			&config.InboundPort,
			&config.ConfigGroup,
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

// BatchGetConfigsByIDs 批量根据ID获取配置
func (dao *ProxyConfigDAO) BatchGetConfigsByIDs(configIDs []int32) ([]*ProxyConfigRecord, error) {
	if len(configIDs) == 0 {
		return []*ProxyConfigRecord{}, nil
	}

	// 构建 IN 子句
	placeholders := make([]string, len(configIDs))
	args := make([]interface{}, len(configIDs))
	for i, id := range configIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, node_id, name, outbound_type, config_json, inbound_port, config_group, version, created_at, updated_at
		FROM proxy_configs
		WHERE id IN (%s)
		ORDER BY created_at DESC
	`, strings.Join(placeholders, ","))

	rows, err := dao.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("批量查询代理配置失败: %v", err)
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
			&config.InboundPort,
			&config.ConfigGroup,
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

// BatchDeleteConfigs 批量删除代理配置
func (dao *ProxyConfigDAO) BatchDeleteConfigs(configIDs []int32) (int64, error) {
	if len(configIDs) == 0 {
		return 0, nil
	}

	// 构建 IN 子句
	placeholders := make([]string, len(configIDs))
	args := make([]interface{}, len(configIDs))
	for i, id := range configIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		DELETE FROM proxy_configs
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	result, err := dao.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("批量删除代理配置失败: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("获取影响的行数失败: %v", err)
	}

	fmt.Printf("[DAO] 批量删除代理配置完成，影响行数: %d\n", affected)
	return affected, nil
}

// BatchUpdateConfigs 批量更新代理配置
func (dao *ProxyConfigDAO) BatchUpdateConfigs(configs map[int32]*ProxyConfigRecord) (int64, error) {
	if len(configs) == 0 {
		return 0, nil
	}

	tx, err := dao.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开始事务失败: %v", err)
	}
	defer tx.Rollback()

	var totalAffected int64
	for id, config := range configs {
		now := time.Now().Unix()
		query := `
			UPDATE proxy_configs
			SET name = ?, outbound_type = ?, config_json = ?, config_group = ?, version = ?, updated_at = ?
			WHERE id = ?
		`

		result, err := tx.Exec(query,
			config.Name,
			config.OutboundType,
			config.ConfigJSON,
			config.ConfigGroup,
			config.Version,
			now,
			id,
		)
		if err != nil {
			return 0, fmt.Errorf("更新配置 %d 失败: %v", id, err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("获取影响行数失败: %v", err)
		}
		totalAffected += affected
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交事务失败: %v", err)
	}

	fmt.Printf("[DAO] 批量更新代理配���完成，影响行数: %d\n", totalAffected)
	return totalAffected, nil
}

// BatchCreateConfigs 批量创建代理配置
func (dao *ProxyConfigDAO) BatchCreateConfigs(configs []*ProxyConfigRecord) (int64, error) {
	if len(configs) == 0 {
		return 0, nil
	}

	// 验证配置数据
	for i, config := range configs {
		if config.NodeID == "" {
			return 0, fmt.Errorf("配置 %d 缺少 node_id", i)
		}
		if config.Name == "" {
			return 0, fmt.Errorf("配置 %d 缺少 name", i)
		}
		if config.OutboundType == "" {
			return 0, fmt.Errorf("配置 %d 缺少 outbound_type", i)
		}
		if config.ConfigJSON == "" {
			return 0, fmt.Errorf("配置 %d 缺少 config_json", i)
		}

		// 解析 InboundPort
		inboundPort, err := parseInboundPort(config.ConfigJSON)
		if err != nil {
			return 0, fmt.Errorf("解析配置 %d 的 InboundPort 失败: %v", i, err)
		}
		config.InboundPort = inboundPort
	}

	tx, err := dao.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开始事务失败: %v", err)
	}
	defer tx.Rollback()

	var totalAffected int64
	for _, config := range configs {
		now := time.Now().Unix()
		config.CreatedAt = now
		config.UpdatedAt = now
		if config.Version == 0 {
			config.Version = 1
		}

		query := `
			INSERT INTO proxy_configs (node_id, name, outbound_type, config_json, inbound_port, config_group, version, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		result, err := tx.Exec(query,
			config.NodeID,
			config.Name,
			config.OutboundType,
			config.ConfigJSON,
			config.InboundPort,
			config.ConfigGroup,
			config.Version,
			config.CreatedAt,
			config.UpdatedAt,
		)
		if err != nil {
			return 0, fmt.Errorf("创建配置 %s 失败: %v", config.Name, err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("获取影响行数失败: %v", err)
		}
		totalAffected += affected
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交事务失败: %v", err)
	}

	fmt.Printf("[DAO] 批量创建代理配置完成，影响行数: %d\n", totalAffected)
	return totalAffected, nil
}

// GetConfigsByGroup 根据配置分组获取配置
func (dao *ProxyConfigDAO) GetConfigsByGroup(configGroup string) ([]*ProxyConfigRecord, error) {
	query := `
		SELECT id, node_id, name, outbound_type, config_json, inbound_port, config_group, version, created_at, updated_at
		FROM proxy_configs
		WHERE config_group = ?
		ORDER BY created_at DESC
	`

	rows, err := dao.db.Query(query, configGroup)
	if err != nil {
		return nil, fmt.Errorf("根据配置分组查询配置失败: %v", err)
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
			&config.InboundPort,
			&config.ConfigGroup,
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

// parseInboundPort 从配置JSON中解析InboundPort
func parseInboundPort(configJSON string) (int32, error) {
	// 简单的 JSON 解析，寻找 "listen_port" 或 "port" 字段
	// 格式: {"listen_port": 8889} 或 {"port": 8889}
	if configJSON == "" {
		return 0, fmt.Errorf("配置JSON为空")
	}

	// 查找 "listen_port" 字段
	if port := findPortInJSON(configJSON, "listen_port"); port > 0 {
		return port, nil
	}

	// 查找 "port" 字段
	if port := findPortInJSON(configJSON, "port"); port > 0 {
		return port, nil
	}

	return 0, fmt.Errorf("未找到listen_port或port字段")
}

// findPortInJSON 在JSON字符串中查找指定字段的端口值
func findPortInJSON(jsonStr, fieldName string) int32 {
	// 查找字段名
	fieldSearch := "\"" + fieldName + "\""
	fieldIndex := indexOf(jsonStr, fieldSearch)
	if fieldIndex == -1 {
		return 0
	}

	// 查找字段值（冒号后的数字）
	colonIndex := indexOfFrom(jsonStr, ":", fieldIndex+len(fieldSearch))
	if colonIndex == -1 {
		return 0
	}

	// 提取数字
	var port int32
	numberStr := ""
	for i := colonIndex + 1; i < len(jsonStr); i++ {
		c := jsonStr[i]
		if c >= '0' && c <= '9' {
			numberStr += string(c)
		} else if c == ' ' || c == '\t' {
			continue
		} else {
			break
		}
	}

	if numberStr == "" {
		return 0
	}

	// 转换为整数
	port = 0
	for _, c := range numberStr {
		port = port*10 + int32(c-'0')
		if port > 65535 {
			return 0
		}
	}

	return port
}

// indexOf 查找子字符串
func indexOf(s, substr string) int {
	return indexOfFrom(s, substr, 0)
}

// indexOfFrom 从指定位置开始查找子字符串
func indexOfFrom(s, substr string, from int) int {
	if from >= len(s) {
		return -1
	}
	for i := from; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
