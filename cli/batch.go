package cli

import (
	"fmt"
	"strconv"
	"strings"

	"goForward/proxy"
	"goForward/sql"
)

// BatchStart 批量启动
func BatchStart(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请提供要启动的ID列表")
	}

	ids, err := parseIDs(args)
	if err != nil {
		return fmt.Errorf("解析ID失败: %v", err)
	}

	successCount := 0
	failCount := 0

	fmt.Println("\n=== 批量启动代理 ===")
	for i, id := range ids {
		if err := startProxy(id); err != nil {
			fmt.Printf("[%d/%d] 启动失败 ID=%d - %v\n", i+1, len(ids), id, err)
			failCount++
		} else {
			fmt.Printf("[%d/%d] 启动成功 ID=%d\n", i+1, len(ids), id)
			successCount++
		}
	}

	fmt.Printf("\n批量启动完成: 成功 %d 个，失败 %d 个\n", successCount, failCount)
	return nil
}

// BatchStop 批量停止
func BatchStop(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请提供要停止的ID列表")
	}

	ids, err := parseIDs(args)
	if err != nil {
		return fmt.Errorf("解析ID失败: %v", err)
	}

	successCount := 0
	failCount := 0

	fmt.Println("\n=== 批量停止代理 ===")
	for i, id := range ids {
		if err := stopProxy(id); err != nil {
			fmt.Printf("[%d/%d] 停止失败 ID=%d - %v\n", i+1, len(ids), id, err)
			failCount++
		} else {
			fmt.Printf("[%d/%d] 停止成功 ID=%d\n", i+1, len(ids), id)
			successCount++
		}
	}

	fmt.Printf("\n批量停止完成: 成功 %d 个，失败 %d 个\n", successCount, failCount)
	return nil
}

// BatchDelete 批量删除
func BatchDelete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请提供要删除的ID列表")
	}

	ids, err := parseIDs(args)
	if err != nil {
		return fmt.Errorf("解析ID失败: %v", err)
	}

	successCount := 0
	failCount := 0

	fmt.Println("\n=== 批量删除代理 ===")
	for i, id := range ids {
		if err := deleteProxy(id); err != nil {
			fmt.Printf("[%d/%d] 删除失败 ID=%d - %v\n", i+1, len(ids), id, err)
			failCount++
		} else {
			fmt.Printf("[%d/%d] 删除成功 ID=%d\n", i+1, len(ids), id)
			successCount++
		}
	}

	fmt.Printf("\n批量删除完成: 成功 %d 个，失败 %d 个\n", successCount, failCount)
	return nil
}

// BatchStatus 批量查询状态
func BatchStatus(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请提供要查询的ID列表")
	}

	ids, err := parseIDs(args)
	if err != nil {
		return fmt.Errorf("解析ID失败: %v", err)
	}

	fmt.Println("\n=== 批量查询代理状态 ===")
	fmt.Println("ID\t状态\t名称\t入站端口\t出站类型")
	fmt.Println(strings.Repeat("-", 60))

	for _, id := range ids {
		proxyConfig, err := sql.GetProxyConfigById(id)
		if err != nil {
			fmt.Printf("%d\t❌ 错误\t%s\n", id, err)
			continue
		}

		status := "已停止"
		if proxyConfig.Status == 0 {
			status = "运行中"
		}

		fmt.Printf("%d\t%s\t%s\t%d\t%s\n",
			proxyConfig.Id,
			status,
			proxyConfig.Name,
			proxyConfig.InboundPort,
			proxyConfig.OutboundType)
	}

	return nil
}

// startProxy 启动单个代理
func startProxy(id int) error {
	// 停止现有代理实例
	pm := proxy.GetProxyManager()
	if err := pm.StopProxy(id); err != nil {
		// 忽略停止失败（可能未运行）
	}

	// 更新数据库状态
	if !sql.UpdateProxyStatus(id, 0) {
		return fmt.Errorf("更新数据库状态失败")
	}

	// 启动代理
	if err := pm.StartProxy(id); err != nil {
		return fmt.Errorf("启动代理失败: %v", err)
	}

	return nil
}

// stopProxy 停止单个代理
func stopProxy(id int) error {
	pm := proxy.GetProxyManager()
	if err := pm.StopProxy(id); err != nil {
		return fmt.Errorf("停止代理失败: %v", err)
	}

	// 更新数据库状态
	if !sql.UpdateProxyStatus(id, 1) {
		return fmt.Errorf("更新数据库状态失败")
	}

	return nil
}

// deleteProxy 删除单个代理
func deleteProxy(id int) error {
	pm := proxy.GetProxyManager()
	if err := pm.StopProxy(id); err != nil {
		// 忽略停止失败
	}

	// 从数据库删除
	if !sql.DeleteProxy(id) {
		return fmt.Errorf("删除数据库记录失败")
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
