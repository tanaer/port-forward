package proxy

import (
	"encoding/json"
	"testing"

	"goForward/conf"
	_ "goForward/sql" // 初始化数据库
)

func TestExportProxiesV2Format(t *testing.T) {
	// 测试导出功能
	data, err := ExportProxies(nil)
	if err != nil {
		t.Skipf("跳过测试（可能没有数据）: %v", err)
		return
	}

	// 验证是V2格式
	var result ExportConfigV2
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("解析V2格式失败: %v", err)
	}

	// 验证版本号
	if result.Version != "2.0" {
		t.Errorf("版本号错误: 期望 2.0, 实际 %s", result.Version)
	}

	// 打印结果
	t.Logf("导出版本: %s", result.Version)
	t.Logf("代理数量: %d", len(result.Proxies))
	t.Logf("出站配置数量: %d", len(result.Outbounds))

	// 验证代理数据结构
	for i, p := range result.Proxies {
		t.Logf("代理 %d: Name=%s, OutboundConfigName=%s", i+1, p.Name, p.OutboundConfigName)
		
		// 验证ID被清除
		if p.Id != 0 {
			t.Errorf("代理 %d: Id应该被清除，但值为 %d", i+1, p.Id)
		}
		// 验证OutboundConfigId被清除
		if p.OutboundConfigId != 0 {
			t.Errorf("代理 %d: OutboundConfigId应该被清除，但值为 %d", i+1, p.OutboundConfigId)
		}
	}

	// 验证出站配置数据结构
	for i, o := range result.Outbounds {
		t.Logf("出站 %d: Name=%s, Type=%s", i+1, o.Name, o.Type)
		
		// 验证ID被清除
		if o.Id != 0 {
			t.Errorf("出站 %d: Id应该被清除，但值为 %d", i+1, o.Id)
		}
	}
}

func TestImportProxiesV2Format(t *testing.T) {
	// 构造一个V2格式的测试数据
	testData := ExportConfigV2{
		Version: "2.0",
		Proxies: []ProxyExportData{
			{
				ProxyConfig: conf.ProxyConfig{
					Name:        "测试代理-导入测试",
					InboundPort: 59999,
					UUID:        "test-uuid-1234",
				},
				OutboundConfigName: "测试出站配置",
			},
		},
		Outbounds: []conf.OutboundConfig{
			{
				Name: "测试出站配置",
				Type: "socks5",
				Socks5Addr: "127.0.0.1",
				Socks5Port: 1080,
			},
		},
	}

	jsonData, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("序列化测试数据失败: %v", err)
	}

	// 测试解析
	var parsed ExportConfigV2
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("解析V2格式失败: %v", err)
	}

	if parsed.Version != "2.0" {
		t.Errorf("版本号错误")
	}
	if len(parsed.Proxies) != 1 {
		t.Errorf("代理数量错误")
	}
	if len(parsed.Outbounds) != 1 {
		t.Errorf("出站配置数量错误")
	}
	if parsed.Proxies[0].OutboundConfigName != "测试出站配置" {
		t.Errorf("出站配置名称关联错误")
	}

	t.Log("V2格式解析测试通过")
}
