package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// RollbackMetrics 回滚系统的 Prometheus 指标集合
type RollbackMetrics struct {
	// 任务总数计数器
	TasksTotal prometheus.Counter

	// 任务执行时长直方图
	TaskDurationSeconds prometheus.Histogram

	// 重试次数分布直方图
	RetryCount prometheus.Histogram

	// 失败任务计数器
	FailedTotal prometheus.Counter

	// 超时重置计数器
	TimeoutResetsTotal prometheus.Counter

	// 处于各状态的任务数量仪表
	TasksStateGauge prometheus.GaugeVec

	// 处理中的任务数
	ProcessingTasksGauge prometheus.Gauge
}

// grpcMetrics gRPC 服务的 Prometheus 指标集合
type GRPCMetrics struct {
	// RPC 调用总数
	HandledTotal prometheus.CounterVec

	// RPC 执行时间
	HandlingSeconds prometheus.HistogramVec

	// 接收的消息数
	MsgReceived prometheus.CounterVec

	// 发送的消息数
	MsgSent prometheus.CounterVec
}

// NewRollbackMetrics 创建回滚系统指标
func NewRollbackMetrics() (*RollbackMetrics, error) {
	taskTotal := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "goforward",
			Subsystem: "rollback",
			Name:      "tasks_total",
			Help:      "Total number of rollback tasks created",
		},
	)

	taskDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "goforward",
			Subsystem: "rollback",
			Name:      "task_duration_seconds",
			Help:      "Rollback task execution duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
	)

	retryCount := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "goforward",
			Subsystem: "rollback",
			Name:      "retry_count",
			Help:      "Distribution of retry counts",
			Buckets:   []float64{0, 1, 2, 3, 4, 5, 10, 20},
		},
	)

	failedTotal := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "goforward",
			Subsystem: "rollback",
			Name:      "failed_total",
			Help:      "Total number of failed rollback tasks",
		},
	)

	timeoutResets := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "goforward",
			Subsystem: "rollback",
			Name:      "timeout_resets_total",
			Help:      "Total number of processing tasks reset due to timeout",
		},
	)

	taskStateGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "goforward",
			Subsystem: "rollback",
			Name:      "tasks_state",
			Help:      "Number of tasks in each state",
		},
		[]string{"state"},
	)

	processingGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "goforward",
			Subsystem: "rollback",
			Name:      "processing_tasks",
			Help:      "Number of tasks currently in processing state",
		},
	)

	// 注册所有指标
	registerer := prometheus.DefaultRegisterer
	if err := registerer.Register(taskTotal); err != nil {
		return nil, fmt.Errorf("register taskTotal failed: %v", err)
	}
	if err := registerer.Register(taskDuration); err != nil {
		return nil, fmt.Errorf("register taskDuration failed: %v", err)
	}
	if err := registerer.Register(retryCount); err != nil {
		return nil, fmt.Errorf("register retryCount failed: %v", err)
	}
	if err := registerer.Register(failedTotal); err != nil {
		return nil, fmt.Errorf("register failedTotal failed: %v", err)
	}
	if err := registerer.Register(timeoutResets); err != nil {
		return nil, fmt.Errorf("register timeoutResets failed: %v", err)
	}
	if err := registerer.Register(taskStateGauge); err != nil {
		return nil, fmt.Errorf("register taskStateGauge failed: %v", err)
	}
	if err := registerer.Register(processingGauge); err != nil {
		return nil, fmt.Errorf("register processingGauge failed: %v", err)
	}

	return &RollbackMetrics{
		TasksTotal:          taskTotal,
		TaskDurationSeconds: taskDuration,
		RetryCount:          retryCount,
		FailedTotal:         failedTotal,
		TimeoutResetsTotal:  timeoutResets,
		TasksStateGauge:     *taskStateGauge,
		ProcessingTasksGauge: processingGauge,
	}, nil
}

// NewGRPCMetrics 创建 gRPC 指标
func NewGRPCMetrics() (*GRPCMetrics, error) {
	handledTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "goforward",
			Subsystem: "grpc",
			Name:      "server_handled_total",
			Help:      "Total number of completed RPC calls",
		},
		[]string{"method", "code"},
	)

	handlingSeconds := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "goforward",
			Subsystem: "grpc",
			Name:      "server_handling_seconds",
			Help:      "Histogram of response latency (seconds)",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	msgReceived := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "goforward",
			Subsystem: "grpc",
			Name:      "server_msg_received_total",
			Help:      "Total number of RPC messages received",
		},
		[]string{"method"},
	)

	msgSent := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "goforward",
			Subsystem: "grpc",
			Name:      "server_msg_sent_total",
			Help:      "Total number of RPC messages sent",
		},
		[]string{"method"},
	)

	// 注册所有指标
	registerer := prometheus.DefaultRegisterer
	if err := registerer.Register(handledTotal); err != nil {
		return nil, fmt.Errorf("register handledTotal failed: %v", err)
	}
	if err := registerer.Register(handlingSeconds); err != nil {
		return nil, fmt.Errorf("register handlingSeconds failed: %v", err)
	}
	if err := registerer.Register(msgReceived); err != nil {
		return nil, fmt.Errorf("register msgReceived failed: %v", err)
	}
	if err := registerer.Register(msgSent); err != nil {
		return nil, fmt.Errorf("register msgSent failed: %v", err)
	}

	return &GRPCMetrics{
		HandledTotal:     *handledTotal,
		HandlingSeconds:  *handlingSeconds,
		MsgReceived:      *msgReceived,
		MsgSent:          *msgSent,
	}, nil
}

// RecordTaskCreated 记录任务创建
func (m *RollbackMetrics) RecordTaskCreated() {
	m.TasksTotal.Inc()
}

// RecordTaskSuccess 记录任务成功
func (m *RollbackMetrics) RecordTaskSuccess(durationSeconds float64) {
	m.TaskDurationSeconds.Observe(durationSeconds)
	m.TasksStateGauge.WithLabelValues("completed").Inc()
}

// RecordTaskFailed 记录任务失败
func (m *RollbackMetrics) RecordTaskFailed(retryCount int) {
	m.FailedTotal.Inc()
	m.RetryCount.Observe(float64(retryCount))
	m.TasksStateGauge.WithLabelValues("failed").Inc()
}

// RecordTimeout 记录超时重置
func (m *RollbackMetrics) RecordTimeout() {
	m.TimeoutResetsTotal.Inc()
}

// SetProcessingTaskCount 设置处理中的任务数
func (m *RollbackMetrics) SetProcessingTaskCount(count int64) {
	m.ProcessingTasksGauge.Set(float64(count))
}

// SetTaskStateCount 设置各状态的任务数
func (m *RollbackMetrics) SetTaskStateCount(state string, count int64) {
	m.TasksStateGauge.WithLabelValues(state).Set(float64(count))
}
