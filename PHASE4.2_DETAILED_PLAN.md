# Phase 4.2 可观测性与配置化增强 - 详细规划

**版本**: v1.7.2.0
**分支**: v2.0-develop
**计划周期**: 4-5 天
**状态**: 📋 规划中

---

## 📊 Phase 4.2 概述

### 核心目标

在 Phase 4.1.1 生产级容错机制的基础上，添加**完整的可观测性**和**参数配置化**，使系统达到**企业级运维要求**。

### 为什么需要 Phase 4.2？

**Phase 4.1.1 的限制**:
1. ❌ 无法看到系统状态（只有日志）
2. ❌ 参数硬编码（无法调优）
3. ❌ Failed 任务未隔离（无死信队列）
4. ❌ 无告警机制（故障不能及时发现）

**Phase 4.2 解决方案**:
1. ✅ Prometheus 指标导出
2. ✅ Grafana 实时仪表板
3. ✅ 参数配置化（命令行+配置文件）
4. ✅ 死信队列（DLQ）实现

---

## 🎯 三个主要工作流

### 工作流 1: 可观测性 (Observability)

**交付物**:
- `control/metrics/prometheus.go` - Prometheus 指标定义
- `control/metrics/recorder.go` - 指标记录器
- Grafana Dashboard JSON 模板
- 告警规则配置

**关键指标**:
```
回滚系统:
  ├─ rollback_task_total{status}         - 任务总数
  ├─ rollback_task_duration_seconds      - 执行时长
  ├─ rollback_task_retries               - 重试次数分布
  ├─ rollback_timeout_resets_total       - 超时重置次数
  └─ rollback_failed_total               - 失败任务数

gRPC 服务:
  ├─ grpc_server_handled_total{method}   - RPC 调用总数
  ├─ grpc_server_handling_seconds        - RPC 执行时间
  └─ grpc_server_msg_received/sent       - 消息统计

系统资源:
  ├─ process_cpu_seconds_total           - CPU 时间
  ├─ process_resident_memory_bytes       - 内存使用
  └─ process_goroutines                  - Goroutine 数量
```

**仪表板示例**:
```
┌─────────────────────────────────────────┐
│   goForward 回滚系统监控 Dashboard       │
├─────────────────────────────────────────┤
│ 任务成功率: 98.5%  | 平均耗时: 2.3s    │
│ 今日失败数: 2      | 超时重置: 5次     │
├─────────────────────────────────────────┤
│ [任务执行时间趋势图]                    │
│ [重试次数分布直方图]                    │
│ [节点可用性饼图]                        │
│ [告警历史表格]                          │
└─────────────────────────────────────────┘
```

---

### 工作流 2: 参数配置化 (Configuration)

**交付物**:
- `config/config.go` - 配置管理
- `config/defaults.go` - 默认值
- `goforward.yaml` - 配置文件模板
- 命令行参数扩展

**可配置参数**:

```yaml
rollback:
  max_retries: 5              # 最大重试次数
  processing_timeout: 600     # Processing超时(秒)
  stalled_scan_interval: 60   # 超时扫描间隔(秒)
  enabled: true               # 是否启用回滚系统

metrics:
  enabled: true               # 是否启用指标导出
  port: 9090                  # Prometheus端口
  scrape_interval: 15s        # 采集间隔

logging:
  level: info                 # 日志级别(debug/info/warn/error)
  format: json                # 日志格式(json/text)
  output: stdout              # 输出位置(stdout/file)
```

**命令行参数**:
```bash
./goForward \
  -port 8889 \
  -pass 123456 \
  -metrics-port 9090 \
  -max-retries 5 \
  -processing-timeout 600 \
  -log-level info \
  -config /etc/goforward.yaml
```

---

### 工作流 3: 死信队列 (DLQ)

**交付物**:
- Migration v8: 创建 `rollback_tasks_dlq` 表
- `control/store/dlq_dao.go` - DLQ 数据访问
- DLQ 管理接口（查看、重放、删除）

**表结构**:
```sql
CREATE TABLE rollback_tasks_dlq (
  id BIGINT PRIMARY KEY,
  original_task_id BIGINT,
  node_id VARCHAR(255),
  config_id INT,
  target_version INT,
  status VARCHAR(50),      -- failed, timeout, error
  failure_reason TEXT,
  retry_count INT,
  moved_to_dlq_at BIGINT,
  dlq_expiry_at BIGINT,    -- 30天后自动清理
  metadata JSON
);
```

**工作流**:
```
任务标记为 Failed
  ↓
转移到 DLQ
  ↓
运维人员分析原因
  ↓
选择:
  ├─ 重放到原队列  (修复后)
  ├─ 手动处理
  └─ 永久删除
```

---

## 📁 文件结构规划

