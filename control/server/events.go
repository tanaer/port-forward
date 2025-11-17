package server

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"goForward/control/store"
)

// EventType 事件类型
type EventType string

const (
	// 节点事件
	EventNodeRegistered    EventType = "node_registered"
	EventNodeHealthChanged EventType = "node_health_changed"
	EventNodeIsolated      EventType = "node_isolated"
	EventNodeRecovered     EventType = "node_recovered"
	EventNodeOffline       EventType = "node_offline"
	EventNodeOnline        EventType = "node_online"

	// 配置事件
	EventConfigCreated        EventType = "config_created"
	EventConfigUpdated        EventType = "config_updated"
	EventConfigDeleted        EventType = "config_deleted"
	EventConfigRolledBack     EventType = "config_rolled_back"
	EventConfigVersionCreated EventType = "config_version_created"
	EventConfigApplied        EventType = "config_applied"
	EventConfigFailed         EventType = "config_failed"

	// 回滚任务事件
	EventRollbackTaskCreated EventType = "rollback_task_created"
	EventRollbackTaskPushed  EventType = "rollback_task_pushed"
)

// Event 事件结构
type Event struct {
	Type      EventType              `json:"type"`
	NodeID    string                 `json:"node_id,omitempty"`
	ConfigID  int32                  `json:"config_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// EventHandler 事件处理器接口
type EventHandler interface {
	Handle(event *Event) error
}

// EventHandlerFunc 事件处理器函数类型
type EventHandlerFunc func(event *Event) error

// Handle 实现 EventHandler 接口
func (f EventHandlerFunc) Handle(event *Event) error {
	return f(event)
}

// EventBus 事件总线
type EventBus struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
	queue    chan *Event
	workers  int
	wg       sync.WaitGroup
	stopped  bool
}

// NewEventBus 创建事件总线
func NewEventBus(workers int) *EventBus {
	if workers <= 0 {
		workers = 4 // 默认4个工作协程
	}

	bus := &EventBus{
		handlers: make(map[EventType][]EventHandler),
		queue:    make(chan *Event, 1000), // 缓冲1000个事件
		workers:  workers,
		stopped:  false,
	}

	// 启动工作协程
	for i := 0; i < workers; i++ {
		bus.wg.Add(1)
		go bus.worker()
	}

	return bus
}

// Subscribe 订阅事件
func (bus *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	bus.handlers[eventType] = append(bus.handlers[eventType], handler)
	log.Printf("[事件总线] 订阅事件: %s, 当前处理器数量: %d", eventType, len(bus.handlers[eventType]))
}

// SubscribeFunc 订阅事件（函数形式）
func (bus *EventBus) SubscribeFunc(eventType EventType, handler func(event *Event) error) {
	bus.Subscribe(eventType, EventHandlerFunc(handler))
}

// Publish 发布事件（异步）
func (bus *EventBus) Publish(event *Event) {
	if bus.stopped {
		log.Printf("[事件总线] 警告: 事件总线已停止，忽略事件: %s", event.Type)
		return
	}

	// 设置时间戳
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	// 异步发送到队列
	select {
	case bus.queue <- event:
		// 成功入队
	default:
		// 队列满，记录警告
		log.Printf("[事件总线] 警告: 事件队列已满，丢弃事件: %s", event.Type)
	}
}

// PublishSync 发布事件（同步）
func (bus *EventBus) PublishSync(event *Event) error {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	bus.mu.RLock()
	handlers := bus.handlers[event.Type]
	bus.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler.Handle(event); err != nil {
			log.Printf("[事件总线] 处理事件失败: %s, 错误: %v", event.Type, err)
			return err
		}
	}

	return nil
}

// worker 工作协程
func (bus *EventBus) worker() {
	defer bus.wg.Done()

	for event := range bus.queue {
		bus.mu.RLock()
		handlers := bus.handlers[event.Type]
		bus.mu.RUnlock()

		// 执行所有订阅的处理器
		for _, handler := range handlers {
			if err := handler.Handle(event); err != nil {
				log.Printf("[事件总线] 处理事件失败: %s, 错误: %v", event.Type, err)
			}
		}
	}
}

// Stop 停止事件总线
func (bus *EventBus) Stop() {
	bus.mu.Lock()
	if bus.stopped {
		bus.mu.Unlock()
		return
	}
	bus.stopped = true
	bus.mu.Unlock()

	close(bus.queue)
	bus.wg.Wait()
	log.Println("[事件总线] 已停止")
}

// GetQueueSize 获取队列大小
func (bus *EventBus) GetQueueSize() int {
	return len(bus.queue)
}

// --- 内置事件处理器 ---

// LogEventHandler 日志记录处理器
type LogEventHandler struct{}

// Handle 处理事件
func (h *LogEventHandler) Handle(event *Event) error {
	eventJSON, _ := json.Marshal(event)
	log.Printf("[事件] %s: %s", event.Type, string(eventJSON))
	return nil
}

// NewLogEventHandler 创建日志处理器
func NewLogEventHandler() *LogEventHandler {
	return &LogEventHandler{}
}

// WebSocketEventHandler WebSocket 通知处理器
type WebSocketEventHandler struct {
	wsHub WebSocketHub
}

// Handle 处理事件
func (h *WebSocketEventHandler) Handle(event *Event) error {
	if h.wsHub == nil {
		return nil
	}

	// 构建 WebSocket 消息
	message := map[string]interface{}{
		"type":      "event",
		"event":     event.Type,
		"node_id":   event.NodeID,
		"config_id": event.ConfigID,
		"data":      event.Data,
		"timestamp": event.Timestamp,
	}

	// 广播到所有 WebSocket 客户端
	h.wsHub.Broadcast(message)
	return nil
}

// NewWebSocketEventHandler 创建 WebSocket 处理器
func NewWebSocketEventHandler(wsHub WebSocketHub) *WebSocketEventHandler {
	return &WebSocketEventHandler{wsHub: wsHub}
}

// DatabaseEventHandler 数据库持久化处理器
type DatabaseEventHandler struct {
	store StoreInterface
}

// StoreInterface 为事件存储定义接口，避免直接依赖具体实现
type StoreInterface interface {
	NodeEventDAO() *store.NodeEventDAO
}

// Handle 处理事件
func (h *DatabaseEventHandler) Handle(event *Event) error {
	if h.store == nil {
		return nil
	}

	// 只持久化节点相关事件
	if event.NodeID == "" {
		return nil
	}

	// 序列化事件数据
	eventData, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	// 保存到数据库
	return h.store.NodeEventDAO().CreateEvent(event.NodeID, string(event.Type), string(eventData))
}

// NewDatabaseEventHandler 创建数据库处理器
func NewDatabaseEventHandler(store StoreInterface) *DatabaseEventHandler {
	return &DatabaseEventHandler{store: store}
}
