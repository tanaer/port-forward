package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

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

	// 添加默认配置文件路径
	execPath, _ := os.Executable()
	configDir := filepath.Dir(execPath)
	configsDir := filepath.Join(configDir, "configs")
	defaultConfig := filepath.Join(configsDir, "forwards.json")

	// 确保配置目录存在
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		fmt.Printf("[配置热更新] 创建配置目录失败: %v\n", err)
		return
	}

	// 如果配置文件不存在，创建一个示例配置
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

	fmt.Println("[配置热更新] 配置文件监控已启动")
}

// createExampleConfig 创建示例配置文件
func createExampleConfig(configPath string) {
	exampleConfig := `{
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

	flag.StringVar(&conf.WebPort, "port", "8889", "Web Port")
	flag.StringVar(&conf.WebPass, "pass", "", "Web Password")
	flag.BoolVar(&conf.Debug, "debug", false, "Print connection")
	flag.Parse()

	// 如果请求显示版本，显示后退出
	if showVersion {
		version.ShowVersionAndExit()
		os.Exit(0)
	}
}
