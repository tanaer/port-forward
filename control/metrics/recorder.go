package metrics

import (
	"sync"
	"time"
)

// Recorder 指标记录器，用于统一管理 Prometheus 指标
type Recorder struct {
	mu              sync.RWMutex
	rollbackMetrics *RollbackMetrics
	grpcMetrics     *GRPCMetrics

	// 任务执行时间记录
	taskTimings map[int64]time.Time
}

// GlobalRecorder 全局指标记录器实例
var GlobalRecorder *Recorder

// InitRecorder 初始化全局指标记录器
func InitRecorder() error {
	rollbackMetrics, err := NewRollbackMetrics()
	if err != nil {
		return err
	}

	grpcMetrics, err := NewGRPCMetrics()
	if err != nil {
		return err
	}

	GlobalRecorder = &Recorder{
		rollbackMetrics: rollbackMetrics,
		grpcMetrics:     grpcMetrics,
		taskTimings:     make(map[int64]time.Time),
	}

	return nil
}

// RecordRollbackTaskStart 记录回滚任务开始
func (r *Recorder) RecordRollbackTaskStart(taskID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.taskTimings[taskID] = time.Now()
	r.rollbackMetrics.RecordTaskCreated()
}

// RecordRollbackTaskSuccess 记录回滚任务成功
func (r *Recorder) RecordRollbackTaskSuccess(taskID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if startTime, exists := r.taskTimings[taskID]; exists {
		duration := time.Since(startTime).Seconds()
		r.rollbackMetrics.RecordTaskSuccess(duration)
		delete(r.taskTimings, taskID)
	}
}

// RecordRollbackTaskFailed 记录回滚任务失败
func (r *Recorder) RecordRollbackTaskFailed(taskID int64, retryCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rollbackMetrics.RecordTaskFailed(retryCount)
	delete(r.taskTimings, taskID)
}

// RecordProcessingTimeout 记录 Processing 超时
func (r *Recorder) RecordProcessingTimeout(taskID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rollbackMetrics.RecordTimeout()
	delete(r.taskTimings, taskID)
}

// SetProcessingTaskCount 设置处理中的任务数量
func (r *Recorder) SetProcessingTaskCount(count int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.rollbackMetrics.SetProcessingTaskCount(count)
}

// SetTaskStateCount 设置各状态的任务数量
func (r *Recorder) SetTaskStateCount(state string, count int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.rollbackMetrics.SetTaskStateCount(state, count)
}

// RecordGRPCHandled 记录 gRPC 调用完成
func (r *Recorder) RecordGRPCHandled(method string, code string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.grpcMetrics.HandledTotal.WithLabelValues(method, code).Inc()
}

// RecordGRPCDuration 记录 gRPC 调用耗时
func (r *Recorder) RecordGRPCDuration(method string, durationSeconds float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.grpcMetrics.HandlingSeconds.WithLabelValues(method).Observe(durationSeconds)
}

// RecordGRPCMsgReceived 记录接收的 gRPC 消息
func (r *Recorder) RecordGRPCMsgReceived(method string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.grpcMetrics.MsgReceived.WithLabelValues(method).Inc()
}

// RecordGRPCMsgSent 记录发送的 gRPC 消息
func (r *Recorder) RecordGRPCMsgSent(method string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.grpcMetrics.MsgSent.WithLabelValues(method).Inc()
}

// GetRecorder 获取全局指标记录器
func GetRecorder() *Recorder {
	return GlobalRecorder
}
