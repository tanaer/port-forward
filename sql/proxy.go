package sql

import (
	"fmt"
	"log"

	"goForward/conf"
	"gorm.io/gorm"
)

const gigabyte = 1024 * 1024 * 1024

// InitProxyTables 初始化代理相关表
func InitProxyTables() {
	db.AutoMigrate(&conf.ProxyConfig{})
	db.AutoMigrate(&conf.Subscription{})
}

// GetProxyList 获取代理配置列表
func GetProxyList() []conf.ProxyConfig {
	var res []conf.ProxyConfig
	db.Model(&conf.ProxyConfig{}).Order("id desc").Find(&res)
	for i := range res {
		totalBytes := res[i].TotalBytes + res[i].TotalGigabyte*gigabyte
		res[i].TotalTraffic = FormatTraffic(totalBytes)
	}
	return res
}

// GetActiveProxies 获取启用的代理配置
func GetActiveProxies() []conf.ProxyConfig {
	var res []conf.ProxyConfig
	db.Model(&conf.ProxyConfig{}).Where("status = ?", 0).Find(&res)
	return res
}

// GetProxy 获取指定代理配置
func GetProxy(id int) conf.ProxyConfig {
	var proxy conf.ProxyConfig
	db.Model(&conf.ProxyConfig{}).Where("id = ?", id).First(&proxy)
	return proxy
}

// AddProxy 添加代理配置
func AddProxy(proxy conf.ProxyConfig) int {
	// 检查端口是否被占用
	var count int64
	db.Model(&conf.ProxyConfig{}).Where("inbound_port = ? AND status = 0", proxy.InboundPort).Count(&count)
	if count > 0 {
		log.Printf("端口 %d 已被占用", proxy.InboundPort)
		return 0
	}

	// 开启事务
	tx := db.Begin()
	if tx.Error != nil {
		log.Println("开启事务失败")
		return 0
	}

	// 插入代理配置
	if err := tx.Create(&proxy).Error; err != nil {
		log.Println("插入代理配置失败:", err)
		tx.Rollback()
		return 0
	}

	// 提交事务
	tx.Commit()
	return proxy.Id
}

// UpdateProxy 更新代理配置
func UpdateProxy(proxy conf.ProxyConfig) bool {
	// 检查端口冲突（排除自己）
	var count int64
	db.Model(&conf.ProxyConfig{}).
		Where("inbound_port = ? AND status = 0 AND id != ?", proxy.InboundPort, proxy.Id).
		Count(&count)
	if count > 0 {
		log.Printf("端口 %d 已被其他配置占用", proxy.InboundPort)
		return false
	}

	result := db.Model(&conf.ProxyConfig{}).Where("id = ?", proxy.Id).Updates(proxy)
	if result.Error != nil {
		log.Println("更新代理配置失败:", result.Error)
		return false
	}
	return true
}

