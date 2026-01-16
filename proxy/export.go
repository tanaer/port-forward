package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"goForward/conf"
	"goForward/sql"
)

// ExportConfig 导出配置（V1版本，保留兼容）
type ExportConfig struct {
	Version string             `json:"version"`
	Proxies []conf.ProxyConfig `json:"proxies"`
}

// ExportConfigV2 导出配置V2（包含出站配置）
type ExportConfigV2 struct {
	Version   string                 `json:"version"`
	Proxies   []ProxyExportData      `json:"proxies"`
	Outbounds []conf.OutboundConfig  `json:"outbounds"`
}

// ProxyExportData 代理导出数据（包含关联的出站配置名称）
type ProxyExportData struct {
	conf.ProxyConfig
	OutboundConfigName string `json:"outboundConfigName,omitempty"` // 关联的出站配置名称
}

// ImportProgress 导入进度
type ImportProgress struct {
	Total    int
	Imported int
	Failed   int
}

// ImportProgressFunc 导入进度回调
type ImportProgressFunc func(ImportProgress)

// ExportProxies 导出代理配置（支持批量选择，V2格式包含出站配置）
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

	// 收集所有关联的出站配置
	outboundMap := make(map[int]*conf.OutboundConfig)
	for _, p := range proxies {
		if p.OutboundConfigId > 0 {
			if _, exists := outboundMap[p.OutboundConfigId]; !exists {
				outbound, err := sql.GetOutbound(p.OutboundConfigId)
				if err == nil && outbound != nil {
					outboundMap[p.OutboundConfigId] = outbound
				}
			}
		}
	}

	// 构建代理导出数据
	var proxyExports []ProxyExportData
	for _, p := range proxies {
		exportData := ProxyExportData{
			ProxyConfig: p,
		}
		// 关联出站配置名称
		if p.OutboundConfigId > 0 {
			if outbound, exists := outboundMap[p.OutboundConfigId]; exists {
				exportData.OutboundConfigName = outbound.Name
			}
		}
		// 清除运行时状态
		exportData.Id = 0
		exportData.Status = 0
		exportData.TotalBytes = 0
		exportData.TotalGigabyte = 0
		exportData.OutboundConfigId = 0 // 导入时根据名称重建关联
		proxyExports = append(proxyExports, exportData)
	}

	// 构建出站配置导出数据
	var outbounds []conf.OutboundConfig
	for _, outbound := range outboundMap {
		o := *outbound
		// 清除运行时状态
		o.Id = 0
		o.Status = 1 // 导入时默认为停用状态
		outbounds = append(outbounds, o)
	}

	exportData := ExportConfigV2{
		Version:   "2.0",
		Proxies:   proxyExports,
		Outbounds: outbounds,
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
	return ImportProxiesWithProgress(jsonData, nil)
}

// ImportProxiesWithProgress 导入代理配置并回调进度（支持V1和V2格式）
func ImportProxiesWithProgress(jsonData string, progressFn ImportProgressFunc) ([]int, error) {
	cleaned := sanitizeImportJSON(jsonData)
	if cleaned == "" {
		return nil, fmt.Errorf("配置数据不能为空")
	}

	// 尝试解析 V2 格式
	var exportDataV2 ExportConfigV2
	if err := json.Unmarshal([]byte(cleaned), &exportDataV2); err == nil && exportDataV2.Version == "2.0" {
		return importProxiesV2(exportDataV2, progressFn)
	}

	// 回退到 V1 格式
	exportData, err := parseExportConfigV1(cleaned)
	if err != nil {
		return nil, err
	}

	return importProxiesV1(exportData, progressFn)
}

