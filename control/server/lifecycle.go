package server

import (
	"fmt"
	"log"
	"sync"
	"time"

	pb "goForward/proto"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusWarning  HealthStatus = "warning"
	HealthStatusCritical HealthStatus = "critical"
	HealthStatusOffline  HealthStatus = "offline"
	HealthStatusUnknown  HealthStatus = "unknown"
)

// HealthThresholds 健康检查阈值
type HealthThresholds struct {
	CPUWarning       int64         // CPU 警告阈值（百分比）
	CPUCritical      int64         // CPU 严重阈值
	MemoryWarning    int64         // 内存警告阈值
	MemoryCritical   int64         // 内存严重阈值
	DiskWarning      int64         // 磁盘警告阈值
	DiskCritical     int64         // 磁盘严重阈值
	HeartbeatTimeout time.Duration // 心跳超时时间
	IsolateTimeout   time.Duration // 自动隔离超时时间
}

// DefaultHealthThresholds 默认健康检查阈值
var DefaultHealthThresholds = HealthThresholds{
	CPUWarning:       80,
	CPUCritical:      90,
	MemoryWarning:    80,
	MemoryCritical:   90,
	DiskWarning:      85,
	DiskCritical:     95,
	HeartbeatTimeout: 90 * time.Second,
	IsolateTimeout:   5 * time.Minute,
}

// LifecycleManager 节点生命周期管理器
type LifecycleManager struct {
	nodeRegistry *NodeRegistry
	eventBus     *EventBus
	thresholds   HealthThresholds
	mu           sync.RWMutex

	// 节点失败计数（用于自动隔离）
	failureCount map[string]int
	// 节点离线时间
	offlineTime map[string]time.Time
}

// NewLifecycleManager 创建生命周期管理器
func NewLifecycleManager(nodeRegistry *NodeRegistry, eventBus *EventBus) *LifecycleManager {
	return &LifecycleManager{
		nodeRegistry: nodeRegistry,
		eventBus:     eventBus,
		thresholds:   DefaultHealthThresholds,
		failureCount: make(map[string]int),
		offlineTime:  make(map[string]time.Time),
	}
}

// SetThresholds 设置健康检查阈值
func (lm *LifecycleManager) SetThresholds(thresholds HealthThresholds) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.thresholds = thresholds
}

// OnNodeRegistered 节点注册事件处理
func (lm *LifecycleManager) OnNodeRegistered(nodeID string, nodeInfo *pb.NodeInfo) {
	log.Printf("[生命周期] 节点注册: %s (%s)", nodeID, nodeInfo.Hostname)

	// 发布节点注册事件
	lm.eventBus.Publish(&Event{
		Type:   EventNodeRegistered,
		NodeID: nodeID,
		Data: map[string]interface{}{
			"hostname":   nodeInfo.Hostname,
			"ip_address": nodeInfo.IpAddress,
			"version":    nodeInfo.Version,
		},
	})

	// 清除失败计数和离线时间
	lm.mu.Lock()
	delete(lm.failureCount, nodeID)
	delete(lm.offlineTime, nodeID)
	lm.mu.Unlock()
}

// CheckNodeHealth 检查节点健康状态
func (lm *LifecycleManager) CheckNodeHealth(nodeID string, health *pb.NodeHealth) HealthStatus {
	if health == nil {
		return HealthStatusUnknown
	}

	lm.mu.RLock()
	thresholds := lm.thresholds
	lm.mu.RUnlock()

	// 计算健康状态
	criticalCount := 0
	warningCount := 0

	// 检查 CPU
	if health.CpuPercent >= thresholds.CPUCritical {
		criticalCount++
	} else if health.CpuPercent >= thresholds.CPUWarning {
		warningCount++
	}

	// 检查内存
	if health.MemoryPercent >= thresholds.MemoryCritical {
		criticalCount++
	} else if health.MemoryPercent >= thresholds.MemoryWarning {
		warningCount++
	}

	// 检查磁盘
	if health.DiskPercent >= thresholds.DiskCritical {
		criticalCount++
	} else if health.DiskPercent >= thresholds.DiskWarning {
		warningCount++
	}

	// 判断健康状态
	if criticalCount >= 2 {
		return HealthStatusCritical
	} else if criticalCount >= 1 || warningCount >= 2 {
		return HealthStatusWarning
	}

	return HealthStatusHealthy
}

