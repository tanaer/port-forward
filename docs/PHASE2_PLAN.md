# Phase 2: 节点生命周期管理开发计划

## 开始时间
2025-11-15

## 目标
实现完整的节点生命周期管理和配置分发系统，为生产环境的多节点管理提供可靠支持。

---

## 现有基础分析

### ✅ 已完成的基础设施（Phase 1）
1. **gRPC 通信层**
   - `RegisterNode` - 节点注册
   - `Heartbeat` - 双向流心跳保活
   - `StreamConfig` - 双向流配置分发
   - `ReportStatus` - 状态上报

2. **数据存储层**
   - `nodes` 表 - 节点信息（含 node_group, tags）
   - `proxy_configs` 表 - 代理配置（含 version, config_group）
   - `node_logs` 表 - 节点日志
   - 完整的 DAO 层和批量操作

3. **节点注册表**
   - `NodeRegistry` - 内存节点管理
   - `NodeInfo` - 节点状态追踪
   - `healthCheckTicker` - 30秒定时健康检查

4. **健康检查机制**
   - 90秒心跳超时检测
   - CPU/内存/磁盘使用率监控
   - 自动标记失联节点

### 🎯 需要增强的功能

#### 3.1 节点生命周期管理
- [ ] **自动发现新节点** - 已有 RegisterNode，需增强通知机制
- [ ] **节点健康检查** - 已有基础，需增强告警和自动恢复
- [ ] **故障节点隔离** - 需实现自动隔离和手动恢复

#### 3.2 配置分发系统
- [ ] **增量配置更新** - StreamConfig 已实现，需优化增量逻辑
- [ ] **配置版本管理** - Version 字段已预留，需实现版本追踪
- [ ] **回滚机制** - 需实现配置快照和回滚逻辑

---

## Phase 2.1: 节点生命周期管理（3天）

### 任务 1: 增强节点自动发现（0.5天）
**目标**: 新节点注册时自动通知管理员和触发初始化流程

**实现**:
1. 在 `RegisterNode` 中添加新节点检测逻辑
2. 通过 WebSocket 实时推送新节点通知
3. 记录节点注册日志到 `node_logs`
4. 添加节点注册事件钩子（可扩展）

**文件**:
- `control/server/grpc_server.go` - 增强 RegisterNode
- `control/server/lifecycle.go` - 新建生命周期管理器

**验证**:
- 新节点注册时 WebSocket 收到通知
- 日志正确记录注册事件

---

### 任务 2: 增强节点健康检查（1天）
**目标**: 完善健康检查机制，支持多级告警和自动恢复

**实现**:
1. **健康状态分级**
   - `healthy` - 所有指标正常
   - `warning` - 单项指标超过阈值（CPU>80%, Mem>80%, Disk>85%）
   - `critical` - 多项指标超过阈值或心跳超时
   - `offline` - 心跳超时 90 秒

2. **告警机制**
   - 状态变化时记录日志
   - 通过 WebSocket 推送告警
   - 支持告警阈值配置

3. **自动恢复检测**
   - 节点从 `offline` 恢复时自动标记为 `healthy`
   - 记录恢复事件

**文件**:
- `control/server/lifecycle.go` - 健康检查逻辑
- `control/server/grpc_server.go` - 更新 checkNodeHealth
- `control/store/database.go` - 添加健康状态字段（可选）

**验证**:
- 模拟节点 CPU 超过 80%，收到 warning 告警
- 模拟节点离线，90 秒后标记为 offline
- 节点恢复后自动标记为 healthy

---

### 任务 3: 实现故障节点隔离（1天）
**目标**: 自动隔离故障节点，防止影响整体服务

**实现**:
1. **自动隔离策略**
   - 节点 `offline` 超过 5 分钟自动隔离
   - 节点连续 3 次健康检查失败自动隔离
   - 隔离后停止向该节点分发配置

2. **手动隔离/恢复**
   - 添加 API: `POST /api/nodes/:id/isolate`
   - 添加 API: `POST /api/nodes/:id/recover`
   - 记录隔离/恢复操作日志

3. **隔离状态管理**
   - 添加 `isolated` 状态
   - 隔离节点在 UI 中特殊标记
   - 支持批量隔离操作

