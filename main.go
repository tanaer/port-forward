package main

import (
	"flag"
	"fmt"
	"sync"
	"goForward/conf"
	"goForward/forward"
	"goForward/proxy"
	"goForward/sql"
	"goForward/web"
)

func main() {
	// 初始化统计聚合器（Phase 1 优化）
	forward.InitStatsAggregator()
	defer forward.ShutdownStatsAggregator()

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
			OutTime:5,
			Blacklist:"",
			Whitelist:"",
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

func init() {
	flag.StringVar(&conf.WebPort, "port", "8889", "Web Port")
	flag.StringVar(&conf.WebPass, "pass", "", "Web Password")
	flag.BoolVar(&conf.Debug, "debug", false, "Print connection")
	flag.Parse()
}
