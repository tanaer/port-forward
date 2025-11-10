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
func DiagnosePerformance() error {
	fmt.Println("=== goForward 性能诊断工具 ===")

	// 1. 系统端口诊断
	if err := diagnosePorts(); err != nil {
		fmt.Printf("❌ 端口诊断失败: %v\n", err)
	}

	// 2. 代理配置诊断
	if err := diagnoseProxyConfigs(); err != nil {
		fmt.Printf("❌ 代理配置诊断失败: %v\n", err)
	}

	// 3. 数据库性能诊断
	if err := diagnoseDatabase(); err != nil {
		fmt.Printf("❌ 数据库诊断失败: %v\n", err)
	}

	// 4. 网络连通性诊断
	if err := diagnoseNetwork(); err != nil {
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

// diagnoseProxyConfigs 诊断代理配置
func diagnoseProxyConfigs() error {
	fmt.Println("2. 代理配置诊断")
	fmt.Println(strings.Repeat("-", 50))

	proxyList := sql.GetProxyList()
	if len(proxyList) == 0 {
		fmt.Println("  无代理配置")
		return nil
	}

	for _, proxy := range proxyList {
		fmt.Printf("  代理 ID=%d: %s\n", proxy.Id, proxy.Name)
		fmt.Printf("    入站端口: %d\n", proxy.InboundPort)
		fmt.Printf("    出站类型: %s\n", proxy.OutboundType)
		fmt.Printf("    状态: %s\n", getStatusText(proxy.Status))

		// 检查配置是否完整
		if proxy.OutboundType == "socks5" {
			if proxy.Socks5Addr == "" || proxy.Socks5Port == 0 {
				fmt.Printf("    ❌ SOCKS5配置不完整\n")
			} else {
				fmt.Printf("    ✅ SOCKS5配置: %s:%d\n", proxy.Socks5Addr, proxy.Socks5Port)
			}
		}

		// 流量统计
		if proxy.TotalGigabyte > 0 {
			fmt.Printf("    流量: %s GB\n", sql.FormatTraffic(proxy.TotalGigabyte*1024*1024*1024))
		} else if proxy.TotalBytes > 0 {
			fmt.Printf("    流量: %s\n", sql.FormatTraffic(proxy.TotalBytes))
		} else {
			fmt.Printf("    流量: 0\n")
		}
		fmt.Println()
	}

	return nil
}

// diagnoseDatabase 诊断数据库性能
func diagnoseDatabase() error {
	fmt.Println("3. 数据库性能诊断")
	fmt.Println(strings.Repeat("-", 50))

	// 获取统计信息
	stats := sql.GetProxyStats()
	fmt.Printf("  总代理数: %v\n", stats["total"])
	fmt.Printf("  活跃代理数: %v\n", stats["active"])
	fmt.Printf("  总流量: %s GB\n", stats["total_traffic"])

	// 检查数据库大小
	if dbSize, err := getDatabaseSize(); err != nil {
		fmt.Printf("  ❌ 获取数据库大小失败: %v\n", err)
	} else {
		fmt.Printf("  数据库大小: %s\n", sql.FormatTraffic(dbSize))
	}

	return nil
}

// diagnoseNetwork 诊断网络连通性
func diagnoseNetwork() error {
	fmt.Println("4. 网络连通性诊断")
	fmt.Println(strings.Repeat("-", 50))

	proxyList := sql.GetActiveProxies()
	if len(proxyList) == 0 {
		fmt.Println("  无活跃代理")
		return nil
	}

	for _, proxy := range proxyList {
		fmt.Printf("  测试代理 ID=%d: %s\n", proxy.Id, proxy.Name)

		if proxy.OutboundType == "socks5" && proxy.Socks5Addr != "" {
			// 测试SOCKS5服务器连通性
			if err := testSocks5Connectivity(proxy.Socks5Addr, proxy.Socks5Port); err != nil {
				fmt.Printf("    ❌ SOCKS5服务器 %s:%d 不可达: %v\n", proxy.Socks5Addr, proxy.Socks5Port, err)
			} else {
				fmt.Printf("    ✅ SOCKS5服务器 %s:%d 可达\n", proxy.Socks5Addr, proxy.Socks5Port)
			}
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
