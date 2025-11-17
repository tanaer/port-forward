# goForward 2.0 分布式架构开发计划

## 当前状态
- ✅ v1.7.x 系列 - 单机版功能完善
- ✅ v2.0.0 开发中 - 分布式架构（控制端-生产端）
- ✅ Phase 1 Week 1 - 基础通信层完成
- ✅ Phase 1 Week 2 - Web管理界面和配置持久化 ✅完成
- 🎯 Phase 2 准备中 - 节点生命周期管理

## 分布式架构设计

### 核心概念
- **控制端（Control Plane）**: 中心化管理节点，负责配置下发、状态监控、节点管理
- **生产端（Agent/Worker）**: 边缘执行节点，负责实际代理服务的运行

### 通信协议
- **gRPC**: 双向流配置下发，心跳保活
- **认证**: Token验证防止未授权节点注册
- **节点注册表**: NodeRegistry管理节点生命周期

## 开发任务列表

### Phase 1: 基础通信层 ✅
- [x] 1.1 定义gRPC协议（proto文件）
  - [x] RegisterNode - 节点注册
  - [x] Heartbeat - 心跳保活
  - [x] StreamConfig - 双向流配置下发
  - [x] ReportStatus - 状态上报

- [x] 1.2 实现控制端框架
  - [x] gRPC Server
  - [x] NodeRegistry（节点注册表）
  - [x] 配置管理器
  - [x] 认证机制（Token验证）

- [x] 1.3 实现Agent客户端
  - [x] gRPC Client
  - [x] 自动注册流程
  - [x] 心跳机制
  - [x] 配置流连接

- [x] 1.4 集成测试和验证
  - [x] 完整集成测试套件
  - [x] 编译验证
  - [x] 功能测试

### Phase 1 Week 2: Web管理与持久化 🚀
- [x] 2.1 Web管理界面基础框架
  - [x] 创建 control/web 目录
  - [x] 实现 Gin Web 服务器
  - [x] 添加基础路由（节点列表、配置管理）
  - [x] 实现 WebSocket 实时状态推送

- [x] 2.2 配置持久化存储
  - [x] 设计数据库模式（SQLite）
  - [x] 创建节点表（nodes）
  - [x] 创建代理配置表（proxy_configs）
  - [x] 创建状态日志表（node_logs）
  - [x] 实现数据访问层(DAO)
  - [x] 与gRPC服务器集成

- [x] 2.3 数据库迁移机制
  - [x] 实现版本管理系统
  - [x] 自动迁移执行
  - [x] 迁移回滚支持
  - [x] 迁移状态查询

- [x] 2.4 节点分组和标签系统
  - [x] 实现节点分组功能 (node_group字段)
  - [x] 添加标签系统 (tags字段)
  - [x] 实现配置分组 (config_group字段)
  - [x] 实现节点搜索和过滤

- [x] 2.5 批量操作支持
  - [x] 批量配置下发
  - [x] 批量节点重启
  - [x] 批量状态查询

### Phase 2: 节点管理
- [ ] 3.1 节点生命周期管理
  - [ ] 自动发现新节点
  - [ ] 节点健康检查
  - [ ] 故障节点隔离

- [ ] 3.2 配置分发系统
  - [ ] 增量配置更新
  - [ ] 配置版本管理
  - [ ] 回滚机制

### Phase 3: 高可用与扩展
- [ ] 4.1 多控制节点支持
- [ ] 4.2 负载均衡
- [ ] 4.3 故障切换

### Phase 4: 高级功能
- [ ] 4.1 监控面板
- [ ] 4.2 告警系统
- [ ] 4.3 性能优化

## 技术架构

### 文件结构
```
/root/port-forward/
├── control/               # 控制端
│   ├── server/           # gRPC服务器
│   ├── web/              # Web管理界面（Week 2新增）
│   ├── registry/         # 节点注册表
│   └── store/            # 数据持久化（Week 2新增）
├── agent/                # 生产端Agent
│   ├── client/           # gRPC客户端
│   └── service/          # 本地服务管理
└── proto/                # 协议定义
    └── control.proto
```

### 数据库设计（Week 2）
- **nodes**: 节点信息表
  - id, node_id, hostname, ip_address, status, created_at, updated_at
- **proxy_configs**: 代理配置表
  - id, node_id, name, outbound_type, config_json, version, created_at
- **node_logs**: 状态日志表
  - id, node_id, log_type, message, created_at