```
control/
├─ metrics/
│  ├─ prometheus.go          (新建) - Prometheus 指标定义
│  ├─ recorder.go            (新建) - 指标记录器
│  └─ types.go               (新建) - 指标类型
├─ config/
│  ├─ config.go              (新建) - 配置结构体
│  ├─ loader.go              (新建) - 配置加载
│  ├─ defaults.go            (新建) - 默认值
│  └─ validator.go           (新建) - 配置验证
├─ store/
│  ├─ dlq_dao.go             (新建) - DLQ 数据访问
│  ├─ migration.go           (修改) - Migration v8
│  └─ rollback_task_dao.go   (修改) - 添加 DLQ 移动逻辑
├─ server/
│  ├─ grpc_server.go         (修改) - 集成 Prometheus
│  ├─ lifecycle.go           (修改) - 配置集成
│  └─ metrics_handler.go     (新建) - /metrics 端点
└─ web/
   ├─ web_server.go          (修改) - 添加 DLQ API
   └─ dlq_handler.go         (新建) - DLQ 管理路由

config/
├─ goforward.yaml            (新建) - 配置文件模板
└─ defaults.yaml             (新建) - 默认配置

grafana/
├─ rollback-system.json      (新建) - Dashboard JSON
└─ alerts.json               (新建) - 告警规则
```

---

## 🔧 实现细节

### 1. Prometheus 指标实现

**核心类**:
```go
type RollbackMetrics struct {
    totalTasks              prometheus.Counter
    taskDuration            prometheus.Histogram
    retryCount              prometheus.Histogram
    timeoutResetsTotal      prometheus.Counter
    failedTotal             prometheus.Counter
}

func (m *RollbackMetrics) RecordTaskStart(taskID int64) {}
func (m *RollbackMetrics) RecordTaskSuccess(taskID int64, duration time.Duration) {}
func (m *RollbackMetrics) RecordTaskFailed(taskID int64, reason string) {}
func (m *RollbackMetrics) RecordTimeout(taskID int64) {}
```

**集成点**:
- `grpc_server.go`: StreamConfig() 中记录任务指标
- `rollback_task_dao.go`: UpdateTaskStatus() 中记录失败
- `grpc_server.go`: monitorStalledTasks() 中记录超时

---

### 2. 配置加载实现

**配置优先级** (从高到低):
```
命令行参数 > 环境变量 > 配置文件 > 默认值
```

**加载顺序**:
```go
func LoadConfig(args []string) *Config {
    // 1. 加载默认值
    cfg := defaultConfig()

    // 2. 加载配置文件
    if configFile := getConfigFile(args); configFile != "" {
        cfg.MergeFile(configFile)
    }

    // 3. 加载环境变量
    cfg.MergeEnv()

    // 4. 加载命令行参数
    cfg.MergeArgs(args)

    // 5. 验证
    cfg.Validate()

    return cfg
}
```

---

### 3. DLQ 实现

**迁移 v8** (migration.go):
```go
func (m *Migrator) addDLQTable(db *sql.DB) error {
    // 创建 DLQ 表
    // 创建相关索引
    // 设置过期清理触发器
}
```

**DLQ DAO**:
```go
type DLQDAO struct {
    db *sql.DB
}

func (dao *DLQDAO) MoveToDLQ(taskID int64, reason string) error {}
func (dao *DLQDAO) ListDLQTasks(limit int) ([]*DLQTask, error) {}
func (dao *DLQDAO) ReplayFromDLQ(taskID int64) error {}
func (dao *DLQDAO) DeleteFromDLQ(taskID int64) error {}
```

**Web API**:
```go
GET  /api/dlq/tasks              - 列出 DLQ 任务
GET  /api/dlq/tasks/:id          - 查看详情
POST /api/dlq/tasks/:id/replay   - 重放任务
DELETE /api/dlq/tasks/:id        - 删除任务
```

---

## 📋 实现清单

### Phase 4.2.1 - 可观测性基础 (2-3 天)

- [ ] 定义 Prometheus 指标
- [ ] 实现指标记录器
- [ ] 集成到 gRPC 服务
- [ ] 集成到回滚系统
- [ ] 创建 /metrics 端点
- [ ] 编写指标测试
- [ ] 创建 Grafana Dashboard

### Phase 4.2.2 - 参数配置化 (1-2 天)

- [ ] 设计配置结构体
- [ ] 实现配置加载器
- [ ] 支持 YAML 配置文件
- [ ] 支持命令行参数
- [ ] 支持环境变量
- [ ] 参数验证
- [ ] 编写配置测试

### Phase 4.2.3 - 死信队列实现 (1-2 天)

- [ ] Migration v8: 创建 DLQ 表
- [ ] 实现 DLQ DAO
- [ ] 实现转移逻辑
- [ ] 实现 DLQ 管理 API
- [ ] 实现重放逻辑
- [ ] 实现过期清理
- [ ] 编写 DLQ 测试

### Phase 4.2.4 - 集成与测试 (1 天)

- [ ] 集成 Prometheus + Grafana
- [ ] 端到端集成测试
- [ ] 性能基准测试
- [ ] 压力测试
- [ ] 文档编写

---

