package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// Store 统一数据存储接口
type Store struct {
	db             *sql.DB
	nodeDAO        *NodeDAO
	configDAO      *ProxyConfigDAO
	logDAO         *NodeLogDAO
	eventDAO       *NodeEventDAO
	versionDAO     *ConfigVersionDAO
}

// NewStore 创建存储实例
func NewStore(dbPath string) (*Store, error) {
	// 创建数据库连接
	db, err := NewDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("创建数据库失败: %v", err)
	}

	// 执行数据库迁移
	migrator := NewMigrator(db.db)
	if err := migrator.Migrate(); err != nil {
		log.Printf("[存储] 数据库迁移失败: %v", err)
		return nil, fmt.Errorf("数据库迁移失败: %v", err)
	}

	// 创建DAO实例
	nodeDAO := NewNodeDAO(db.db)
	configDAO := NewProxyConfigDAO(db.db)
	logDAO := NewNodeLogDAO(db.db)
	eventDAO := NewNodeEventDAO(db.db)
	versionDAO := NewConfigVersionDAO(db.db)

	store := &Store{
		db:         db.db,
		nodeDAO:    nodeDAO,
		configDAO:  configDAO,
		logDAO:     logDAO,
		eventDAO:   eventDAO,
		versionDAO: versionDAO,
	}

	log.Println("[存储] 数据存储层已初始化")
	return store, nil
}

// NewMemoryStore 创建内存存储（用于测试）
func NewMemoryStore() (*Store, error) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		return nil, err
	}

	// 执行数据库迁移
	migrator := NewMigrator(db.db)
	if err := migrator.Migrate(); err != nil {
		log.Printf("[存储] 内存数据库迁移失败: %v", err)
		return nil, fmt.Errorf("数据库迁移失败: %v", err)
	}

	nodeDAO := NewNodeDAO(db.db)
	configDAO := NewProxyConfigDAO(db.db)
	logDAO := NewNodeLogDAO(db.db)
	eventDAO := NewNodeEventDAO(db.db)
	versionDAO := NewConfigVersionDAO(db.db)

	return &Store{
		db:         db.db,
		nodeDAO:    nodeDAO,
		configDAO:  configDAO,
		logDAO:     logDAO,
		eventDAO:   eventDAO,
		versionDAO: versionDAO,
	}, nil
}

// NodeDAO 返回节点DAO
func (s *Store) NodeDAO() *NodeDAO {
	return s.nodeDAO
}

// ProxyConfigDAO 返回代理配置DAO
func (s *Store) ProxyConfigDAO() *ProxyConfigDAO {
	return s.configDAO
}

// NodeLogDAO 返回节点日志DAO
func (s *Store) NodeLogDAO() *NodeLogDAO {
	return s.logDAO
}

// NodeEventDAO 返回节点事件DAO
func (s *Store) NodeEventDAO() *NodeEventDAO {
	return s.eventDAO
}

// ConfigVersionDAO 返回配置版本DAO
func (s *Store) ConfigVersionDAO() *ConfigVersionDAO {
	return s.versionDAO
}

// Close 关闭存储
func (s *Store) Close() error {
	log.Println("[存储] 正在关闭数据存储层")
	return s.db.Close()
}

// GetDatabasePath 获取数据库文件路径
func GetDatabasePath() string {
	// 获取当前执行文件目录
	if _, err := os.Executable(); err != nil {
		log.Println("[存储] 无法获取执行文件路径，使用当前目录")
		return "./control.db"
	}

	// 在执行文件同目录下创建数据库
	dbDir := os.Getenv("GOFORWARD_DB_DIR")
	if dbDir == "" {
		dbDir = os.TempDir()
	}

	return fmt.Sprintf("%s/goForward_control.db", dbDir)
}

// HealthCheck 健康检查
func (s *Store) HealthCheck() error {
	if s.db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	if err := s.db.Ping(); err != nil {
		return fmt.Errorf("数据库连接不可用: %v", err)
	}

	// 检查必要表是否存在
	tables := []string{"nodes", "proxy_configs", "node_logs"}
	for _, table := range tables {
		row := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
		var name string
		if err := row.Scan(&name); err != nil {
			return fmt.Errorf("缺少必要表: %s", table)
		}
	}

	return nil
}
