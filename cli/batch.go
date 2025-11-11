package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// BatchStart 批量启动
func BatchStart(args []string, apiServerAddr, token string) error {
	if len(args) == 0 {
		return fmt.Errorf("请提供要启动的ID列表")
	}

	ids, err := parseIDs(args)
	if err != nil {
		return fmt.Errorf("解析ID失败: %v", err)
	}

	// 使用HTTP客户端
	client := NewAPIClientWithToken(apiServerAddr, token)
	result, err := client.BatchStart(ids)
	if err != nil {
		return fmt.Errorf("调用API失败: %v", err)
	}

	fmt.Println("=== 批量启动代理 ===")
	for _, id := range ids {
		success := false
		for _, successID := range result.Success {
			if id == successID {
				success = true
				break
			}
		}
		if success {
			fmt.Printf("ID=%d 启动成功\n", id)
		} else {
			msg := result.Failed[id]
			fmt.Printf("ID=%d 启动失败 - %s\n", id, msg)
		}
	}

	fmt.Printf("\n批量启动完成: 成功 %d 个，失败 %d 个\n", len(result.Success), len(result.Failed))
	if result.Message != "" {
		fmt.Println(result.Message)
	}
	return nil
}

// BatchStop 批量停止
func BatchStop(args []string, apiServerAddr, token string) error {
	if len(args) == 0 {
		return fmt.Errorf("请提供要停止的ID列表")
	}

	ids, err := parseIDs(args)
	if err != nil {
		return fmt.Errorf("解析ID失败: %v", err)
	}

	// 使用HTTP客户端
	client := NewAPIClientWithToken(apiServerAddr, token)
	result, err := client.BatchStop(ids)
	if err != nil {
		return fmt.Errorf("调用API失败: %v", err)
	}

	fmt.Println("=== 批量停止代理 ===")
	for _, id := range ids {
		success := false
		for _, successID := range result.Success {
			if id == successID {
				success = true
				break
			}
		}
		if success {
			fmt.Printf("ID=%d 停止成功\n", id)
		} else {
			msg := result.Failed[id]
			fmt.Printf("ID=%d 停止失败 - %s\n", id, msg)
		}
	}

	fmt.Printf("\n批量停止完成: 成功 %d 个，失败 %d 个\n", len(result.Success), len(result.Failed))
	if result.Message != "" {
		fmt.Println(result.Message)
	}
	return nil
}

// BatchDelete 批量删除
func BatchDelete(args []string, apiServerAddr, token string) error {
	if len(args) == 0 {
		return fmt.Errorf("请提供要删除的ID列表")
	}

	ids, err := parseIDs(args)
	if err != nil {
		return fmt.Errorf("解析ID失败: %v", err)
	}

	// 使用HTTP客户端
	client := NewAPIClientWithToken(apiServerAddr, token)
	result, err := client.BatchDelete(ids)
	if err != nil {
		return fmt.Errorf("调用API失败: %v", err)
	}

	fmt.Println("=== 批量删除代理 ===")
	for _, id := range ids {
		success := false
		for _, successID := range result.Success {
			if id == successID {
				success = true
				break
			}
		}
		if success {
			fmt.Printf("ID=%d 删除成功\n", id)
		} else {
			msg := result.Failed[id]
			fmt.Printf("ID=%d 删除失败 - %s\n", id, msg)
		}
	}

	fmt.Printf("\n批量删除完成: 成功 %d 个，失败 %d 个\n", len(result.Success), len(result.Failed))
	if result.Message != "" {
		fmt.Println(result.Message)
	}
	return nil
}

// BatchStatus 批量查询状态
func BatchStatus(args []string, apiServerAddr, token string) error {
	if len(args) == 0 {
		return fmt.Errorf("请提供要查询的ID列表")
	}

	ids, err := parseIDs(args)
	if err != nil {
		return fmt.Errorf("解析ID失败: %v", err)
	}

	// 使用HTTP客户端获取代理列表
	client := NewAPIClientWithToken(apiServerAddr, token)
	proxies, err := client.GetProxyList()
	if err != nil {
		return fmt.Errorf("调用API失败: %v", err)
	}

	// 过滤出查询的ID
	var statuses []ProxyInfo
	idSet := make(map[int]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	for _, proxy := range proxies {
		if idSet[proxy.ID] {
			statuses = append(statuses, proxy)
		}
	}

	fmt.Println("=== 批量查询代理状态 ===")
	fmt.Println("ID\t状态\t名称\t入站端口\t出站类型")
	fmt.Println(strings.Repeat("-", 60))

	for _, proxy := range statuses {
		status := "已停止"
		if proxy.Status == 0 {
			status = "运行中"
		}

		traffic := ""
		if proxy.TotalBytes > 0 {
			traffic = fmt.Sprintf(" (流量: %s)", formatTraffic(proxy.TotalBytes))
		}

		fmt.Printf("%d\t%s\t%s\t%d\t%s%s\n",
			proxy.ID,
			status,
			proxy.Name,
			proxy.InboundPort,
			proxy.OutboundType,
			traffic)
	}

	if len(statuses) == 0 {
		fmt.Println("未找到匹配的代理配置")
	}

	return nil
}

// parseIDs 解析ID列表
func parseIDs(args []string) ([]int, error) {
	var ids []int
	seen := make(map[int]bool) // 用于去重

	for _, arg := range args {
		// 支持逗号分隔
		parts := strings.Split(arg, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			// 支持范围，如 "1-5"
			if strings.Contains(part, "-") {
				rangeParts := strings.Split(part, "-")
				if len(rangeParts) == 2 {
					start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
					if err != nil {
						return nil, fmt.Errorf("无效的起始ID: %s", part)
					}
					end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
					if err != nil {
						return nil, fmt.Errorf("无效的结束ID: %s", part)
					}
					if start > end {
						start, end = end, start
					}
					for i := start; i <= end; i++ {
						if !seen[i] {
							ids = append(ids, i)
							seen[i] = true
						}
					}
					continue
				}
			}

			// 单个ID
			id, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("无效的ID: %s", part)
			}
			if !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
	}

	return ids, nil
}

// formatTraffic 格式化流量显示
func formatTraffic(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
