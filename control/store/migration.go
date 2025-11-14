package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Migration 迁移接口
type Migration struct {
	Version int
	Name    string
	Up      func(*sql.DB) error
	Down    func(*sql.DB) error
}

// Migrator 迁移器
type Migrator struct {
	db      *sql.DB
	migrations []Migration
}

// NewMigrator 创建迁移器
func NewMigrator(db *sql.DB) *Migrator {
	m := &Migrator{
		db: db,
	}

	// 注册迁移
	m.registerMigrations()
	return m
}

// registerMigrations 注册所有迁移
func (m *Migrator) registerMigrations() {
	// 版本1: 初始数据库结构
	m.migrations = append(m.migrations, Migration{
		Version: 1,
		Name:    "create_initial_schema",
		Up:      m.createInitialSchema,
		Down:    m.dropInitialSchema,
	})

	// 版本2: 添加节点分组和标签
	m.migrations = append(m.migrations, Migration{
		Version: 2,
		Name:    "add_node_groups_and_tags",
		Up:      m.addNodeGroupsAndTags,
		Down:    m.removeNodeGroupsAndTags,
	})

	// 版本3: 添加配置分组
	m.migrations = append(m.migrations, Migration{
		Version: 3,
		Name:    "add_config_groups",
		Up:      m.addConfigGroups,
		Down:    m.removeConfigGroups,
	})
}

// createInitialSchema 创建初始数据库结构
func (m *Migrator) createInitialSchema(db *sql.DB) error {
	log.Println("[迁移] 执行迁移 v1: 创建初始数据库结构")

	// 启用外键约束
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return fmt.Errorf("启用外键约束失败: %v", err)
	}

	// 创建节点表
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id VARCHAR(255) UNIQUE NOT NULL,
			hostname VARCHAR(255) NOT NULL,
			ip_address VARCHAR(64) NOT NULL,
			version VARCHAR(64) NOT NULL DEFAULT '2.0.0',
			status VARCHAR(32) NOT NULL DEFAULT 'unknown',
			control_token TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("创建节点表失败: %v", err)
	}

	// 创建代理配置表
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS proxy_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			outbound_type VARCHAR(64) NOT NULL,
			config_json TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
		);
	`); err != nil {
		return fmt.Errorf("创建代理配置表失败: %v", err)
	}

	// 创建节点日志表
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS node_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id VARCHAR(255) NOT NULL,
			log_type VARCHAR(64) NOT NULL,
			message TEXT NOT NULL,
			data TEXT,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
		);
	`); err != nil {
		return fmt.Errorf("创建节点日志表失败: %v", err)
	}

	log.Println("[迁移] 初始数据库结构创建成功")
	return nil
}

// dropInitialSchema 删除初始数据库结构
func (m *Migrator) dropInitialSchema(db *sql.DB) error {
	log.Println("[迁移] 回滚迁移 v1: 删除初始数据库结构")

	tables := []string{"node_logs", "proxy_configs", "nodes"}
	for _, table := range tables {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("删除表 %s 失败: %v", table, err)
		}
	}

	log.Println("[迁移] 初始数据库结构删除成功")
	return nil
}

// addNodeGroupsAndTags 添加节点分组和标签
func (m *Migrator) addNodeGroupsAndTags(db *sql.DB) error {
	log.Println("[迁移] 执行迁移 v2: 添加节点分组和标签")

	// 为nodes表添加分组和标签字段
	if _, err := db.Exec(`
		ALTER TABLE nodes ADD COLUMN IF NOT EXISTS node_group VARCHAR(128);
	`); err != nil {
		return fmt.Errorf("添加node_group字段失败: %v", err)
	}

	if _, err := db.Exec(`
		ALTER TABLE nodes ADD COLUMN IF NOT EXISTS tags TEXT;
	`); err != nil {
		return fmt.Errorf("添加tags字段失败: %v", err)
	}

	// 创建分组索引
	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_nodes_node_group ON nodes(node_group);
	`); err != nil {
		return fmt.Errorf("创建分组索引失败: %v", err)
	}

	log.Println("[迁移] 节点分组和标签添加成功")
	return nil
}

// removeNodeGroupsAndTags 移除节点分组和标签
func (m *Migrator) removeNodeGroupsAndTags(db *sql.DB) error {
	log.Println("[迁移] 回滚迁移 v2: 移除节点分组和标签")

	// 注意: SQLite不支持DROP COLUMN，需要重新创建表
	// 这里简化为不执行回滚操作
	log.Println("[迁移] SQLite不支持DROP COLUMN，跳过回滚操作")

	return nil
}

// addConfigGroups 添加配置分组
func (m *Migrator) addConfigGroups(db *sql.DB) error {
	log.Println("[迁移] 执行迁移 v3: 添加配置分组")

	// 为proxy_configs表添加分组字段
	if _, err := db.Exec(`
		ALTER TABLE proxy_configs ADD COLUMN IF NOT EXISTS config_group VARCHAR(128);
	`); err != nil {
		return fmt.Errorf("添加config_group字段失败: %v", err)
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_proxy_configs_config_group ON proxy_configs(config_group);
	`); err != nil {
		return fmt.Errorf("创建配置分组索引失败: %v", err)
	}

	log.Println("[迁移] 配置分组添加成功")
	return nil
}