// UpdateProxyStatus 更新代理状态
func UpdateProxyStatus(id int, status int) bool {
	result := db.Model(&conf.ProxyConfig{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		log.Println("更新代理状态失败:", result.Error)
		return false
	}
	return true
}

// DeleteProxy 删除代理配置
func DeleteProxy(id int) bool {
	// 先删除关联的订阅
	db.Where("proxy_id = ?", id).Delete(&conf.Subscription{})

	// 删除代理配置
	if err := db.Where("id = ?", id).Delete(&conf.ProxyConfig{}).Error; err != nil {
		log.Println("删除代理配置失败:", err)
		return false
	}
	return true
}

// UpdateProxyTraffic 更新代理流量统计
func UpdateProxyTraffic(id int, bytes uint64) bool {
	if bytes == 0 {
		return true
	}
	result := db.Model(&conf.ProxyConfig{}).
		Where("id = ?", id).
		UpdateColumn("total_bytes", gorm.Expr("total_bytes + ?", bytes))
	if result.Error != nil {
		log.Println("更新流量统计失败:", result.Error)
		return false
	}
	return true
}

// UpdateProxyTrafficGB 更新代理流量统计(GB)
func UpdateProxyTrafficGB(id int, gb uint64) bool {
	if gb == 0 {
		return true
	}
	result := db.Model(&conf.ProxyConfig{}).
		Where("id = ?", id).
		UpdateColumn("total_gigabyte", gorm.Expr("total_gigabyte + ?", gb))
	if result.Error != nil {
		log.Println("更新流量统计失败:", result.Error)
		return false
	}
	return true
}

// GetSubscription 获取订阅配置
func GetSubscription(proxyId int) conf.Subscription {
	var sub conf.Subscription
	db.Model(&conf.Subscription{}).Where("proxy_id = ?", proxyId).First(&sub)
	return sub
}

// GetSubscriptionByToken 通过token获取订阅
func GetSubscriptionByToken(token string) conf.Subscription {
	var sub conf.Subscription
	result := db.Model(&conf.Subscription{}).Where("access_token = ?", token).First(&sub)
	if result.Error != nil {
		// 记录不存在，返回空结构体
		return conf.Subscription{}
	}
	return sub
}

// AddSubscription 添加订阅
func AddSubscription(sub conf.Subscription) int {
	// 检查是否已存在
	var existing conf.Subscription
	result := db.Model(&conf.Subscription{}).Where("proxy_id = ?", sub.ProxyId).First(&existing)
	if result.Error == nil && existing.Id > 0 {
		// 更新token
		db.Model(&conf.Subscription{}).Where("id = ?", existing.Id).Update("access_token", sub.AccessToken)
		return existing.Id
	}

	// 创建新订阅
	tx := db.Begin()
	if tx.Error != nil {
		log.Println("开启事务失败")
		return 0
	}

	if err := tx.Create(&sub).Error; err != nil {
		log.Println("创建订阅失败:", err)
		tx.Rollback()
		return 0
	}

	tx.Commit()
	return sub.Id
}

// UpdateSubscriptionStats 更新订阅统计
func UpdateSubscriptionStats(token string, clientCount int, traffic uint64) bool {
	result := db.Model(&conf.Subscription{}).
		Where("access_token = ?", token).
		Updates(map[string]interface{}{
			"client_count":  clientCount,
			"total_traffic": traffic,
		})
	if result.Error != nil {
		log.Println("更新订阅统计失败:", result.Error)
		return false
	}
	return true
}

// DeleteSubscription 删除订阅
func DeleteSubscription(proxyId int) bool {
	if err := db.Where("proxy_id = ?", proxyId).Delete(&conf.Subscription{}).Error; err != nil {
		log.Println("删除订阅失败:", err)
		return false
	}
	return true
}

// CheckProxyPortAvailable 检查代理端口是否可用
func CheckProxyPortAvailable(port int, excludeId int) bool {
	var count int64
	query := db.Model(&conf.ProxyConfig{}).Where("inbound_port = ? AND status = 0", port)
	if excludeId > 0 {
		query = query.Where("id != ?", excludeId)
	}
	query.Count(&count)
	return count == 0
}

// CheckHy2Socks5PortAvailable 检查Hysteria2 SOCKS5端口是否可用
func CheckHy2Socks5PortAvailable(port int, excludeId int) bool {
	var count int64
	query := db.Model(&conf.ProxyConfig{}).Where("outbound_type = ? AND hy2_socks5_port = ? AND status = 0", "hysteria2", port)
	if excludeId > 0 {
		query = query.Where("id != ?", excludeId)
	}
	query.Count(&count)
	return count == 0
}

// GetProxyByPort 根据端口获取代理配置
func GetProxyByPort(port int) conf.ProxyConfig {
	var proxy conf.ProxyConfig
	result := db.Model(&conf.ProxyConfig{}).Where("inbound_port = ?", port).First(&proxy)
	if result.Error != nil {
		// 记录不存在，返回空结构体
		return conf.ProxyConfig{}
	}
	return proxy
}

// GetProxyStats 获取代理统计信息
func GetProxyStats() map[string]interface{} {
	var total, active int64
	db.Model(&conf.ProxyConfig{}).Count(&total)
	db.Model(&conf.ProxyConfig{}).Where("status = 0").Count(&active)

	var bytesSum uint64
	var gigSum uint64
	db.Model(&conf.ProxyConfig{}).Select("COALESCE(SUM(total_bytes),0)").Scan(&bytesSum)
	db.Model(&conf.ProxyConfig{}).Select("COALESCE(SUM(total_gigabyte),0)").Scan(&gigSum)

	totalTrafficBytes := bytesSum + gigSum*gigabyte

	return map[string]interface{}{
		"total":                  total,
		"active":                 active,
		"total_traffic":          FormatTraffic(totalTrafficBytes),
		"total_traffic_bytes":    totalTrafficBytes,
		"total_traffic_gigabyte": gigSum,
	}
}

// GetDB 获取数据库实例（供其他包使用）
func GetDB() *gorm.DB {
	return db
}

// FormatTraffic 格式化流量显示
func FormatTraffic(bytes uint64) string {
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
