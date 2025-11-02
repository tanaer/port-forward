# StatsAggregator API 约定（最终版）

## 背景
解决plan.md中StatsAggregator定义不一致的问题，统一使用`PendingStats`结构体。

## 核心定义

```go
type StatsAggregator struct {
    mu      sync.RWMutex
    pending map[int]*PendingStats  // ✓ 统一使用 PendingStats
    ticker  *time.Ticker
}

type PendingStats struct {
    Bytes uint64  // ✓ 字段名：Bytes
    GB    uint64  // ✓ 字段名：GB
}
```

## 关键方法

```go
// 添加统计数据（累加逻辑）
func (sa *StatsAggregator) Add(id int, bytes uint64, gb uint64) {
    sa.mu.Lock()
    defer sa.mu.Unlock()

    if existing, ok := sa.pending[id]; ok {
        existing.Bytes += bytes
        existing.GB += gb
    } else {
        sa.pending[id] = &PendingStats{Bytes: bytes, GB: gb}
    }
}

// 批量刷新（带错误回退）
func (sa *StatsAggregator) flush() {
    sa.mu.Lock()
    defer sa.mu.Unlock()

    if len(sa.pending) == 0 {
        return
    }

    // 优先批量更新
    if err := sql.BatchUpdateStats(sa.pending); err != nil {
        // 回退到逐个更新
        for id, stats := range sa.pending {
            if err := sql.UpdateForwardDirect(id, stats.Bytes, stats.GB); err != nil {
                return // 保留失败条目下次重试
            }
        }
    }

    sa.pending = make(map[int]*PendingStats)
}
```

## 与 sql 包的协作

### sql/batch.go（新增）
```go
func BatchUpdateStats(stats map[int]*PendingStats) error {
    tx := db.Begin()
    for id, stats := range stats {
        if err := tx.Exec("UPDATE ...", stats.Bytes, stats.GB, id).Error; err != nil {
            tx.Rollback()
            return err
        }
    }
    return tx.Commit().Error
}
```

### sql/direct.go（新增）
```go
func UpdateForwardDirect(id int, bytes uint64, gb uint64) error {
    tx := db.Begin()
    if err := tx.Exec("UPDATE ...", bytes, gb, id).Error; err != nil {
        tx.Rollback()
        return err
    }
    return tx.Commit().Error
}
```

## 向后兼容

### forward/legacy.go（新增）
```go
var globalAggregator *StatsAggregator

func UpdateForwardBytes(id int, bytes uint64) error {
    globalAggregator.Add(id, bytes, 0)
    return nil
}

func UpdateForwardGb(id int, gb uint64) error {
    globalAggregator.Add(id, 0, gb)
    return nil
}
```

## 实施检查清单

- [ ] 使用 `PendingStats` 而非 `TrafficStats`
- [ ] 字段名为 `Bytes` 和 `GB`（非 `bytes` 和 `Gigabyte`）
- [ ] 创建 `sql/batch.go` 和 `sql/direct.go`
- [ ] 创建 `forward/legacy.go` 适配层
- [ ] 编译通过且无未使用导入

---

**更新日期**: 2025-11-02
**状态**: 已统一，可直接执行
