package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ConfigVersionRecord 配置版本记录
type ConfigVersionRecord struct {
	ID             int64  `json:"id"`
	ConfigID       int32  `json:"config_id"`
	Version        int32  `json:"version"`
	ConfigSnapshot string `json:"config_snapshot"` // 完整配置JSON快照
	ChangeType     string `json:"change_type"`     // create, update, rollback
	ChangeSummary  string `json:"change_summary"`  // 变更摘要
	CreatedBy      string `json:"created_by"`      // 操作者标识
	CreatedAt      int64  `json:"created_at"`
}

// ConfigVersionDAO 配置版本数据访问对象
type ConfigVersionDAO struct {
	db *sql.DB
}

// NewConfigVersionDAO 创建配置版本DAO
func NewConfigVersionDAO(db *sql.DB) *ConfigVersionDAO {
	return &ConfigVersionDAO{db: db}
}

// CreateVersion 创建配置版本快照
func (dao *ConfigVersionDAO) CreateVersion(configID int32, version int32, snapshot string, changeType, changeSummary, createdBy string) (*ConfigVersionRecord, error) {
	now := time.Now().Unix()

	query := `
		INSERT INTO config_versions (config_id, version, config_snapshot, change_type, change_summary, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := dao.db.Exec(query,
		configID,
		version,
		snapshot,
		changeType,
		changeSummary,
		createdBy,
		now,
	)

	if err != nil {
		return nil, fmt.Errorf("创建配置版本失败: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取版本ID失败: %v", err)
	}
	fmt.Printf("[DAO] 配置版本已创建: config_id=%d, version=%d, id=%d\n", configID, version, id)

	return &ConfigVersionRecord{
		ID:             id,
		ConfigID:       configID,
		Version:        version,
		ConfigSnapshot: snapshot,
		ChangeType:     changeType,
		ChangeSummary:  changeSummary,
		CreatedBy:      createdBy,
		CreatedAt:      now,
	}, nil
}

// GetVersionByID 根据版本ID获取配置版本
func (dao *ConfigVersionDAO) GetVersionByID(id int64) (*ConfigVersionRecord, error) {
	query := `
		SELECT id, config_id, version, config_snapshot, change_type, change_summary, created_by, created_at
		FROM config_versions
		WHERE id = ?
	`

	record := &ConfigVersionRecord{}
	err := dao.db.QueryRow(query, id).Scan(
		&record.ID,
		&record.ConfigID,
		&record.Version,
		&record.ConfigSnapshot,
		&record.ChangeType,
		&record.ChangeSummary,
		&record.CreatedBy,
		&record.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询配置版本失败: %v", err)
	}

	return record, nil
}

// GetVersionByConfigIDAndVersion 根据配置ID和版本号获取
func (dao *ConfigVersionDAO) GetVersionByConfigIDAndVersion(configID int32, version int32) (*ConfigVersionRecord, error) {
	query := `
		SELECT id, config_id, version, config_snapshot, change_type, change_summary, created_by, created_at
		FROM config_versions
		WHERE config_id = ? AND version = ?
	`

	record := &ConfigVersionRecord{}
	err := dao.db.QueryRow(query, configID, version).Scan(
		&record.ID,
		&record.ConfigID,
		&record.Version,
		&record.ConfigSnapshot,
		&record.ChangeType,
		&record.ChangeSummary,
		&record.CreatedBy,
		&record.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询配置版本失败: %v", err)
	}

	return record, nil
}

// GetVersionHistory 获取配置版本历史（分页）
func (dao *ConfigVersionDAO) GetVersionHistory(configID int32, limit, offset int) ([]*ConfigVersionRecord, error) {
	query := `
		SELECT id, config_id, version, config_snapshot, change_type, change_summary, created_by, created_at
		FROM config_versions
		WHERE config_id = ?
		ORDER BY version DESC
		LIMIT ? OFFSET ?
	`

	rows, err := dao.db.Query(query, configID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询配置版本历史失败: %v", err)
	}
	defer rows.Close()

	var records []*ConfigVersionRecord
	for rows.Next() {
		record := &ConfigVersionRecord{}
		err := rows.Scan(
			&record.ID,
			&record.ConfigID,
			&record.Version,
			&record.ConfigSnapshot,
			&record.ChangeType,
			&record.ChangeSummary,
			&record.CreatedBy,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描配置版本记录失败: %v", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// GetLatestVersion 获取最新版本号
func (dao *ConfigVersionDAO) GetLatestVersion(configID int32) (int32, error) {
	query := `
		SELECT COALESCE(MAX(version), 0)
		FROM config_versions
		WHERE config_id = ?
	`

	var version int32
	err := dao.db.QueryRow(query, configID).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("查询最新版本号失败: %v", err)
	}

	return version, nil
}

// DeleteVersionsOlderThan 删除指定时间之前的版本（保留最近N个版本）
func (dao *ConfigVersionDAO) DeleteVersionsOlderThan(configID int32, keepCount int) error {
	// 先找出要保留的最小版本号
	query := `
		SELECT version FROM config_versions
		WHERE config_id = ?
		ORDER BY version DESC
		LIMIT 1 OFFSET ?
	`

	var minVersionToKeep int32
	err := dao.db.QueryRow(query, configID, keepCount-1).Scan(&minVersionToKeep)
	if err != nil {
		if err == sql.ErrNoRows {
			// 版本数少于keepCount，不删除
			return nil
		}
		return fmt.Errorf("查询最小保留版本失败: %v", err)
	}

	// 删除旧版本
	deleteQuery := `
		DELETE FROM config_versions
		WHERE config_id = ? AND version < ?
	`

	_, err = dao.db.Exec(deleteQuery, configID, minVersionToKeep)
	if err != nil {
		return fmt.Errorf("删除旧版本失败: %v", err)
	}

	fmt.Printf("[DAO] 已删除配置 %d 的旧版本（保留%d个最新版本）\n", configID, keepCount)
	return nil
}

// CountVersions 统计版本数
func (dao *ConfigVersionDAO) CountVersions(configID int32) (int, error) {
	query := `SELECT COUNT(*) FROM config_versions WHERE config_id = ?`

	var count int
	err := dao.db.QueryRow(query, configID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计版本数失败: %v", err)
	}

	return count, nil
}

// VersionDiff 配置版本差异结构
type VersionDiff struct {
	FromVersion   int32       `json:"from_version"`
	ToVersion     int32       `json:"to_version"`
	OldSnapshot   interface{} `json:"old_snapshot"`
	NewSnapshot   interface{} `json:"new_snapshot"`
	ChangedFields []string    `json:"changed_fields"`
}

// GetVersionDiff 获取两个版本之间的差异
func (dao *ConfigVersionDAO) GetVersionDiff(configID int32, fromVersion, toVersion int32) (*VersionDiff, error) {
	// 获取源版本
	fromRecord, err := dao.GetVersionByConfigIDAndVersion(configID, fromVersion)
	if err != nil {
		return nil, err
	}
	if fromRecord == nil {
		return nil, fmt.Errorf("源版本不存在: version=%d", fromVersion)
	}

	// 获取目标版本
	toRecord, err := dao.GetVersionByConfigIDAndVersion(configID, toVersion)
	if err != nil {
		return nil, err
	}
	if toRecord == nil {
		return nil, fmt.Errorf("目标版本不存在: version=%d", toVersion)
	}

	// 解析两个快照
	var oldConfig, newConfig map[string]interface{}
	if err := json.Unmarshal([]byte(fromRecord.ConfigSnapshot), &oldConfig); err != nil {
		return nil, fmt.Errorf("解析源版本快照失败: %v", err)
	}
	if err := json.Unmarshal([]byte(toRecord.ConfigSnapshot), &newConfig); err != nil {
		return nil, fmt.Errorf("解析目标版本快照失败: %v", err)
	}

	// 计算差异字段
	changedFields := []string{}
	for key := range oldConfig {
		if oldConfig[key] != newConfig[key] {
			changedFields = append(changedFields, key)
		}
	}
	for key := range newConfig {
		if _, exists := oldConfig[key]; !exists {
			changedFields = append(changedFields, key)
		}
	}

	return &VersionDiff{
		FromVersion:   fromVersion,
		ToVersion:     toVersion,
		OldSnapshot:   oldConfig,
		NewSnapshot:   newConfig,
		ChangedFields: changedFields,
	}, nil
}
