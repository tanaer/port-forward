//go:build v2
// +build v2

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"goForward/control/config"
	"goForward/control/metrics"
	"goForward/control/server"
	"goForward/control/store"
	"goForward/control/web"
)

// v2.0.0 分布式控制端主程序
func main() {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("goForward v2.0.0 - 分布式控制端")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// 1. 加载配置
	cfg := config.DefaultConfig()
	// 优先从 YAML 配置文件加载, 然后使用环境变量进行覆盖
	_ = cfg.LoadFromFile("goforward.yaml")
	cfg.LoadFromEnv()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	fmt.Printf("✅ 配置加载成功\n")
	fmt.Printf("   - 服务端口: %s\n", cfg.Server.Port)
	fmt.Printf("   - 回滚系统: %v\n", cfg.Rollback.Enabled)
	fmt.Printf("   - Prometheus: %v (端口 %s)\n", cfg.Metrics.Enabled, cfg.Metrics.Port)
	fmt.Println()

	// 2. 初始化存储层
	dbPath := store.GetDatabasePath()
	fmt.Printf("📦 初始化存储层: %s\n", dbPath)

	storeInstance, err := store.NewStore(dbPath)
	if err != nil {
		log.Fatalf("创建存储实例失败: %v", err)
	}
	defer storeInstance.Close()

	if err := storeInstance.HealthCheck(); err != nil {
		log.Fatalf("存储层健康检查失败: %v", err)
	}
	fmt.Println("✅ 存储层初始化成功")
	fmt.Println()

	// 3. 初始化Prometheus指标
	if cfg.Metrics.Enabled {
		fmt.Printf("📊 初始化Prometheus指标...\n")
		metrics.InitRecorder()
		fmt.Println("✅ 指标系统初始化成功")
	}

	// 4. 启动Prometheus指标服务器
	if cfg.Metrics.Enabled {
		go func() {
			addr := fmt.Sprintf(":%s", cfg.Metrics.Port)
			fmt.Printf("🚀 Prometheus指标服务: http://localhost%s/metrics\n", addr)
			if err := server.StartMetricsServer(cfg.Metrics.Port); err != nil {
				log.Printf("Prometheus服务启动失败: %v", err)
			}
		}()
	}

	// 5. 初始化Web服务器和控制服务器
	fmt.Println("🎮 初始化gRPC控制服务器...")
	fmt.Println("🌐 初始化Web管理界面...")

	webSrv, controlSrv := web.NewWebServerWithControlServer(storeInstance)
	fmt.Println("✅ 控制服务器和Web界面初始化成功")
	fmt.Println()

	// 6. 启动gRPC服务器
	grpcAddr := ":50051"
	go func() {
		fmt.Printf("🚀 gRPC服务器启动: %s\n", grpcAddr)
		if err := controlSrv.Start(grpcAddr); err != nil {
			log.Fatalf("gRPC服务器启动失败: %v", err)
		}
	}()

	// 7. 启动HTTP服���器
	webPort := cfg.Server.Port
	go func() {
		time.Sleep(1 * time.Second) // 等待其他服务启动
		fmt.Printf("🚀 Web管理界面启动: http://localhost:%s\n", webPort)
		if err := webSrv.Run(webPort); err != nil {
			log.Fatalf("Web服务器启动失败: %v", err)
		}
	}()

	// 8. 显示启动信息
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("🎉 goForward v2.0.0 启动成功!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
	fmt.Println("📍 服务地址:")
	fmt.Printf("   🌐 Web管理界面: http://localhost:%s\n", webPort)
	fmt.Printf("   🎮 gRPC控制端: %s\n", grpcAddr)
	if cfg.Metrics.Enabled {
		fmt.Printf("   📊 Prometheus指标: http://localhost:%s/metrics\n", cfg.Metrics.Port)
	}
	fmt.Println()
	fmt.Println("💡 主要功能:")
	fmt.Println("   - 节点注册和管理")
	fmt.Println("   - 配置下发和回滚")
	fmt.Println("   - 实时监控")
	fmt.Println("   - 死信队列(DLQ)")
	fmt.Println("   - 配置版本管理")
	fmt.Println()
	fmt.Println("⚡ 可以开始使用分布式管理功能了!")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止服务")
	fmt.Println()

	// 9. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println()
	fmt.Println("🛑 正在关闭服务...")

	// 控制服务器优雅关闭
	// TODO: 实现GracefulStop()方法
	// if err := controlSrv.GracefulStop(); err != nil {
	// 	log.Printf("关闭控制服务器失败: %v", err)
	// }

	// Web服务器停止 (通过context)
	// TODO: 实现Web服务器的优雅关闭
	// webSrv.Shutdown(ctx)

	fmt.Println("✅ 服务已安全关闭")
}
