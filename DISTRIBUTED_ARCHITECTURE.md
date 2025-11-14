# goForward 2.0 分布式架构文档

## 概述

goForward 2.0 采用分布式架构设计，实现控制端-生产端分离，支持多节点管理和自动化运维。

## 架构设计

### 核心组件

#### 1. 控制端（Control Plane）
- **职责**: 中心化管理节点
- **功能**:
  - 节点注册和认证
  - 配置分发和版本管理
  - 节点健康监控
  - 状态聚合和决策

#### 2. 生产端（Agent/Worker）
- **职责**: 边缘执行节点
- **功能**:
  - 代理服务运行
  - 健康状态采集
  - 实时配置应用
  - 故障自动恢复

### 通信协议

使用 **gRPC** 实现高效的双向流通信：

#### 1. RegisterNode - 节点注册
- Agent 启动时向控制端注册
- 返回控制令牌用于后续认证
- 记录节点基本信息（IP、主机名、版本）

#### 2. Heartbeat - 心跳保活（双向流）
- Agent 定期上报健康状态
- 包含 CPU、内存、磁盘、网络指标
- 控制端监控节点存活状态（90秒超时）

#### 3. StreamConfig - 配置分发（双向流）
- 支持实时配置下发
- 操作类型：get（获取）、update（更新）、delete（删除）
- 增量更新和版本管理

#### 4. ReportStatus - 状态上报
- Agent 主动上报代理运行状态
- 包含代理列表、连接数、流量统计
- 控制端分析并推荐操作（restart、update_config）

## 目录结构

```
/root/port-forward/
├── proto/                    # 协议定义
│   ├── control.proto        # gRPC协议定义
│   ├── control.pb.go        # 生成的Protobuf代码
│   └── control_grpc.pb.go   # 生成的gRPC存根代码
│
├── control/                  # 控制端
│   └── server/
│       ├── grpc_server.go   # gRPC服务器实现
│       └── grpc_test.go     # 集成测试
│
├── agent/                    # 生产端
│   └── client/
│       └── grpc_client.go   # Agent客户端实现
│
└── TODO.md                   # 开发计划
```

## 核心数据结构

### NodeInfo - 节点信息
```protobuf
message NodeInfo {
  string node_id = 1;           // 节点唯一ID
  string hostname = 2;          // 主机名
  string ip_address = 3;        // IP地址
  string version = 4;           // Agent版本
  int64  uptime = 5;            // 运行时间（秒）
  map<string, string> labels = 6; // 标签
}
```

### ProxyConfig - 代理配置
```protobuf
message ProxyConfig {
  int32 id = 1;                 // 配置ID
  string name = 2;              // 配置名称
  string outbound_type = 3;     // 出站类型
  int32 inbound_port = 4;       // 入站端口
  string target_server = 5;     // 目标服务器
  int32 target_port = 6;        // 目标端口
  map<string, string> params = 7; // 其他参数
}
```

### NodeHealth - 健康状态
```protobuf
message NodeHealth {
  int64 cpu_percent = 1;        // CPU使用率
  int64 memory_percent = 2;     // 内存使用率
  int64 disk_percent = 3;       // 磁盘使用率
  int32 active_connections = 4; // 活跃连接数
  double network_rx = 5;        // 网络接收（KB/s）
  double network_tx = 6;        // 网络发送（KB/s）
}
```

## 关键特性

### 1. 高可用性
- **心跳保活**: 90秒超时检测，节点失联自动标记
- **故障切换**: 检测到代理异常时推荐重启操作
- **健康检查**: 定期收集节点健康指标

### 2. 配置管理
- **版本控制**: 支持配置的历史版本管理
- **增量更新**: 只下发变更的配置
- **原子操作**: 配置更新是原子性的

### 3. 安全机制
- **Token认证**: 节点注册���获得控制令牌
- **元数据传递**: 每次请求携带认证信息
- **加密通信**: gRPC默认支持TLS加密

### 4. 监控体系
- **实时指标**: CPU、内存、磁盘、网络
- **代理状态**: 运行状态、连接数、流量
- **错误追踪**: 详细的错误信息和堆栈

## 开发进度

### Phase 1: 基础通信层 ✅
- [x] 定义gRPC协议（proto文件）
- [x] 实现控制端框架
- [x] 实现Agent客户端
- [x] 集成测试验证

### Phase 2: 节点管理（计划中）
- [ ] 节点生命周期管理
- [ ] 配置分发系统
- [ ] 故障节点隔离

### Phase 3: 高可用与扩展（计划中）
- [ ] 多控制节点支持
- [ ] 负载均衡
- [ ] 故障切换

### Phase 4: Web管理界面（计划中）
- [ ] 控制端Web UI
- [ ] 节点列表和状态
- [ ] 配置管理界面
- [ ] 监控面板

## 使用说明

### 启动控制端
```go
server := server.NewControlServer()
if err := server.Start(":8889"); err != nil {
    log.Fatal(err)
}
```

### 启动Agent客户端
```go
agent := client.NewAgentClient("localhost:8889", "node-001")
if err := agent.Connect(); err != nil {
    log.Fatal(err)
}

// 注册节点
if err := agent.RegisterNode(); err != nil {
    log.Fatal(err)
}

// 启动心跳
if err := agent.StartHeartbeat(); err != nil {
    log.Fatal(err)
}

// 获取配置
configs, err := agent.RequestConfig()
if err != nil {
    log.Fatal(err)
}
```

## 测试验证

运行集成测试：
```bash
cd control/server
go test -v -run TestControlServer
```

测试覆盖：
- ✅ 节点注册和认证
- ✅ 心跳保活通信（双向流）
- ✅ 配置管理（增删改查）
- ✅ 状态上报和错误处理
- ✅ 性能基准测试

## 后续规划

Phase 1 Week 2 将实现：
1. **Web管理界面基础框架**
2. **配置持久化存储**
3. **节点分组和标签系统**
4. **批量操作支持**

## 贡献指南

开发建议：
1. 每个新特性添加对应的单元测试和集成测试
2. 遵循 gRPC 最佳实践，使用双向流处理实时通信
3. 保持日志详细但不过量，区分 info/warn/error 级别
4. 节点健康指标采集应该是轻量级的，不影响性能

## 许可证

本项目采用与 goForward 1.0 相同的许可证。