package server

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"goForward/control/store"
)

// VersionManager 版本管理器 - 负责版本控制、快照、推送、回滚
type VersionManager struct {
	versionDAO *store.ConfigVersionDAO
	eventBus   *EventBus
	mu         sync.RWMutex

	// 缓存最后推送状态
	pushStatus map[int32]PushStatus // configID -> PushStatus
}

// PushStatus 配置推送状态
type PushStatus struct {
	ConfigID       int32
	CurrentVersion int32
	LastPushTime   int64
	LastPushResult string // "success", "pending", "failed"
	NodesPushed    map[string]bool
}

// NewVersionManager 创建版本管理器
func NewVersionManager(versionDAO *store.ConfigVersionDAO, eventBus *EventBus) *VersionManager {
	return &VersionManager{
		versionDAO: versionDAO,
		eventBus:   eventBus,
		pushStatus: make(map[int32]PushStatus),
	}
}

// CaptureVersion 捕获配置版本快照
// 在每次配置变更时调用，自动生成版本号并保存快照
func (vm *VersionManager) CaptureVersion(
	configID int32,
	configJSON string,
	changeType string, // "create", "update", "restore"
	changeSummary string,
	createdBy string,
) (*store.ConfigVersionRecord, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// 获取最新版本号并递增
	latestVersion, err := vm.versionDAO.GetLatestVersion(configID)
	if err != nil {
		log.Printf("[VersionManager] 查询最新版本号失败: %v\n", err)
		return nil, err
	}

	newVersion := latestVersion + 1

	// 创建版本记录
	record, err := vm.versionDAO.CreateVersion(
		configID,
		newVersion,
		configJSON,
		changeType,
		changeSummary,
		createdBy,
	)
	if err != nil {
		log.Printf("[VersionManager] 创建版本快照失败: %v\n", err)
		return nil, err
	}

	// 触发版本创建事件
	vm.eventBus.Publish(&Event{
		Type:     EventConfigVersionCreated,
		ConfigID: configID,
		Data: map[string]interface{}{
			"version":        newVersion,
			"change_type":    changeType,
			"change_summary": changeSummary,
			"created_by":     createdBy,
		},
		Timestamp: time.Now().Unix(),
	})

	log.Printf("[VersionManager] 配置版本已捕获: config_id=%d, version=%d, type=%s\n", configID, newVersion, changeType)
	return record, nil
}

// GetVersionHistory 获取配置版本历史
func (vm *VersionManager) GetVersionHistory(configID int32, limit, offset int) ([]*store.ConfigVersionRecord, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.versionDAO.GetVersionHistory(configID, limit, offset)
}

// GetVersion 获取指定版本的配置
func (vm *VersionManager) GetVersion(configID int32, version int32) (*store.ConfigVersionRecord, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.versionDAO.GetVersionByConfigIDAndVersion(configID, version)
}

// GetVersionDiff 获取两个版本之间的差异
func (vm *VersionManager) GetVersionDiff(configID int32, fromVersion, toVersion int32) (*store.VersionDiff, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.versionDAO.GetVersionDiff(configID, fromVersion, toVersion)
}

// RecordPushStart 记录配置推送开始
func (vm *VersionManager) RecordPushStart(configID int32, nodeIDs []string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	latestVersion, err := vm.versionDAO.GetLatestVersion(configID)
	if err != nil {
		log.Printf("[VersionManager] 查询最新版本号失败: %v\n", err)
		return
	}

	nodePushed := make(map[string]bool)
	for _, nodeID := range nodeIDs {
		nodePushed[nodeID] = false
	}

	vm.pushStatus[configID] = PushStatus{
		ConfigID:       configID,
		CurrentVersion: latestVersion,
		LastPushTime:   time.Now().Unix(),
		LastPushResult: "pending",
		NodesPushed:    nodePushed,
	}
}

// RecordPushResult 记录推送结果
func (vm *VersionManager) RecordPushResult(configID int32, nodeID string, success bool, err string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	status, exists := vm.pushStatus[configID]
	if !exists {
		return
	}

	status.NodesPushed[nodeID] = success

	eventType := EventConfigApplied
	if !success {
		eventType = EventConfigFailed
	}

	vm.eventBus.Publish(&Event{
		Type:     eventType,
		ConfigID: configID,
		NodeID:   nodeID,
		Data: map[string]interface{}{
			"version": status.CurrentVersion,
			"error":   err,
			"success": success,
		},
		Timestamp: time.Now().Unix(),
	})
}

// GetPushStatus 获取推送状态
func (vm *VersionManager) GetPushStatus(configID int32) *PushStatus {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if status, exists := vm.pushStatus[configID]; exists {
		return &status
	}
	return nil
}

// CleanupOldVersions 清理旧版本（只保留最近N个）
func (vm *VersionManager) CleanupOldVersions(configID int32, keepCount int) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	return vm.versionDAO.DeleteVersionsOlderThan(configID, keepCount)
}

// GetConfigSnapshot 获取配置快照（用于与当前配置对比）
func (vm *VersionManager) GetConfigSnapshot(configID int32, version int32) (map[string]interface{}, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	record, err := vm.versionDAO.GetVersionByConfigIDAndVersion(configID, version)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("版本不存在: version=%d", version)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(record.ConfigSnapshot), &snapshot); err != nil {
		return nil, fmt.Errorf("解析快照失败: %v", err)
	}

	return snapshot, nil
}

// ComputeConfigHash 计算配置哈希值（用于检测变更）
func (vm *VersionManager) ComputeConfigHash(configJSON string) string {
	// 简化实现：计算长度作为示例
	// 实际应该使用SHA256等算法
	return fmt.Sprintf("%d_%d", len(configJSON), time.Now().Unix())
}

// ValidateVersionChain 验证版本链完整性
func (vm *VersionManager) ValidateVersionChain(configID int32) (bool, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	count, err := vm.versionDAO.CountVersions(configID)
	if err != nil {
		return false, err
	}

	if count == 0 {
		return true, nil
	}

	// 获取所有版本并验证版本号连续性
	history, err := vm.versionDAO.GetVersionHistory(configID, count, 0)
	if err != nil {
		return false, err
	}

	// history是从高版本到低版本排序的
	for i := 0; i < len(history)-1; i++ {
		if history[i].Version != history[i+1].Version+1 {
			return false, fmt.Errorf("版本链不连续: %d -> %d", history[i].Version, history[i+1].Version)
		}
	}

	return true, nil
}

// GetVersionStats 获取版本统计信息
func (vm *VersionManager) GetVersionStats(configID int32) (map[string]interface{}, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	count, err := vm.versionDAO.CountVersions(configID)
	if err != nil {
		return nil, err
	}

	latestVersion, err := vm.versionDAO.GetLatestVersion(configID)
	if err != nil {
		return nil, err
	}

	status := vm.pushStatus[configID]
	failedCount := 0
	successCount := 0
	for _, pushed := range status.NodesPushed {
		if pushed {
			successCount++
		} else {
			failedCount++
		}
	}

	return map[string]interface{}{
		"total_versions": count,
		"latest_version": latestVersion,
		"push_status":    status.LastPushResult,
		"nodes_success":  successCount,
		"nodes_failed":   failedCount,
		"last_push_time": status.LastPushTime,
	}, nil
}
