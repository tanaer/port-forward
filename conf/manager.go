package conf

import (
	"context"
	"sync"
)

// ConfigSubscriber 配置更新订阅者接口
type ConfigSubscriber interface {
	OnConfigUpdate(forwardID int) error
}

// ConfigManager 配置管理器，替代全局变量
// 设计目标：减少全局状态，依赖注入替代全局变量
type ConfigManager struct {
	stopChan        chan string               // 替代 conf.Ch（保持双向以兼容旧代码）
	runningManagers map[string]contextManager // 替代 conf.Wg
	mu              sync.RWMutex

	// 封装全局配置，避免直接暴露
	webPort string
	webPass string
	debug   bool

	// 订阅者模式，支持配置热更新
	subscribers []ConfigSubscriber
}

// contextManager 跟踪单个转发实例的生命周期
type contextManager struct {
	id        int
	protocol  string
	localPort string
	ctx       context.Context
	cancel    context.CancelFunc
	wg        *sync.WaitGroup
}

// NewConfigManager 创建新的配置管理器
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		stopChan:        make(chan string),
		runningManagers: make(map[string]contextManager),
		webPort:         WebPort,
		webPass:         WebPass,
		debug:           Debug,
	}
}

// SendStopSignal 发送停止信号（兼容旧 conf.Ch 的使用方式）
// 渐进式迁移：保留旧接口，内部使用新的ConfigManager
// 注意：这是阻塞发送，与原始 conf.Ch <- key 语义一致
func (cm *ConfigManager) SendStopSignal(key string) error {
	// 修复：使用阻塞发送，确保信号送达receiver
	// 原始代码使用 conf.Ch <- key（无select），这里保持相同语义
	cm.stopChan <- key
	return nil
}

// StopChan 获取停止通道（双向以支持旧代码的传递模式）
func (cm *ConfigManager) StopChan() chan string {
	return cm.stopChan
}

// RegisterForward 注册转发实例（替代 conf.Wg.Add）
// 返回context用于协程管理
func (cm *ConfigManager) RegisterForward(id int, protocol, localPort string, wg *sync.WaitGroup) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.runningManagers[localPort+protocol] = contextManager{
		id:        id,
		protocol:  protocol,
		localPort: localPort,
		ctx:       ctx,
		cancel:    cancel,
		wg:        wg,
	}

	return ctx
}

// UnregisterForward 取消注册转发实例（替代协程结束）
func (cm *ConfigManager) UnregisterForward(protocol, localPort string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	key := localPort + protocol
	if manager, exists := cm.runningManagers[key]; exists {
		manager.cancel()
		delete(cm.runningManagers, key)
	}
}

// Shutdown 优雅关闭所有转发实例（替代 conf.Wg.Wait）
func (cm *ConfigManager) Shutdown() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 取消所有转发实例
	for key, manager := range cm.runningManagers {
		manager.cancel()
		delete(cm.runningManagers, key)
	}

	// 关闭停止通道
	close(cm.stopChan)
}

// 配置访问器方法（封装全局变量）
func (cm *ConfigManager) GetWebPort() string {
	return cm.webPort
}

func (cm *ConfigManager) GetWebPass() string {
	return cm.webPass
}

func (cm *ConfigManager) IsDebug() bool {
	return cm.debug
}

func (cm *ConfigManager) SetWebPort(port string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.webPort = port
}

func (cm *ConfigManager) SetWebPass(pass string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.webPass = pass
}

func (cm *ConfigManager) SetDebug(debug bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.debug = debug
}

// 订阅者模式支持配置热更新
func (cm *ConfigManager) Subscribe(subscriber ConfigSubscriber) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.subscribers = append(cm.subscribers, subscriber)
}

func (cm *ConfigManager) NotifySubscribers(forwardID int) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var err error
	for _, subscriber := range cm.subscribers {
		if e := subscriber.OnConfigUpdate(forwardID); e != nil {
			err = e
		}
	}
	return err
}
