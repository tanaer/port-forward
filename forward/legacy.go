package forward

// 全局统计聚合器实例
var globalAggregator *StatsAggregator

// InitStatsAggregator 初始化全局统计聚合器
func InitStatsAggregator() {
	if globalAggregator == nil {
		globalAggregator = NewStatsAggregator()
	}
}

// ShutdownStatsAggregator 关闭全局统计聚合器
func ShutdownStatsAggregator() {
	if globalAggregator != nil {
		globalAggregator.Stop()
		globalAggregator = nil
	}
}

// UpdateForwardBytes 更新字节数（向后兼容）
// 这是现有代码调用的接口，现在委托给聚合器处理
func UpdateForwardBytes(id int, bytes uint64) error {
	if globalAggregator == nil {
		return nil
	}
	globalAggregator.Add(id, bytes, 0)
	return nil
}

// UpdateForwardGb 更新GB数（向后兼容）
// 这是现有代码调用的接口，现在委托给聚合器处理
func UpdateForwardGb(id int, gb uint64) error {
	if globalAggregator == nil {
		return nil
	}
	globalAggregator.Add(id, 0, gb)
	return nil
}
