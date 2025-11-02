package sql

// PendingStats 包装待更新的统计数据
// 位置：sql包中定义，forward包引用，避免循环依赖
type PendingStats struct {
	Bytes uint64 // 字节数
	GB    uint64 // 千兆字节数（累计）
}
