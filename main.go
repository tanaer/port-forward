package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"goForward/cli"
	"goForward/conf"
	"goForward/forward"
	"goForward/hotreload"
	"goForward/metrics"
	"goForward/proxy"
	"goForward/sql"
	"goForward/version"
	"goForward/web"
)

func main() {
	// 显示启动信息
	fmt.Printf("goForward %s 启动中...\n", version.GetVersion())
	fmt.Printf("启动时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	// 初始化性能监控（Week 3）
	metrics.InitMetrics()

	// 初始化统计聚合器（Phase 1 优化）
	forward.InitStatsAggregator()
	defer forward.ShutdownStatsAggregator()

	// 初始化配置热更新（Phase 3）
	initConfigHotReload()

	go web.Run()

	// 启动所有活动的代理配置
	go loadActiveProxies()

	// 初始化通道
	conf.Ch = make(chan string)
	forwardList := sql.GetAction()
	if len(forwardList) == 0 {
		//添加测试数据
		testData := conf.ConnectionStats{
			LocalPort:  conf.WebPort,
			RemotePort: conf.WebPort,
			RemoteAddr: "127.0.0.1",
			OutTime:    5,
			Blacklist:  "",
			Whitelist:  "",
			Protocol:   "udp",
		}
		sql.AddForward(testData)
		forwardList = sql.GetForwardList()
	}
	var largeStats forward.LargeConnectionStats
	largeStats.Connections = make([]*forward.ConnectionStats, len(forwardList))
	for i := range forwardList {
		connectionStats := &forward.ConnectionStats{
			ConnectionStats: conf.ConnectionStats{
				Id:         forwardList[i].Id,
				Protocol:   forwardList[i].Protocol,
				LocalPort:  forwardList[i].LocalPort,
				RemotePort: forwardList[i].RemotePort,
				RemoteAddr: forwardList[i].RemoteAddr,
				Whitelist:  forwardList[i].Whitelist,
				Blacklist:  forwardList[i].Blacklist,
				OutTime:    forwardList[i].OutTime,
				TotalBytes: forwardList[i].TotalBytes,
			},
			TotalBytesOld:  forwardList[i].TotalBytes,
			TotalBytesLock: sync.Mutex{},
			TCPConnections: make(map[string]*forward.IPStruct),
		}

		largeStats.Connections[i] = connectionStats
	}
	// 设置 WaitGroup 计数为连接数
	conf.Wg.Add(len(largeStats.Connections))

	// 并发执行多个转发
	for _, stats := range largeStats.Connections {
		go forward.Run(stats, &conf.Wg)
	}
	conf.Wg.Wait()
	defer close(conf.Ch)
}

// loadActiveProxies 加载并启动所有活动的代理配置
func loadActiveProxies() {
	// 给 web 服务一点启动时间
	// time.Sleep(2 * time.Second)

	activeProxies := sql.GetActiveProxies()
	if len(activeProxies) == 0 {
		fmt.Println("[Proxy] 没有找到活动的代理配置")
		return
	}

	fmt.Printf("[Proxy] 找到 %d 个活动的代理配置，开始启动...\n", len(activeProxies))
	pm := proxy.GetProxyManager()

	for _, proxyConfig := range activeProxies {
		fmt.Printf("[Proxy] 启动代理 ID=%d Port=%d...\n", proxyConfig.Id, proxyConfig.InboundPort)
		if err := pm.StartProxy(proxyConfig.Id); err != nil {
			fmt.Printf("[Proxy] 启动代理 ID=%d 失败: %v\n", proxyConfig.Id, err)
		} else {
			fmt.Printf("[Proxy] 代理 ID=%d 启动成功\n", proxyConfig.Id)
		}
	}
}

// initConfigHotReload 初始化配置热更新
func initConfigHotReload() {
	hotReloader := hotreload.NewHotReloader()

	// 使用当前工作目录而非可执行文件路径
	// 这确保开发时（go run .）和部署时都能正确监控配置文件
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("[配置热更新] 获取工作目录失败: %v\n", err)
		return
	}

	configsDir := filepath.Join(workDir, "configs")
	defaultConfig := filepath.Join(configsDir, "forwards.json")

	// 确保配置目录存在
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		fmt.Printf("[配置热更新] 创建配置目录失败: %v\n", err)
		return
	}

	// 如果配置文件不存在，创建一个示例配置（使用ID=1）
	if _, err := os.Stat(defaultConfig); os.IsNotExist(err) {
		createExampleConfig(defaultConfig)
	}

	hotReloader.AddConfigPath(defaultConfig)

	// 设置重新加载配置的处理函数
	hotReloader.SetReloadHandler(reloadConfiguration)

	// 启动配置热更新监控
	if err := hotReloader.Start(); err != nil {
		fmt.Printf("[配置热更新] 启动失败: %v\n", err)
		return
	}

	fmt.Printf("[配置热更新] 正在监控配置文件: %s\n", defaultConfig)
}

// createExampleConfig 创建示例配置文件
func createExampleConfig(configPath string) {
	exampleConfig := `{
  "id": 1,
  "localPort": "9999",
  "remotePort": "9999",
  "remoteAddr": "127.0.0.1",
  "protocol": "tcp",
  "outTime": 30,
  "whitelist": "192.168.1.0/24",
  "blacklist": "",
  "remark": "示例TCP转发配置 - 可编辑此文件进行热更新"
}`

	if err := os.WriteFile(configPath, []byte(exampleConfig), 0644); err != nil {
		fmt.Printf("[配置热更新] 创建示例配置失败: %v\n", err)
	} else {
		fmt.Printf("[配置热更新] 已创建示例配置文件: %s\n", configPath)
	}
}

