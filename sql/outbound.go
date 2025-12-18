package sql

import (
	"errors"
	"goForward/conf"
)

// GetOutboundList 获取所有出站配置列表
func GetOutboundList() ([]conf.OutboundConfig, error) {
	var list []conf.OutboundConfig
	result := db.Order("status ASC, id DESC").Find(&list)
	return list, result.Error
}

// GetOutbound 获取单个出站配置
func GetOutbound(id int) (*conf.OutboundConfig, error) {
	var outbound conf.OutboundConfig
	result := db.First(&outbound, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &outbound, nil
}

// GetActiveOutbound 获取当前启用的出站配置
func GetActiveOutbound() (*conf.OutboundConfig, error) {
	var outbound conf.OutboundConfig
	result := db.Where("status = ?", 0).First(&outbound)
	if result.Error != nil {
		return nil, result.Error
	}
	return &outbound, nil
}

// AddOutbound 添加出站配置
func AddOutbound(outbound *conf.OutboundConfig) (int, error) {
	// 检查名称是否重复
	var count int64
	db.Model(&conf.OutboundConfig{}).Where("name = ?", outbound.Name).Count(&count)
	if count > 0 {
		return 0, errors.New("出站配置名称已存在")
	}

	// 如果设置为启用，先禁用其他所有配置
	if outbound.Status == 0 {
		db.Model(&conf.OutboundConfig{}).Where("status = ?", 0).Update("status", 1)
	}

	result := db.Create(outbound)
	if result.Error != nil {
		return 0, result.Error
	}
	return outbound.Id, nil
}

// UpdateOutbound 更新出站配置
func UpdateOutbound(outbound *conf.OutboundConfig) error {
	// 检查名称是否重复（排除自己）
	var count int64
	db.Model(&conf.OutboundConfig{}).Where("name = ? AND id != ?", outbound.Name, outbound.Id).Count(&count)
	if count > 0 {
		return errors.New("出站配置名称已存在")
	}

	// 如果设置为启用，先禁用其他所有配置
	if outbound.Status == 0 {
		db.Model(&conf.OutboundConfig{}).Where("status = ? AND id != ?", 0, outbound.Id).Update("status", 1)
	}

	return db.Save(outbound).Error
}

// DeleteOutbound 删除出站配置
func DeleteOutbound(id int) error {
	// 检查是否有代理正在使用此出站配置
	var count int64
	db.Model(&conf.ProxyConfig{}).Where("outbound_config_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("该出站配置正在被代理使用，无法删除")
	}

	return db.Delete(&conf.OutboundConfig{}, id).Error
}

// SetOutboundActive 设置指定出站配置为启用状态（其他自动禁用）
func SetOutboundActive(id int) error {
	// 先禁用所有
	if err := db.Model(&conf.OutboundConfig{}).Where("status = ?", 0).Update("status", 1).Error; err != nil {
		return err
	}
	// 启用指定的
	return db.Model(&conf.OutboundConfig{}).Where("id = ?", id).Update("status", 0).Error
}

// SetOutboundInactive 禁用指定出站配置
func SetOutboundInactive(id int) error {
	return db.Model(&conf.OutboundConfig{}).Where("id = ?", id).Update("status", 1).Error
}

// GetOutboundCount 获取出站配置数量
func GetOutboundCount() int64 {
	var count int64
	db.Model(&conf.OutboundConfig{}).Count(&count)
	return count
}

// GetActiveOutboundCount 获取启用的出站配置数量
func GetActiveOutboundCount() int64 {
	var count int64
	db.Model(&conf.OutboundConfig{}).Where("status = ?", 0).Count(&count)
	return count
}
