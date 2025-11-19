package server

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goForward/control/metrics"
)

// StartMetricsServer 启动 Prometheus metrics 服务器
// 注意: 必须先调用 metrics.InitRecorder() 再调用此函数
func StartMetricsServer(port string) error {
	// 注册 /metrics 端点
	http.Handle("/metrics", promhttp.Handler())

	log.Printf("[指标服务] 启动 Prometheus metrics 服务器，监听端口: :%s", port)

	// 在 goroutine 中启动 HTTP 服务器
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("[指标服务] Prometheus HTTP 服务器错误: %v", err)
		}
	}()

	return nil
}

// GetMetricsRecorder 获取指标记录器
func GetMetricsRecorder() *metrics.Recorder {
	return metrics.GetRecorder()
}
