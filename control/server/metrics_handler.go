package server

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goForward/control/metrics"
)

// StartMetricsServer 启动 Prometheus metrics 服务器
func StartMetricsServer(port string) error {
	// 初始化指标记录器
	if err := metrics.InitRecorder(); err != nil {
		return err
	}

	// 注册 /metrics 端点
	http.Handle("/metrics", promhttp.Handler())

	log.Printf("[指标服务] 启动 Prometheus metrics 服务器: %s", port)

	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("[指标服务] metrics 服务器错误: %v", err)
		}
	}()

	return nil
}

// GetMetricsRecorder 获取指标记录器
func GetMetricsRecorder() *metrics.Recorder {
	return metrics.GetRecorder()
}