// importProxiesV2 导入V2格式配置（包含出站配置）
func importProxiesV2(exportData ExportConfigV2, progressFn ImportProgressFunc) ([]int, error) {
	total := len(exportData.Proxies)
	if total == 0 {
		return nil, fmt.Errorf("配置为空")
	}

	// 第一步：导入出站配置，建立名称到ID的映射
	outboundNameToId := make(map[string]int)
	for _, outbound := range exportData.Outbounds {
		// 检查是否已存在同名出站配置
		existingId := findOutboundByName(outbound.Name)
		if existingId > 0 {
			outboundNameToId[outbound.Name] = existingId
			fmt.Printf("[Import] 出站配置 %s 已存在，使用现有ID=%d\n", outbound.Name, existingId)
			continue
		}

		// 添加新的出站配置
		newOutbound := outbound
		newOutbound.Status = 1 // 默认停用
		id, err := sql.AddOutbound(&newOutbound)
		if err != nil {
			fmt.Printf("[Import] 添加出站配置 %s 失败: %v\n", outbound.Name, err)
			continue
		}
		outboundNameToId[outbound.Name] = id
		fmt.Printf("[Import] 成功导入出站配置 %s ID=%d\n", outbound.Name, id)
	}

	// 第二步：导入代理配置
	imported := 0
	failed := 0
	var importedIds []int
	pm := GetProxyManager()

	notifyImportProgress(progressFn, ImportProgress{
		Total:    total,
		Imported: imported,
		Failed:   failed,
	})

	for _, exportProxy := range exportData.Proxies {
		cfg := exportProxy.ProxyConfig

		// 恢复出站配置关联
		if exportProxy.OutboundConfigName != "" {
			if outboundId, exists := outboundNameToId[exportProxy.OutboundConfigName]; exists {
				cfg.OutboundConfigId = outboundId
			}
		}

		// 添加到数据库
		id := sql.AddProxy(cfg)
		if id == 0 {
			fmt.Printf("[Import] 添加代理 %s 失败\n", cfg.Name)
			failed++
			notifyImportProgress(progressFn, ImportProgress{
				Total:    total,
				Imported: imported,
				Failed:   failed,
			})
			continue
		}

		// 启动代理
		if err := pm.StartProxy(id); err != nil {
			fmt.Printf("[Import] 启动代理 ID=%d 失败: %v\n", id, err)
			failed++
			notifyImportProgress(progressFn, ImportProgress{
				Total:    total,
				Imported: imported,
				Failed:   failed,
			})
			continue
		}

		imported++
		importedIds = append(importedIds, id)
		notifyImportProgress(progressFn, ImportProgress{
			Total:    total,
			Imported: imported,
			Failed:   failed,
		})
		fmt.Printf("[Import] 成功导入代理 ID=%d Name=%s\n", id, cfg.Name)
	}

	return importedIds, nil
}

// importProxiesV1 导入V1格式配置（向后兼容）
func importProxiesV1(exportData ExportConfig, progressFn ImportProgressFunc) ([]int, error) {
	total := len(exportData.Proxies)
	if total == 0 {
		return nil, fmt.Errorf("配置为空")
	}
	imported := 0
	failed := 0

	var importedIds []int
	pm := GetProxyManager()

	notifyImportProgress(progressFn, ImportProgress{
		Total:    total,
		Imported: imported,
		Failed:   failed,
	})

	for _, cfg := range exportData.Proxies {
		// 添加到数据库
		id := sql.AddProxy(cfg)
		if id == 0 {
			fmt.Printf("[Import] 添加代理 %s 失败\n", cfg.Name)
			failed++
			notifyImportProgress(progressFn, ImportProgress{
				Total:    total,
				Imported: imported,
				Failed:   failed,
			})
			continue
		}

		// 启动代理
		if err := pm.StartProxy(id); err != nil {
			fmt.Printf("[Import] 启动代理 ID=%d 失败: %v\n", id, err)
			failed++
			notifyImportProgress(progressFn, ImportProgress{
				Total:    total,
				Imported: imported,
				Failed:   failed,
			})
			continue
		}

		imported++
		importedIds = append(importedIds, id)
		notifyImportProgress(progressFn, ImportProgress{
			Total:    total,
			Imported: imported,
			Failed:   failed,
		})
		fmt.Printf("[Import] 成功导入代理 ID=%d Name=%s\n", id, cfg.Name)
	}

	return importedIds, nil
}

// findOutboundByName 根据名称查找出站配置
func findOutboundByName(name string) int {
	outbounds, err := sql.GetOutboundList()
	if err != nil {
		return 0
	}
	for _, o := range outbounds {
		if o.Name == name {
			return o.Id
		}
	}
	return 0
}

func notifyImportProgress(progressFn ImportProgressFunc, progress ImportProgress) {
	if progressFn != nil {
		progressFn(progress)
	}
}

func parseExportConfigV1(cleaned string) (ExportConfig, error) {
	var exportData ExportConfig
	err := json.Unmarshal([]byte(cleaned), &exportData)
	if err == nil && len(exportData.Proxies) > 0 {
		return exportData, nil
	}

	var proxies []conf.ProxyConfig
	if errArray := json.Unmarshal([]byte(cleaned), &proxies); errArray == nil {
		if len(proxies) == 0 {
			return ExportConfig{}, fmt.Errorf("配置为空")
		}
		return ExportConfig{
			Version: "1.0",
			Proxies: proxies,
		}, nil
	}

	if err != nil {
		return ExportConfig{}, fmt.Errorf("解析配置失败: %w", err)
	}

	return ExportConfig{}, fmt.Errorf("配置为空")
}

func sanitizeImportJSON(input string) string {
	trimmed := strings.TrimSpace(input)
	return strings.TrimPrefix(trimmed, "\ufeff")
}

// ImportProxiesFromFile 从文件导入代理配置
func ImportProxiesFromFile(filename string) ([]int, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	return ImportProxies(string(data))
}
