package hotreload

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"goForward/conf"
	"goForward/sql"
	"goForward/utils"
	"goForward/validator"
	yamlv3 "gopkg.in/yaml.v3"
)

// HotReloader 配置热更新管理器
type HotReloader struct {
	watchers     map[string]*fsnotify.Watcher
	configPaths  []string
	reloadHandler func() // 重新加载配置的处理函数
}

// NewHotReloader 创建新的配置热更新管理器
func NewHotReloader() *HotReloader {
	return &HotReloader{
		watchers:    make(map[string]*fsnotify.Watcher),
		configPaths: make([]string, 0),
	}
}

// AddConfigPath 添加要监控的配置文件路径
func (hr *HotReloader) AddConfigPath(path string) {
	// 确保路径是绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Printf("[HotReloader] 转换绝对路径失败: %v", err)
		return
	}

	hr.configPaths = append(hr.configPaths, absPath)
}

// SetReloadHandler 设置重新加载配置的处理函数
func (hr *HotReloader) SetReloadHandler(handler func()) {
	hr.reloadHandler = handler
}

// Start 启动配置热更新监控
func (hr *HotReloader) Start() error {
	if len(hr.configPaths) == 0 {
		log.Println("[HotReloader] 没有配置文件需要监控")
		return nil
	}

	// 为每个配置文件创建监控器
	for _, configPath := range hr.configPaths {
		if err := hr.watchConfig(configPath); err != nil {
			log.Printf("[HotReloader] 监控配置文件失败 %s: %v", configPath, err)
			return err
		}
	}

	log.Printf("[HotReloader] 已启动监控 %d 个配置文件", len(hr.configPaths))
	return nil
}

// watchConfig 监控单个配置文件
func (hr *HotReloader) watchConfig(configPath string) error {
	// 创建新的监控器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建监控器失败: %w", err)
	}
	hr.watchers[configPath] = watcher

	// 获取配置文件的目录
	configDir := filepath.Dir(configPath)

	// 监控目录（而非文件本身），这样可以捕获文件被替换的情况
	if err := watcher.Add(configDir); err != nil {
		return fmt.Errorf("添加监控目录失败: %w", err)
	}

	// 启动 goroutine 处理文件变化事件
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[HotReloader] 监控 %s 时发生 panic: %v", configPath, r)
			}
		}()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// 只处理配置文件相关的事件
				if hr.shouldProcessEvent(event, configPath) {
					log.Printf("[HotReloader] 检测到配置文件变化: %s", event.Name)
					hr.handleConfigChange(configPath)
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[HotReloader] 监控错误: %v", err)
			}
		}
	}()

	log.Printf("[HotReloader] 正在监控配置文件: %s", configPath)
	return nil
}

// shouldProcessEvent 判断是否应该处理该事件
func (hr *HotReloader) shouldProcessEvent(event fsnotify.Event, configPath string) bool {
	// 只处理写入事件
	if event.Op&fsnotify.Write != fsnotify.Write {
		return false
	}

	// 检查是否是目标配置文件
	absEventPath, _ := filepath.Abs(event.Name)
	return absEventPath == configPath
}

// handleConfigChange 处理配置文件变化
func (hr *HotReloader) handleConfigChange(configPath string) {
	// 防止频繁变化导致的重复处理
	time.Sleep(100 * time.Millisecond)

	// 读取配置并写入数据库
	if err := hr.loadAndUpdateConfig(configPath); err != nil {
		log.Printf("[HotReloader] 更新配置失败: %v", err)
		return
	}

	log.Printf("[HotReloader] 配置文件热更新成功: %s", configPath)

	// 调用重新加载处理函数
	if hr.reloadHandler != nil {
		go hr.reloadHandler()
	}
}

// loadAndUpdateConfig 从文件读取配置并写入数据库
func (hr *HotReloader) loadAndUpdateConfig(configPath string) error {
	// 读取配置文件
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置文件格式
	var config *conf.ConnectionStats
	switch filepath.Ext(configPath) {
	case ".json":
		if err := json.Unmarshal(configData, &config); err != nil {
			return fmt.Errorf("解析JSON配置失败: %w", err)
		}
	case ".yaml", ".yml":
		if err := yamlv3.Unmarshal(configData, &config); err != nil {
			return fmt.Errorf("解析YAML配置失败: %w", err)
		}
	default:
		return fmt.Errorf("不支持的配置文件格式: %s", filepath.Ext(configPath))
	}

	// 验证配置
	v := validator.NewConfigValidator()
	if err := v.Validate(config); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 写入数据库（权威数据源）
	if config.Id > 0 {
		// 更新现有配置
		if ok, msg := utils.UpdateForward(*config); !ok {
			return fmt.Errorf("更新配置失败: %s", msg)
		}
	} else {
		// 添加新配置
		if id := sql.AddForward(*config); id == 0 {
			return fmt.Errorf("添加配置到数据库失败")
		}
	}

	return nil
}

// Stop 停止配置热更新监控
func (hr *HotReloader) Stop() {
	for configPath, watcher := range hr.watchers {
		if err := watcher.Close(); err != nil {
			log.Printf("[HotReloader] 关闭监控器失败 %s: %v", configPath, err)
		}
	}
	hr.watchers = make(map[string]*fsnotify.Watcher)
	log.Println("[HotReloader] 配置文件监控已停止")
}

// ReloadNow 手动触发配置重新加载
func (hr *HotReloader) ReloadNow() {
	log.Println("[HotReloader] 手动触发配置重新加载")
	for _, configPath := range hr.configPaths {
		if err := hr.loadAndUpdateConfig(configPath); err != nil {
			log.Printf("[HotReloader] 手动重新加载失败 %s: %v", configPath, err)
		} else {
			log.Printf("[HotReloader] 手动重新加载成功 %s", configPath)
		}
	}
}

// GetWatchedConfigs 获取所有被监控的配置文件
func (hr *HotReloader) GetWatchedConfigs() []string {
	return append([]string(nil), hr.configPaths...)
}

// GetStats 获取监控统计信息
func (hr *HotReloader) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"watched_configs": len(hr.configPaths),
		"active_watchers": len(hr.watchers),
		"config_paths":    hr.configPaths,
	}
}