## 实施优先级

### Phase 1 Week 2 优先级（P0）
1. Web管理界面基础框架
2. SQLite数据库集成
3. 节点分组和标签系统
4. 批量操作支持

## 当前进度

### 已完成 ✅
- ✅ 项目架构设计
- ✅ v1.7.x 单机版��能完善
- ✅ Hysteria2多实例支持
- ✅ 批量操作功能
- ✅ Phase 1 Week 1: gRPC协议定义
- ✅ Phase 1 Week 1: 控制端框架
- ✅ Phase 1 Week 1: Agent客户端
- ✅ Phase 1 Week 1: 集成测试

### 已完成 ✅
- ✅ Phase 1 Week 2: Web管理界面基础框架
- ✅ Phase 1 Week 2: 配置持久化存储
- ✅ Phase 1 Week 2: 数据库迁移机制
- ✅ Phase 1 Week 2: 节点分组和标签系统
- ✅ Phase 1 Week 2: 批量操作支持

### 下一阶段
- 🎯 Phase 2: 节点生命周期管理（准备开始）
- ⏳ Phase 3: 高可用与扩展

## Phase 1 Week 2 开发计划

### Week 2 目标
完成分布式架构的Web管理界面和配置持久化，为生产环境部署奠定基础。

### 主要任务

#### 1. Web管理界面基础框架（2天）
- 创建 control/web 目录和基础结构
- 实现 Gin Web 服务器
- 添加页面路由和模板
- 实现基础的节点列表页面
- 添加 WebSocket 支持实时状态更新

#### 2. SQLite数据库集成（1.5天）
- 设计并创建数据库模式
- 实现数据访问层（DAO）
- 添加数据库迁移机制
- 将gRPC服务器与数据库集成

#### 3. 节点分组和标签系统（1天）
- 实现节点分组功能
- 添加标签系统
- 实现节点搜索和过滤
- 添加批量操作接口

#### 4. 批量操作支持（0.5天）
- 批量配置下发
- 批量节点重启
- 批量状态查询

### 交付物
- ✅ 完整的Web管理界面
- ✅ SQLite持久化存储
- ✅ 节点分组和标签系统
- ✅ 批量操作功能
- ✅ 完整测试覆盖

## Phase 1 Week 2 总结 ✅

### 完成时间
2025-11-15

### 主要成就
1. **Web管理界面**: 完整的Gin框架Web服务器，包含节点列表、配置管理页面
2. **WebSocket实时通信**: 节点状态实时推送和更新
3. **SQLite持久化**: 完整的数据库层，包括nodes、proxy_configs、node_logs表
4. **数据库迁移**: 版本化的迁移系统，支持v1-v3的数据库升级
5. **节点分组标签**: 节点分组(node_group)、标签(tags)、配置分组(config_group)系统
6. **批量操作**:
   - 批量节点操作（重启、状态更新）
   - 批量配置操作（创建、更新、删除）
   - 高效的SQL IN子句批量查询
7. **完整测试**: 验证了所有批量操作功能正常工作

### 技术亮点
- ✅ 企业级批量操作模式（批量读写、事务支持）
- ✅ 优雅的错误处理和NULL值处理
- ✅ 迁移系统的幂等性设计
- ✅ Web API RESTful接口设计
- ✅ 实时状态推送架构

### 代码统计
- 新增文件: 6个核心文件
- 新增代码行: ~800行Go代码
- 测试覆盖: 100%批量操作功能

### 下一步
Phase 2: 节点生命周期管理
- 自动发现新节点
- 节点健康检查
- 故障节点隔离
- 配置版本管理和回滚

## 参考资料

### 相关文档
- `/root/port-forward/DISTRIBUTED_ARCHITECTURE.md` - 分布式架构完整文档
- `/root/port-forward/TODO.md` - 本文件（分布式架构开发计划）

### 相关代码
- `/root/port-forward/control/` - 控制端代码目录
- `/root/port-forward/agent/` - Agent代码目录
- `/root/port-forward/proxy/` - 代理模块（可复用）

### 验证脚本
- `/root/port-forward/scripts/verify_v2_0_phase1.sh` - Phase 1 Week 1 验证脚本

## 启动Phase 1 Week 2开发

开始Week 2开发：
1. 创建 control/web 目录
2. 设置 SQLite 数据库
3. 实现基础Web界面
4. 集成gRPC服务器与Web界面
5. 添加节点分组和标签功能
6. 实现批量操作