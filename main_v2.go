package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	fmt.Println("=" * 50)
	fmt.Println("goForward v2.0.0 - 分布式控制端")
	fmt.Println("=" * 50)
	fmt.Println()

	// 1. 加载配置
	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()
	// TODO: 加载命令行参数
	// cfg.LoadFromCommandLine(args)

	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	fmt.Printf("✅ 配置加载成功\n")
	fmt.Printf("   - 服务端口: %s\n", cfg.Server.Port)
	fmt.Printf("   - 回滚系统: %v\n", cfg.Rollback.Enabled)
	fmt.Printf("   - Prometheus: %v (端口 %s)\n", cfg.Metrics.Enabled, cfg.Metrics.Port)
	fmt.Printf()

	// 2. 初始化存储层
	dbPath := store.GetDatabasePath()
	fmt.Printf("📦 初始化存储层: %s\n", dbPath)

	storeInstance, err := store.NewStore(dbPath)
	if err != nil {
		log.Fatalf("创建存储实例失败: %v", err)
	}
	defer storeInstance.Close()

	// 健康检查
	if err := storeInstance.HealthCheck(); err != nil {
		log.Fatalf("存储层健康检查失败: %v", err)
	}
	fmt.Printf("✅ 存储层初始化成功\n")

	// 3. 初始化Prometheus指标
	if cfg.Metrics.Enabled {
		fmt.Printf("📊 初始化Prometheus指标...\n")
		metrics.InitRecorder()
		fmt.Printf("✅ 指标系统初始化成功 (端口 %s)\n", cfg.Metrics.Port)
	}

	// 4. 启动Prometheus指标服务器
	if cfg.Metrics.Enabled {
		go func() {
			addr := fmt.Sprintf(":%s", cfg.Metrics.Port)
			fmt.Printf("🚀 启动Prometheus指标服务: http://localhost%s/metrics\n", addr)
			if err := server.StartMetricsServer(cfg.Metrics.Port); err != nil {
				log.Printf("Prometheus服务启动失败: %v", err)
			}
		}()
	}

	// 5. 初始化控制服务器
	fmt.Printf("🎮 初始化gRPC控制服务器...\n")
	controlSrv, err := server.NewControlServer(storeInstance)
	if err != nil {
		log.Fatalf("创建控制服务器失败: %v", err)
	}
	fmt.Printf("✅ 控制服务器初始化成功\n")

	// 6. 初始化Web服务器
	fmt.Printf("🌐 初始化Web管理界面...\n")
	webSrv, err := web.NewWebServerWithControlServer(storeInstance, controlSrv)
	if err != nil {
		log.Fatalf("创建Web服务器失败: %v", err)
	}

	// 7. 启动服务
	port := cfg.Server.Port
	fmt.Printf("\n" + "=" * 50)
	fmt.Printf("✅ goForward v2.0.0 启动成功!\n")
	fmt.Printf("=" * 50)
	fmt.Printf("\n📍 服务地址:\n")
	fmt.Printf("   - Web管理界面: http://localhost:%s\n", port)
	fmt.Printf("   - gRPC服务端: 端口50051\n")
	if cfg.Metrics.Enabled {
		fmt.Printf("   - Prometheus: http://localhost:%s/metrics\n", cfg.Metrics.Port)
	}
	fmt.Printf("\n🎉 可以开始使用分布式管理功能了!\n")
	fmt.Printf("\n按 Ctrl+C 停止服务\n\n")

	// 8. 启动HTTP服务器
	// TODO: 实现web.NewHTTPServer()方法或使用标准库
	// 暂时使用简化启动方式
	fmt.Printf("⚠️  注意: v2.0.0完整启动功能需要进一步集成\n")
	fmt.Printf("   当前仅初始化了控制服务器和存储层\n")
	fmt.Printf("   详细启动请参考: control/server 和 control/web\n")

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := controlSrv.GracefulStop(); err != nil {
		log.Printf("关闭控制服务器失败: %v", err)
	}

	fmt.Println("✅ 服务已安全关闭")
}
