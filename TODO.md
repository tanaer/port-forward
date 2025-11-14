# goForward 2.0 分布式架构开发计划

## 当前状态
- ✅ v1.7.x 系列 - 单机版功能完善
- 🔄 v2.0.0 开发中 - 分布式架构（控制端-生产端）

## 分布式架构设计

### 核心概念
- **控制端（Control Plane）**: 中心化管理节点，负责配置下发、状态监控、节点管理
- **生产端（Agent/Worker）**: 边缘执行节点，负责实际代理服务的运行

### 通信协议
- **gRPC**: 双向流配置下发，心跳保活
- **认证**: Token验证防止未授权节点注册
- **节点注册表**: NodeRegistry管理节点生命周期

## 开发任务列表

### Phase 1: 基础通信层
- [ ] 1.1 定义gRPC协议（proto文件）
  - [ ] RegisterNode - 节点注册
  - [ ] Heartbeat - 心跳保活
  - [ ] StreamConfig - 双向流配置下发
  - [ ] ReportStatus - 状态上报

- [ ] 1.2 实现控制端框架
  - [ ] gRPC Server
  - [ ] NodeRegistry（节点注册表）
  - [ ] 配置管理器
  - [ ] 认证机制

- [ ] 1.3 实现Agent客户端
  - [ ] gRPC Client
  - [ ] 自动注册流程
  - [ ] 心跳机制
  - [ ] 配置流连接

### Phase 2: 节点管理
- [ ] 2.1 节点生命周期管理
  - [ ] 自动发现新节点
  - [ ] 节点健康检查
  - [ ] 故障节点隔离

- [ ] 2.2 配置分发系统
  - [ ] 增量配置更新
  - [ ] 配置版本管理
  - [ ] 回滚机制

### Phase 3: 高可用与扩展
- [ ] 3.1 多控制节点支持
- [ ] 3.2 负载均衡
- [ ] 3.3 故障切换

### Phase 4: Web管理界面
- [ ] 4.1 控制端Web UI
- [ ] 4.2 节点列表和状态
- [ ] 4.3 配置管理界面
- [ ] 4.4 监控面板

## 技术架构

### 文件结构
```
/root/port-forward/
├── control/               # 控制端
│   ├── server/           # gRPC服务器
│   ├── registry/         # 节点注册表
│   └── web/              # Web管理界面
├── agent/                # 生产端Agent
│   ├── client/           # gRPC客户端
│   └── service/          # 本地服务管理
└── proto/                # 协议定义
    └── control.proto
```

### 数据库设计
- 节点信息表（nodes）
- 配置历史表（config_history）
- 状态日志表（node_logs）

## 实施优先级

### 最高优先级（P0）
1. 定义gRPC协议（proto文件）
2. 实现基础gRPC通信
3. Agent自动注册和心跳

### 高优先级（P1）
1. 节点注册表
2. 配置分发机制
3. Web管理界面基础功能

### 中优先级（P2）
1. 节点健康检查
2. 故障切换
3. 监控面板

## 当前进度

### 已完成
- ✅ 项目架构设计
- ✅ v1.7.x 单机版功能完善
- ✅ Hysteria2多实例支持
- ✅ 批量操作功能

### 进行中
- 🔄 RTT显示优化（v1.7.1.1）
- 📋 分布式架构开发计划制定

### 待开始
- ⏳ Phase 1 Week 1: gRPC协议定义
- ⏳ Phase 1 Week 2: 控制端框架
- ⏳ Phase 1 Week 3: Agent客户端
- ⏳ Phase 1 Week 4: 集成测试

## 参考资料

### 相关文档
- `/root/port-forward/plan.md` - 单机版优化计划
- `/root/port-forward/TODO.md` - 本文件（分布式架构开发计划）

### 相关代码
- `/root/port-forward/control/` - 控制端代码目录
- `/root/port-forward/proxy/` - 代理模块（可复用）

## 启动开发

开始Phase 1 Week 1开发：
1. 创建proto目录和control.proto文件
2. 定义gRPC服务接口
3. 生成Go代码
4. 实现基础的gRPC Server