// UpdateNodeHealth 更新节点健康状态（用于心跳处理）
func (lm *LifecycleManager) UpdateNodeHealth(nodeID string, health *pb.NodeHealth) {
	// 获取节点注册表中的当前状态
	lm.nodeRegistry.mu.RLock()
	nodeInfo, exists := lm.nodeRegistry.nodes[nodeID]
	lm.nodeRegistry.mu.RUnlock()

	if !exists {
		log.Printf("[生命周期] 节点 %s 不存在，跳过健康检查", nodeID)
		return
	}

	// 检查节点是否从离线状态恢复
	if nodeInfo.Status == "offline" {
		log.Printf("[生命周期] 检测到节点 %s 从离线状态恢复", nodeID)
		lm.OnNodeOnline(nodeID)
	}

	// 计算健康状态（使用之前保存的健康状态，如果没有则默认为 unknown）
	var oldStatus HealthStatus = HealthStatusUnknown
	if nodeInfo.Health != nil {
		// 从节点状态字符串转换
		oldStatus = HealthStatus(nodeInfo.Status)
	}

	newStatus := lm.CheckNodeHealth(nodeID, health)

	// 更新节点注册表中的健康状态
	lm.nodeRegistry.mu.Lock()
	lm.nodeRegistry.nodes[nodeID].Health = health
	lm.nodeRegistry.nodes[nodeID].Status = string(newStatus)
	lm.nodeRegistry.mu.Unlock()

	// 处理健康状态变化
	lm.OnHealthChanged(nodeID, oldStatus, newStatus, health)
}

// OnHealthChanged 健康状态变化处理
func (lm *LifecycleManager) OnHealthChanged(nodeID string, oldStatus, newStatus HealthStatus, health *pb.NodeHealth) {
	if oldStatus == newStatus {
		return
	}

	log.Printf("[生命周期] 节点 %s 健康状态变化: %s -> %s", nodeID, oldStatus, newStatus)

	// 发布健康状态变化事件
	lm.eventBus.Publish(&Event{
		Type:   EventNodeHealthChanged,
		NodeID: nodeID,
		Data: map[string]interface{}{
			"old_status":         string(oldStatus),
			"new_status":         string(newStatus),
			"cpu_percent":        health.CpuPercent,
			"memory_percent":     health.MemoryPercent,
			"disk_percent":       health.DiskPercent,
			"active_connections": health.ActiveConnections,
		},
		Timestamp: time.Now().Unix(),
	})

	// 更新失败计数
	lm.mu.Lock()
	if newStatus == HealthStatusCritical || newStatus == HealthStatusOffline {
		lm.failureCount[nodeID]++
		log.Printf("[生命周期] 节点 %s 失败计数: %d", nodeID, lm.failureCount[nodeID])
	} else if newStatus == HealthStatusHealthy && oldStatus != HealthStatusUnknown {
		// 恢复健康，清除失败计数
		delete(lm.failureCount, nodeID)
		log.Printf("[生命周期] 节点 %s 健康状态恢复，清除失败计数", nodeID)
	}
	lm.mu.Unlock()

	// 检查是否需要自动隔离
	lm.checkAutoIsolate(nodeID)
}

// OnNodeOffline 节点离线处理
func (lm *LifecycleManager) OnNodeOffline(nodeID string) {
	log.Printf("[生命周期] 节点离线: %s", nodeID)

	// 记录离线时间
	lm.mu.Lock()
	if _, exists := lm.offlineTime[nodeID]; !exists {
		lm.offlineTime[nodeID] = time.Now()
	}
	lm.mu.Unlock()

	// 发布节点离线事件
	lm.eventBus.Publish(&Event{
		Type:   EventNodeOffline,
		NodeID: nodeID,
		Data: map[string]interface{}{
			"offline_time": time.Now().Unix(),
		},
	})

	// 检查是否需要自动隔离
	lm.checkAutoIsolate(nodeID)
}

// OnNodeOnline 节点上线处理
func (lm *LifecycleManager) OnNodeOnline(nodeID string) {
	log.Printf("[生命周期] 节点上线: %s", nodeID)

	// 清除离线时间和失败计数
	lm.mu.Lock()
	delete(lm.offlineTime, nodeID)
	delete(lm.failureCount, nodeID)
	lm.mu.Unlock()

	// 发布节点上线事件
	lm.eventBus.Publish(&Event{
		Type:   EventNodeOnline,
		NodeID: nodeID,
		Data: map[string]interface{}{
			"online_time": time.Now().Unix(),
		},
	})
}