**文件**:
- `control/server/lifecycle.go` - 隔离逻辑
- `control/web/web_server.go` - 隔离/恢复 API
- `control/store/node_dao.go` - 隔离状态持久化

**验证**:
- 节点离线 5 分钟后自动隔离
- 手动隔离节点成功
- 隔离节点不再接收配置更新

---

### 任务 4: 节点生命周期事件系统（0.5天）
**目标**: 统一的事件系统，支持扩展和审计

**实现**:
1. **事件类型定义**
   - `NodeRegistered` - 节点注册
   - `NodeHealthChanged` - 健康状态变化
   - `NodeIsolated` - 节点隔离
   - `NodeRecovered` - 节点恢复
   - `NodeOffline` - 节点离线

2. **事件处理器**
   - 日志记录处理器
   - WebSocket 通知处理器
   - 数据库持久化处理器

3. **事件订阅机制**
   - 支持多个处理器订阅同一事件
   - 异步事件处理，不阻塞主流程

**文件**:
- `control/server/events.go` - 新建事件系统
- `control/server/lifecycle.go` - 集成事件系统

**验证**:
- 节点注册时触发 NodeRegistered 事件
- 事件被所有订阅者正确处理

---

## Phase 2.2: 配置分发系统（2天）

### 任务 5: 实现增量配置更新（0.5天）
**目标**: 只推送变化的配置，减少网络开销

**实现**:
1. **配置变更检测**
   - 比较配置 Version 字段
   - 计算配置 Hash 值
   - 只推送变化的配置项

2. **增量推送逻辑**
   - Agent 上报当前配置版本
   - 控制端计算差异
   - 只推送新增/修改/删除的配置

**文件**:
- `control/server/config_manager.go` - 新建配置管理器
- `control/server/grpc_server.go` - 更新 StreamConfig

**验证**:
- 配置未变化时不推送
- 只推送变化的配置项

---

### 任务 6: 实现配置版本管理（0.5天）
**目标**: 追踪配置历史，支持版本回滚

**实现**:
1. **配置版本追踪**
   - 每次配置更新自动递增 Version
   - 记录配置变更历史

2. **配置快照**
   - 保存每个版本的完整配置
   - 支持查询历史版本

3. **版本比较**
   - 对比两个版本的差异
   - 生成变更报告

**文件**:
- `control/store/config_version_dao.go` - 新建版本 DAO
- `control/store/migration.go` - 添加 config_versions 表
- `control/web/web_server.go` - 版本查询 API

**验证**:
- 配置更新后 Version 自动递增
- 可以查询历史版本
- 版本比较功能正常

---

### 任务 7: 实现配置回滚机制（1天）
**目标**: 支持一键回滚到历史版本

**实现**:
1. **回滚 API**
   - `POST /api/configs/:id/rollback` - 回滚到指定版本
   - 验证目标版本存在
   - 创建回滚操作记录

2. **回滚逻辑**
   - 从快照恢复配置
   - 推送到所有相关节点
   - 记录回滚事件

3. **回滚验证**
   - 回滚前备份当前配置
   - 回滚后验证配置生效
   - 支持回滚失败时恢复

**文件**:
- `control/server/config_manager.go` - 回滚逻辑
- `control/web/web_server.go` - 回滚 API
- `control/store/config_version_dao.go` - 版本恢复

**验证**:
- 回滚到历史版本成功
- 配置正确推送到节点
- 回滚操作被正确记录

---

## 数据库变更

### 新增表

#### 1. config_versions（配置版本表）
```sql
CREATE TABLE config_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    version INTEGER NOT NULL,
    config_snapshot TEXT NOT NULL,  -- JSON 格式的完整配置
    change_type VARCHAR(32) NOT NULL,  -- create, update, delete
    change_summary TEXT,
    created_by VARCHAR(128),
    created_at INTEGER NOT NULL,
    FOREIGN KEY (config_id) REFERENCES proxy_configs(id) ON DELETE CASCADE
);

CREATE INDEX idx_config_versions_config_id ON config_versions(config_id);
CREATE INDEX idx_config_versions_version ON config_versions(version);
```

