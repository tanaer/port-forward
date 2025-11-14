package proxy

import (
	"encoding/json"
	"fmt"
	"os"

	"goForward/conf"
	"goForward/sql"
)

// ExportConfig 导出配置
type ExportConfig struct {
	Version string              `json:"version"`
	Proxies []conf.ProxyConfig  `json:"proxies"`
}

// ExportProxies 导出代理配置（支持批量选择）
func ExportProxies(ids []int) (string, error) {
	var proxies []conf.ProxyConfig

	if len(ids) == 0 {
		// 导出所有代理
		proxies = sql.GetProxyList()
	} else {
		// 导出指定ID的代理
		for _, id := range ids {
			cfg := sql.GetProxy(id)
			if cfg.Id != 0 {
				proxies = append(proxies, cfg)
			}
		}
	}

	if len(proxies) == 0 {
		return "", fmt.Errorf("没有可导出的代理配置")
	}

	// 清除运行时状态
	for i := range proxies {
		proxies[i].Id = 0
		proxies[i].Status = 0
		proxies[i].TotalBytes = 0
		proxies[i].TotalGigabyte = 0
	}

	exportData := ExportConfig{
		Version: "1.0",
		Proxies: proxies,
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %v", err)
	}

	return string(data), nil
}

// ExportProxiesToFile 导出代理配置到文件
func ExportProxiesToFile(ids []int, filename string) error {
	data, err := ExportProxies(ids)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, []byte(data), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// ImportProxies 导入代理配置
func ImportProxies(jsonData string) ([]int, error) {
	var exportData ExportConfig
	if err := json.Unmarshal([]byte(jsonData), &exportData); err != nil {
		return nil, fmt.Errorf("解析配置失败: %v", err)
	}

	var importedIds []int
	pm := GetProxyManager()

	for _, cfg := range exportData.Proxies {
		// 添加到数据库
		id := sql.AddProxy(cfg)
		if id == 0 {
			fmt.Printf("[Import] 添加代理 %s 失败\n", cfg.Name)
			continue
		}

		// 启动代理
		if err := pm.StartProxy(id); err != nil {
			fmt.Printf("[Import] 启动代理 ID=%d 失败: %v\n", id, err)
			continue
		}

		importedIds = append(importedIds, id)
		fmt.Printf("[Import] 成功导入代理 ID=%d Name=%s\n", id, cfg.Name)
	}

	return importedIds, nil
}

// ImportProxiesFromFile 从文件导入代理配置
func ImportProxiesFromFile(filename string) ([]int, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	return ImportProxies(string(data))
}
