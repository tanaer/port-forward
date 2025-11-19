package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Database 数据库结构
type Database struct {
	db *sql.DB
}

// NodeRecord 节点记录
type NodeRecord struct {
	ID           int    `json:"id"`
	NodeID       string `json:"node_id"`
	Hostname     string `json:"hostname"`
	IPAddress    string `json:"ip_address"`
	Version      string `json:"version"`
	Status       string `json:"status"`
	ControlToken string `json:"control_token"`
	NodeGroup    string `json:"node_group"`
	Tags         string `json:"tags"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// ProxyConfigRecord 代理配置记录
type ProxyConfigRecord struct {
	ID           int32  `json:"id"`
	NodeID       string `json:"node_id"`
	Name         string `json:"name"`
	OutboundType string `json:"outbound_type"`
	ConfigJSON   string `json:"config_json"`
	InboundPort  int32  `json:"inbound_port"` // 从配置JSON中解析
	ConfigGroup  string `json:"config_group"`
	Version      int32  `json:"version"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// NodeLogRecord 节点日志记录
type NodeLogRecord struct {
	ID        int    `json:"id"`
	NodeID    string `json:"node_id"`
	LogType   string `json:"log_type"` // heartbeat, error, config_update
	Message   string `json:"message"`
	Data      string `json:"data"` // JSON格式的附加数据
	CreatedAt int64  `json:"created_at"`
}

// NewDatabase 创建数据库连接
func NewDatabase(dbPath string) (*Database, error) {
	// 确保目录存在
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %v", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	database := &Database{db: db}

	// 初始化数据库
	if err := database.init(); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %v", err)
	}

	log.Printf("[数据库] SQLite数据库已打开: %s", dbPath)
	return database, nil
}

// init 初始化数据库表
func (d *Database) init() error {
	// 启用外键约束
	if _, err := d.db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return fmt.Errorf("启用外键约束失败: %v", err)
	}

	// 创建节点表
	if err := d.createNodesTable(); err != nil {
		return err
	}

	// 创建代理配置表
	if err := d.createProxyConfigsTable(); err != nil {
		return err
	}

	// 创建节点日志表
	if err := d.createNodeLogsTable(); err != nil {
		return err
	}

	return nil
}

// createNodesTable 创建节点表
func (d *Database) createNodesTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id VARCHAR(255) UNIQUE NOT NULL,
		hostname VARCHAR(255) NOT NULL,
		ip_address VARCHAR(64) NOT NULL,
		version VARCHAR(64) NOT NULL DEFAULT '2.0.0',
		status VARCHAR(32) NOT NULL DEFAULT 'unknown',
		control_token TEXT NOT NULL,
		node_group VARCHAR(128),
		tags TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_nodes_node_id ON nodes(node_id);
	CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
	CREATE INDEX IF NOT EXISTS idx_nodes_ip_address ON nodes(ip_address);
	CREATE INDEX IF NOT EXISTS idx_nodes_node_group ON nodes(node_group);
	`

	if _, err := d.db.Exec(query); err != nil {
		return fmt.Errorf("创建节点表失败: %v", err)
	}

	log.Println("[数据库] 节点表创建成功")
	return nil
}

// createProxyConfigsTable 创建代理配置表
func (d *Database) createProxyConfigsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS proxy_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		outbound_type VARCHAR(64) NOT NULL,
		config_json TEXT NOT NULL,
		inbound_port INTEGER NOT NULL DEFAULT 0,
		config_group VARCHAR(128),
		version INTEGER NOT NULL DEFAULT 1,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_proxy_configs_node_id ON proxy_configs(node_id);
	CREATE INDEX IF NOT EXISTS idx_proxy_configs_outbound_type ON proxy_configs(outbound_type);
	CREATE INDEX IF NOT EXISTS idx_proxy_configs_config_group ON proxy_configs(config_group);
	`

	if _, err := d.db.Exec(query); err != nil {
		return fmt.Errorf("创建代理配置表失败: %v", err)
	}

	log.Println("[数据库] 代理配置表创建成功")
	return nil
}

// createNodeLogsTable 创建节点日志表
func (d *Database) createNodeLogsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS node_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id VARCHAR(255) NOT NULL,
		log_type VARCHAR(64) NOT NULL,
		message TEXT NOT NULL,
		data TEXT,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_node_logs_node_id ON node_logs(node_id);
	CREATE INDEX IF NOT EXISTS idx_node_logs_log_type ON node_logs(log_type);
	CREATE INDEX IF NOT EXISTS idx_node_logs_created_at ON node_logs(created_at);
	`

	if _, err := d.db.Exec(query); err != nil {
		return fmt.Errorf("创建节点日志表失败: %v", err)
	}

	log.Println("[数据库] 节点日志表创建成功")
	return nil
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	log.Println("[数据库] 正在关闭数据库连接")
	return d.db.Close()
}