#### 2. node_events（节点事件表）
```sql
CREATE TABLE node_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(64) NOT NULL,  -- registered, health_changed, isolated, recovered, offline
    event_data TEXT,  -- JSON 格式的事件详情
    created_at INTEGER NOT NULL,
    FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
);

CREATE INDEX idx_node_events_node_id ON node_events(node_id);
CREATE INDEX idx_node_events_event_type ON node_events(event_type);
CREATE INDEX idx_node_events_created_at ON node_events(created_at);
```

### 表结构修改

#### nodes 表添加字段
```sql
ALTER TABLE nodes ADD COLUMN health_status VARCHAR(32) DEFAULT 'unknown';  -- healthy, warning, critical, offline
ALTER TABLE nodes ADD COLUMN isolated BOOLEAN DEFAULT 0;  -- 是否被隔离
ALTER TABLE nodes ADD COLUMN isolated_at INTEGER;  -- 隔离时间
ALTER TABLE nodes ADD COLUMN isolated_reason TEXT;  -- 隔离原因
```

---

## 文件结构

```
/root/port-forward/
├── control/
│   ├── server/
│   │   ├── grpc_server.go          # 现有：gRPC 服务器
│   │   ├── lifecycle.go            # 新增：生命周期管理器
│   │   ├── events.go               # 新增：事件系统
│   │   └── config_manager.go       # 新增：配置管理器
│   ├── store/
│   │   ├── config_version_dao.go   # 新增：配置版本 DAO
│   │   ├── node_event_dao.go       # 新增：节点事件 DAO
│   │   └── migration.go            # 更新：添加新表迁移
│   └── web/
│       └── web_server.go           # 更新：添加新 API
└── PHASE2_PLAN.md                  # 本文件
```

---

## 测试计划

### 单元测试
- [ ] `lifecycle_test.go` - 生命周期管理器测试
- [ ] `events_test.go` - 事件系统测试
- [ ] `config_manager_test.go` - 配置管理器测试
- [ ] `config_version_dao_test.go` - 配置版本 DAO 测试

### 集成测试
- [ ] 节点注册到隔离的完整流程
- [ ] 配置创建到回滚的完整流程
- [ ] 多节点并发场景测试

### 性能测试
- [ ] 100 个节点同时心跳
- [ ] 1000 次配置更新性能
- [ ] 事件系统吞吐量测试

---

## 里程碑

### Milestone 1: 节点生命周期管理（Day 1-3）
- ✅ 增强节点自动发现
- ✅ 增强节点健康检查
- ✅ 实现故障节点隔离
- ✅ 节点生命周期事件系统

### Milestone 2: 配置分发系统（Day 4-5）
- ✅ 实现增量配置更新
- ✅ 实现配置版本管理
- ✅ 实现配置回滚机制

### Milestone 3: 测试和文档（Day 6）
- ✅ 完整的单元测试
- ✅ 集成测试
- ✅ API 文档
- ✅ 用户手册

---

## 风险和依赖

### 风险
1. **性能风险**: 大量节点同时心跳可能影响性能
   - 缓解: 使用连接池，优化心跳处理逻辑

2. **数据一致性**: 配置回滚时可能出现不一致
   - 缓解: 使用事务，添加验证机制

3. **事件风暴**: 大量事件可能阻塞系统
   - 缓解: 异步处理，添加事件队列

### 依赖
1. Phase 1 Week 2 的所有功能必须稳定
2. Agent 端需要支持配置版本上报（可选）
3. WebSocket 连接稳定性

---

## 成功标准

### 功能完整性
- ✅ 所有 Phase 2 任务完成
- ✅ 所有测试通过
- ✅ 文档完整

### 性能指标
- ✅ 支持 100+ 节点同时在线
- ✅ 心跳处理延迟 < 100ms
- ✅ 配置推送延迟 < 1s

### 可靠性指标
- ✅ 节点离线检测准确率 100%
- ✅ 配置回滚成功率 > 99%
- ✅ 事件处理无丢失

---

**计划制定时间**: 2025-11-15  
**预计完成时间**: 2025-11-21  
**负责人**: Claude Code