// checkAutoIsolate 检查是否需要自动隔离
func (lm *LifecycleManager) checkAutoIsolate(nodeID string) {
	lm.mu.RLock()
	failureCount := lm.failureCount[nodeID]
	offlineTime, isOffline := lm.offlineTime[nodeID]
	thresholds := lm.thresholds
	lm.mu.RUnlock()

	shouldIsolate := false
	reason := ""

	// 检查失败次数
	if failureCount >= 3 {
		shouldIsolate = true
		reason = fmt.Sprintf("连续 %d 次健康检查失败", failureCount)
	}

	// 检查离线时长
	if isOffline && time.Since(offlineTime) > thresholds.IsolateTimeout {
		shouldIsolate = true
		reason = fmt.Sprintf("离线超过 %v", thresholds.IsolateTimeout)
	}

	if shouldIsolate {
		log.Printf("[生命周期] 自动隔离节点 %s: %s", nodeID, reason)
		lm.IsolateNode(nodeID, reason, true)
	}
}

// IsolateNode 隔离节点
func (lm *LifecycleManager) IsolateNode(nodeID string, reason string, auto bool) error {
	lm.nodeRegistry.mu.Lock()
	node, exists := lm.nodeRegistry.nodes[nodeID]
	if !exists {
		lm.nodeRegistry.mu.Unlock()
		return fmt.Errorf("节点不存在: %s", nodeID)
	}

	// 检查是否已经隔离
	if node.Status == "isolated" {
		lm.nodeRegistry.mu.Unlock()
		return fmt.Errorf("节点已被隔离: %s", nodeID)
	}

	// 标记为隔离状态
	node.Status = "isolated"
	lm.nodeRegistry.mu.Unlock()

	log.Printf("[生命周期] 节点已隔离: %s, 原因: %s, 自动: %v", nodeID, reason, auto)

	// 发布节点隔离事件
	lm.eventBus.Publish(&Event{
		Type:   EventNodeIsolated,
		NodeID: nodeID,
		Data: map[string]interface{}{
			"reason":    reason,
			"auto":      auto,
			"timestamp": time.Now().Unix(),
		},
	})

	return nil
}

// RecoverNode 恢复节点
func (lm *LifecycleManager) RecoverNode(nodeID string) error {
	lm.nodeRegistry.mu.Lock()
	node, exists := lm.nodeRegistry.nodes[nodeID]
	if !exists {
		lm.nodeRegistry.mu.Unlock()
		return fmt.Errorf("节点不存在: %s", nodeID)
	}

	// 检查是否处于隔离状态
	if node.Status != "isolated" {
		lm.nodeRegistry.mu.Unlock()
		return fmt.Errorf("节点未被隔离: %s", nodeID)
	}

	// 恢复为活动状态
	node.Status = "active"
	lm.nodeRegistry.mu.Unlock()

	// 清除失败计数和离线时间
	lm.mu.Lock()
	delete(lm.failureCount, nodeID)
	delete(lm.offlineTime, nodeID)
	lm.mu.Unlock()

	log.Printf("[生命周期] 节点已恢复: %s", nodeID)

	// 发布节点恢复事件
	lm.eventBus.Publish(&Event{
		Type:   EventNodeRecovered,
		NodeID: nodeID,
		Data: map[string]interface{}{
			"timestamp": time.Now().Unix(),
		},
	})

	return nil
}

// GetNodeHealthStatus 获取节点健康状态
func (lm *LifecycleManager) GetNodeHealthStatus(nodeID string) (HealthStatus, error) {
	lm.nodeRegistry.mu.RLock()
	node, exists := lm.nodeRegistry.nodes[nodeID]
	lm.nodeRegistry.mu.RUnlock()

	if !exists {
		return HealthStatusUnknown, fmt.Errorf("节点不存在: %s", nodeID)
	}

	// 检查是否隔离
	if node.Status == "isolated" {
		return HealthStatusOffline, nil
	}

	// 检查是否离线
	if node.Status == "inactive" {
		return HealthStatusOffline, nil
	}

	// 检查健康指标
	return lm.CheckNodeHealth(nodeID, node.Health), nil
}

// IsNodeIsolated 检查节点是否被隔离
func (lm *LifecycleManager) IsNodeIsolated(nodeID string) bool {
	lm.nodeRegistry.mu.RLock()
	defer lm.nodeRegistry.mu.RUnlock()

	node, exists := lm.nodeRegistry.nodes[nodeID]
	if !exists {
		return false
	}

	return node.Status == "isolated"
}

// GetFailureCount 获取节点失败计数
func (lm *LifecycleManager) GetFailureCount(nodeID string) int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.failureCount[nodeID]
}

// GetOfflineTime 获取节点离线时间
func (lm *LifecycleManager) GetOfflineTime(nodeID string) (time.Time, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	t, exists := lm.offlineTime[nodeID]
	return t, exists
}