## 📊 工作量估算

| 任务 | 估计 | 说明 |
|------|------|------|
| 可观测性 | 2-3 天 | Prometheus + Grafana |
| 参数配置 | 1-2 天 | 配置管理系统 |
| DLQ实现 | 1-2 天 | 死信队列 |
| 集成测试 | 1 天 | 完整验证 |
| **总计** | **5-8 天** | **预留缓冲** |

---

## ✅ Phase 4.2 验收标准

### 功能验收

- [ ] Prometheus 能正确导出所有指标
- [ ] Grafana Dashboard 实时显示数据
- [ ] 所有参数能通过配置文件配置
- [ ] 命令行参数优先级正确
- [ ] DLQ 能正确接收失败任务
- [ ] DLQ 任务能正确重放
- [ ] 过期任务能自动清理

### 性能验收

- [ ] 指标记录不增加 > 1% CPU
- [ ] 不增加显著内存消耗
- [ ] 查询 DLQ 任务 < 100ms
- [ ] 配置加载 < 50ms

### 质量验收

- [ ] 所有新代码通过 go vet
- [ ] 测试覆盖率 ≥ 80%
- [ ] 无 breaking changes
- [ ] 文档完善

---

## 🎯 关键决策

### 1. Prometheus vs 其他方案

**选择 Prometheus 的原因**:
- ✅ 云原生标准（K8s 原生支持）
- ✅ 简单易集成（无需修改系统架构）
- ✅ 生态完整（Grafana 无缝集成）
- ✅ 零额外依赖（Go 原生库支持）

### 2. 配置优先级顺序

**选择的顺序** (命令行 > 环境变量 > 文件 > 默认):
- ✅ 用户友好（命令行优先）
- ✅ 容器友好（环境变量支持）
- ✅ 生产就绪（配置文件支持）
- ✅ 开箱即用（默认值完整）

### 3. DLQ 保留时间

**选择 30 天**:
- ✅ 足够长（生产事件分析）
- ✅ 不过长（磁盘占用可控）
- ✅ 符合审计要求
- ✅ 可配置调整

---

## 📈 预期收益

### 对运维的价值

✅ **可视化**: 一目了然地看到系统状态
✅ **告警**: 自动发现问题而不是被动应对
✅ **调优**: 数据驱动的参数调整
✅ **故障排查**: DLQ 帮助快速定位问题

### 对开发的价值

✅ **指标齐全**: 充分的系统洞察
✅ **灵活配置**: 无需重新编译
✅ **容器友好**: 环境变量配置
✅ **测试友好**: Prometheus 库有完整的测试工具

### 对用户的价值

✅ **可靠性**: 故障自动恢复 (Phase 4.1.1 + DLQ)
✅ **透明性**: 完整的系统可观测
✅ **可维护性**: 参数灵活配置
✅ **企业级**: 达到生产级质量

---

## 🚀 后续规划

### Phase 4.3 (未来)

- UI 管理界面增强
- DLQ 可视化管理
- 告警规则自定义
- 指标导出增强 (OpenTelemetry)

### Phase 5 (未来)

- Agent 执行反馈
- 完整的链路追踪
- 分布式追踪集成

---

## 📝 开发顺序建议

1. **先做可观测性** (Prometheus)
   - 理由: 其他工作都能从中受益

2. **再做参数配置** (Configuration)
   - 理由: 可观测性依赖配置（指标开关）

3. **最后做 DLQ**
   - 理由: 完整的故障处理流程

---

## 💡 技术亮点

1. **零修改集成**
   - Prometheus 指标无需修改现有接口
   - 配置管理与现有逻辑解耦

2. **向后兼容**
   - 默认配置与 Phase 4.1.1 相同
   - 现有部署无需改动

3. **生产友好**
   - Prometheus 是业界标准
   - Grafana 是通用工具
   - DLQ 是常见模式

---

## 成功标志

✅ Grafana 仪表板实时显示系统状态
✅ 通过参数文件调整系统行为
✅ Failed 任务可查看和重放
✅ 告警规则能及时发现问题

---

**下一步**: 从 Phase 4.2.1 开始实施

---

## 附录: Grafana Dashboard 示例

```json
{
  "dashboard": {
    "title": "goForward 回滚系统",
    "panels": [
      {
        "title": "任务成功率",
        "targets": [
          "rate(rollback_task_total{status='completed'}[5m])"
        ]
      },
      {
        "title": "平均执行时间",
        "targets": [
          "histogram_quantile(0.95, rollback_task_duration_seconds)"
        ]
      },
      {
        "title": "失败任务数",
        "targets": [
          "increase(rollback_failed_total[1h])"
        ]
      },
      {
        "title": "超时重置次数",
        "targets": [
          "increase(rollback_timeout_resets_total[1h])"
        ]
      }
    ]
  }
}
```

---

**规划完成日期**: 2025-11-19
**计划开始日期**: 2025-11-20 (预计)
**目标完成日期**: 2025-11-24 (预计)
