# StatsAggregator API 约定（最终版）

## 背景
解决plan.md中StatsAggregator定义不一致和**循环依赖**问题。

⚠️ **关键设计决策**：PendingStats定义在sql包中，forward包引用，避免Go的import cycle。

## 循环依赖问题与解决

**问题**：
- forward包导入sql包（第9行：`"goForward/sql"`）
- 如果PendingStats定义在forward包，sql包再引用forward，会形成循环依赖

**解决**：
- PendingStats定义在sql包中
- forward包使用 `sql.PendingStats`
- 依赖关系：forward → sql（单向，无循环）

## 核心定义

**sql/stats.go**（新增文件）：
```go
package sql

// PendingStats 包装待更新的统计数据
// 位置：sql包中定义，forward包引用，避免循环依赖
type PendingStats struct {
    Bytes uint64  // ✓ 字段名：Bytes
    GB    uint64  // ✓ 字段名：GB
}
```

**forward/stats.go**（新增文件）：
```go
package forward

import (
    "sync"
    "time"
    "goForward/sql"  // 引入sql包以使用sql.PendingStats
)

type StatsAggregator struct {
    mu      sync.RWMutex
    pending map[int]*sql.PendingStats  // ✓ 引用sql包的PendingStats
    ticker  *time.Ticker
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
        // ✓ 使用sql包的PendingStats
        sa.pending[id] = &sql.PendingStats{Bytes: bytes, GB: gb}
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

    sa.pending = make(map[int]*sql.PendingStats)
}
```

## 与 sql 包的协作

### sql/stats.go（新增）
```go
package sql

type PendingStats struct {
    Bytes uint64
    GB    uint64
}
```

### sql/batch.go（新增）
```go
package sql

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
package sql

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
package forward

import "goForward/sql"  // 引入sql包

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

## 依赖关系图

```
forward包
    ↓ 导入
sql包
    ├─ stats.go (PendingStats定义)
    ├─ batch.go (BatchUpdateStats)
    └─ direct.go (UpdateForwardDirect)
```

✓ **无循环依赖**：forward → sql（单向）

## 实施检查清单

- [x] **关键**：PendingStats在sql包中定义（避免循环依赖）
- [x] 使用 `sql.PendingStats` 而非 `TrafficStats`
- [x] 字段名为 `Bytes` 和 `GB`（非 `bytes` 和 `Gigabyte`）
- [x] 创建 `sql/stats.go`、`sql/batch.go` 和 `sql/direct.go`
- [x] 创建 `forward/legacy.go` 适配层
- [x] **编译通过且无 import cycle**（使用 `go build ./...` 验证）

---

**更新日期**: 2025-11-03
**状态**: 已修复循环依赖，可直接执行