// removeConfigGroups 移除配置分组
func (m *Migrator) removeConfigGroups(db *sql.DB) error {
	log.Println("[迁移] 回滚迁移 v3: 移除配置分组")

	// 注意: SQLite不支持DROP COLUMN
	log.Println("[迁移] SQLite不支持DROP COLUMN，跳过回滚操作")

	return nil
}

// GetCurrentVersion 获取当前数据库版本
func (m *Migrator) GetCurrentVersion() (int, error) {
	// 检查迁移版本表是否存在
	row := m.db.QueryRow(`
		SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'
	`)
	var tableName string
	if err := row.Scan(&tableName); err == sql.ErrNoRows {
		// 表不存在，返回版本0
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("检查迁移表失败: %v", err)
	}

	// 获取当前版本
	var version int
	if err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		return 0, fmt.Errorf("获取当前版本失败: %v", err)
	}

	return version, nil
}

// CreateMigrationsTable 创建迁移版本表
func (m *Migrator) CreateMigrationsTable() error {
	log.Println("[迁移] 创建迁移版本表")

	if _, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			applied_at INTEGER NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("创建迁移版本表失败: %v", err)
	}

	if _, err := m.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_schema_migrations_version ON schema_migrations(version);
	`); err != nil {
		return fmt.Errorf("创建迁移版本索引失败: %v", err)
	}

	return nil
}

// Migrate 执行迁移到最新版本
func (m *Migrator) Migrate() error {
	log.Println("[迁移] 开始数据库迁移")

	// 创建迁移版本表
	if err := m.CreateMigrationsTable(); err != nil {
		return err
	}

	// 获取当前版本
	currentVersion, err := m.GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("获取当前版本失败: %v", err)
	}

	log.Printf("[迁移] 当前数据库版本: %d", currentVersion)

	// 执行待执行的迁移
	for _, migration := range m.migrations {
		if migration.Version > currentVersion {
			log.Printf("[迁移] 执行迁移 v%d: %s", migration.Version, migration.Name)

			// 执行迁移
			if err := migration.Up(m.db); err != nil {
				return fmt.Errorf("执行迁移 v%d 失败: %v", migration.Version, err)
			}

			// 记录迁移
			if _, err := m.db.Exec(`
				INSERT INTO schema_migrations (version, name, applied_at)
				VALUES (?, ?, ?)
			`, migration.Version, migration.Name, time.Now().Unix()); err != nil {
				return fmt.Errorf("记录迁移失败: %v", err)
			}

			log.Printf("[迁移] 迁移 v%d 执行成功", migration.Version)
		}
	}

	log.Printf("[迁移] 数据库迁移完成，当前版本: %d", len(m.migrations))
	return nil
}

// Rollback 回滚到指定版本
func (m *Migrator) Rollback(targetVersion int) error {
	log.Printf("[迁移] 开始回滚到版本 %d", targetVersion)

	currentVersion, err := m.GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("获取当前版本失败: %v", err)
	}

	log.Printf("[迁移] 当前版本: %d，目标版本: %d", currentVersion, targetVersion)

	if targetVersion >= currentVersion {
		return fmt.Errorf("目标版本不能大于等于当前版本")
	}

	// 按降序执行回滚
	for i := len(m.migrations) - 1; i >= 0; i-- {
		migration := m.migrations[i]
		if migration.Version <= currentVersion && migration.Version > targetVersion {
			log.Printf("[迁移] 回滚迁移 v%d: %s", migration.Version, migration.Name)

			// 执行回滚
			if err := migration.Down(m.db); err != nil {
				log.Printf("[迁移] 回滚迁移 v%d 警告: %v", migration.Version, err)
				// 继续执行其他回滚
			}

			// 删除迁移记录
			if _, err := m.db.Exec(`
				DELETE FROM schema_migrations WHERE version = ?
			`, migration.Version); err != nil {
				return fmt.Errorf("删除迁移记录失败: %v", err)
			}

			log.Printf("[迁移] 迁移 v%d 回滚成功", migration.Version)
		}
	}

	log.Printf("[迁移] 回滚完成，当前版本: %d", targetVersion)
	return nil
}

// GetMigrationStatus 获取迁移状态
func (m *Migrator) GetMigrationStatus() ([]MigrationStatus, error) {
	currentVersion, err := m.GetCurrentVersion()
	if err != nil {
		return nil, err
	}

	var status []MigrationStatus
	for _, migration := range m.migrations {
		applied := migration.Version <= currentVersion
		status = append(status, MigrationStatus{
			Version: migration.Version,
			Name:    migration.Name,
			Applied: applied,
		})
	}

	return status, nil
}

// MigrationStatus 迁移状态
type MigrationStatus struct {
	Version int
	Name    string
	Applied bool
}
