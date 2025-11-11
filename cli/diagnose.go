package cli

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"goForward/sql"
)

// DiagnosePerformance 性能诊断
func DiagnosePerformance(apiServerAddr, token string) error {
	fmt.Println("=== goForward 性能诊断工具 ===")
	fmt.Println()

	// 使用HTTP客户端获取诊断数据
	client := NewAPIClientWithToken(apiServerAddr, token)
	result, err := client.GetDiagnosis()
	if err != nil {
		return fmt.Errorf("调用API失败: %v", err)
	}

	// 1. 代理配置诊断
	if err := diagnoseProxyConfigsFromAPI(result.Proxies); err != nil {
		fmt.Printf("❌ 代理配置诊断失败: %v\n", err)
	}

	// 2. 数据库性能诊断
	if err := diagnoseDatabaseFromAPI(result.Database); err != nil {
		fmt.Printf("❌ 数据库诊断失败: %v\n", err)
	}

	// 3. 网络连通性诊断
	if err := diagnoseNetworkFromAPI(result.Network); err != nil {
		fmt.Printf("❌ 网络诊断失败: %v\n", err)
	}

	fmt.Println("=== 性能诊断完成 ===")
	return nil
}

// diagnosePorts 诊断端口占用情况
func diagnosePorts() error {
	fmt.Println("1. 端口占用诊断")
	fmt.Println(strings.Repeat("-", 50))

	proxyList := sql.GetProxyList()
	if len(proxyList) == 0 {
		fmt.Println("  无代理配置")
		return nil
	}

	portMap := make(map[int][]int) // port -> list of proxy IDs
	for _, proxy := range proxyList {
		portMap[proxy.InboundPort] = append(portMap[proxy.InboundPort], proxy.Id)
	}

	var ports []int
	for port := range portMap {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	for _, port := range ports {
		proxyIDs := portMap[port]
		if len(proxyIDs) > 1 {
			fmt.Printf("  ⚠️  端口 %d 被多个代理占用: %v\n", port, proxyIDs)
		} else {
			fmt.Printf("  ✅ 端口 %d 正常 (代理ID: %d)\n", port, proxyIDs[0])
		}

		// 检查端口是否在监听
		if isPortInUse(port) {
			fmt.Printf("     端口 %d 正在监听\n", port)
		} else {
			fmt.Printf("     ⚠️  端口 %d 未监听 (代理可能未运行)\n", port)
		}
	}

	return nil
}

// diagnoseProxyConfigsFromAPI 从API结果诊断代理配置
func diagnoseProxyConfigsFromAPI(proxies []ProxyInfo) error {
	fmt.Println("1. 代理配置诊断")
	fmt.Println(strings.Repeat("-", 50))

	if len(proxies) == 0 {
		fmt.Println("  无代理配置")
		return nil
	}

	for _, proxy := range proxies {
		fmt.Printf("  代理 ID=%d: %s\n", proxy.ID, proxy.Name)
		fmt.Printf("    入站端口: %d\n", proxy.InboundPort)
		fmt.Printf("    出站类型: %s\n", proxy.OutboundType)
		fmt.Printf("    状态: %s\n", getStatusText(proxy.Status))

		// 流量统计
		if proxy.TotalBytes > 0 {
			fmt.Printf("    流量: %s\n", sql.FormatTraffic(proxy.TotalBytes))
		} else {
			fmt.Printf("    流量: 0\n")
		}
		fmt.Println()
	}

	return nil
}

// diagnoseDatabaseFromAPI 从API结果诊断数据库性能
func diagnoseDatabaseFromAPI(stats TrafficStats) error {
	fmt.Println("2. 数据库性能诊断")
	fmt.Println(strings.Repeat("-", 50))

	fmt.Printf("  总代理数: %d\n", stats.Total)
	fmt.Printf("  活跃代理数: %d\n", stats.Active)
	fmt.Printf("  总流量: %d GB\n", stats.TotalTraffic)

	return nil
}

// diagnoseNetworkFromAPI 从API结果诊断网络连通性
func diagnoseNetworkFromAPI(network []NetworkTestResult) error {
	fmt.Println("3. 网络连通性诊断")
	fmt.Println(strings.Repeat("-", 50))

	if len(network) == 0 {
		fmt.Println("  无SOCKS5代理需要测试")
		return nil
	}

	for _, test := range network {
		if test.OK {
			fmt.Printf("  ✅ %s:%d 可达\n", test.Addr, test.Port)
		} else {
			fmt.Printf("  ❌ %s:%d 不可达: %s\n", test.Addr, test.Port, test.Error)
		}
	}

	return nil
}

// isPortInUse 检查端口是否在监听
func isPortInUse(port int) bool {
	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true // 端口被占用
	}
	conn.Close()
	return false
}

// getStatusText 获取状态文本
func getStatusText(status int) string {
	if status == 0 {
		return "运行中"
	}
	return "已停止"
}

// getDatabaseSize 获取数据库大小
func getDatabaseSize() (uint64, error) {
	// 简化实现，返回估算值
	// 实际应该读取数据库文件大小
	return 0, nil
}

// testSocks5Connectivity 测试SOCKS5连通性
func testSocks5Connectivity(addr string, port int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", addr, port), 3*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