// reloadConfiguration 重新加载配置
func reloadConfiguration() {
	fmt.Println("[配置热更新] 重新加载配置中...")

	// 重新加载所有转发表
	forwardList := sql.GetAction()
	for _, stats := range forwardList {
		// 重新启动转发（如果需要）
		fmt.Printf("[配置热更新] 重新加载转发: %s:%s -> %s:%s\n",
			stats.Protocol, stats.LocalPort, stats.RemoteAddr, stats.RemotePort)
	}

	fmt.Println("[配置热更新] 配置重新加载完成")
}

func init() {
	// 添加版本标志
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Show version information and exit")

	// Web服务配置
	flag.StringVar(&conf.WebPort, "port", "8889", "Web Port")
	flag.StringVar(&conf.WebPass, "pass", "", "Web Password")
	flag.BoolVar(&conf.Debug, "debug", false, "Print connection")

	// 导入配置命令
	var importConfigPath string
	var createExampleConfigPath string
	flag.StringVar(&importConfigPath, "import-config", "", "Import configuration from file (JSON/YAML format)")
	flag.StringVar(&createExampleConfigPath, "create-example-config", "", "Create example configuration file")

	// 批量操作命令
	var batchStartIds string
	var batchStopIds string
	var batchDeleteIds string
	var batchStatusIds string
	flag.StringVar(&batchStartIds, "batch-start", "", "Batch start proxies (comma-separated IDs or ranges like 1-5)")
	flag.StringVar(&batchStopIds, "batch-stop", "", "Batch stop proxies (comma-separated IDs or ranges like 1-5)")
	flag.StringVar(&batchDeleteIds, "batch-delete", "", "Batch delete proxies (comma-separated IDs or ranges like 1-5)")
	flag.StringVar(&batchStatusIds, "batch-status", "", "Batch query proxy status (comma-separated IDs or ranges like 1-5)")

	// 性能诊断命令
	var diagnosePerformance bool
	flag.BoolVar(&diagnosePerformance, "diagnose", false, "Run performance diagnosis (port usage, proxy configs, network connectivity)")

	// API服务器地址（用于CLI与主服务通信）
	var apiServerAddr string
	flag.StringVar(&apiServerAddr, "server", "http://localhost:8889", "API server address for CLI tools")

	// API访问令牌（用于CLI访问API）
	flag.StringVar(&conf.APIToken, "api-token", "", "API access token for CLI tools (required for API access when set)")

	flag.Parse()

	// 如果请求显示版本，显示后退出
	if showVersion {
		version.ShowVersionAndExit()
		os.Exit(0)
	}

	// 处理导入配置命令
	if importConfigPath != "" {
		fmt.Println("=== goForward 导入配置工具 ===")
		if err := cli.ImportFromFile(importConfigPath); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 导入配置失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ 导入配置完成")
		fmt.Println("注意: 导入的配置需要重启goForward服务才能生效")
		os.Exit(0)
	}

	// 创建示例配置文件
	if createExampleConfigPath != "" {
		fmt.Println("=== goForward 示例配置生成器 ===")
		if err := cli.CreateExampleConfig(createExampleConfigPath); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 创建示例配置失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ 示例配置文件已创建: %s\n", createExampleConfigPath)
		fmt.Println("\n文件格式说明:")
		fmt.Println("- 修改此文件可批量导入转发表和代理配置")
		fmt.Println("- 支持同时定义 forwards 和 proxies")
		fmt.Println("- 使用 -import-config 导入此配置文件")
		os.Exit(0)
	}

	// 批量操作命令
	if batchStartIds != "" {
		fmt.Println("=== goForward 批量启动工具 ===")
		args := strings.Split(batchStartIds, ",")
		if err := cli.BatchStart(args, apiServerAddr, conf.APIToken); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 批量启动失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if batchStopIds != "" {
		fmt.Println("=== goForward 批量停止工具 ===")
		args := strings.Split(batchStopIds, ",")
		if err := cli.BatchStop(args, apiServerAddr, conf.APIToken); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 批量停止失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if batchDeleteIds != "" {
		fmt.Println("=== goForward 批量删除工具 ===")
		args := strings.Split(batchDeleteIds, ",")
		if err := cli.BatchDelete(args, apiServerAddr, conf.APIToken); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 批量删除失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if batchStatusIds != "" {
		fmt.Println("=== goForward 批量状态查询工具 ===")
		args := strings.Split(batchStatusIds, ",")
		if err := cli.BatchStatus(args, apiServerAddr, conf.APIToken); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 批量状态查询失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// 性能诊断命令
	if diagnosePerformance {
		if err := cli.DiagnosePerformance(apiServerAddr, conf.APIToken); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 性能诊断失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// 验证端口格式
	if !isValidPort(conf.WebPort) {
		fmt.Fprintf(os.Stderr, "错误: 无效的Web端口 '%s'\n", conf.WebPort)
		os.Exit(1)
	}
}

// isValidPort 验证端口号
func isValidPort(portStr string) bool {
	if portStr == "" {
		return false
	}
	// 支持端口范围格式，如 "8000-9000"
	if strings.Contains(portStr, "-") {
		parts := strings.Split(portStr, "-")
		if len(parts) != 2 {
			return false
		}
		return isValidPortNumber(parts[0]) && isValidPortNumber(parts[1])
	}
	return isValidPortNumber(portStr)
}

// isValidPortNumber 验证单个端口号
func isValidPortNumber(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return port > 0 && port <= 65535
}
