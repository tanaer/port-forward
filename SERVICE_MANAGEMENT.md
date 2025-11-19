# goForward v2.0.0 服务管理指南

## 📋 快速开始

### 启动服务

```bash
./manage_v2.sh start
```

### 停止服务

```bash
./manage_v2.sh stop
```

### 重启服务

```bash
./manage_v2.sh restart
```

### 查看状态

```bash
./manage_v2.sh status
```

### 查看实时日志

```bash
./manage_v2.sh logs
```

### 清理被占用的端口

```bash
./manage_v2.sh clean
```

---

## 🛠️ 服务管理脚本

### manage_v2.sh - 综合管理工具

**功能最完整的管理脚本，推荐使用**

```bash
# 帮助信息
./manage_v2.sh help

# 启动
./manage_v2.sh start

# 停止
./manage_v2.sh stop

# 重启
./manage_v2.sh restart

# 状态
./manage_v2.sh status

# 日志
./manage_v2.sh logs

# 清理
./manage_v2.sh clean
```

### 各单独脚本说明

| 脚本 | 功能 | 说明 |
|------|------|------|
| `manage_v2.sh` | 综合管理 | **推荐使用，功能最完整** |
| `start_v2.sh` | 启动服务 | 简单启动脚本 |
| `stop_v2.sh` | 停止服务 | 优雅停止 + 强制杀死 |
| `restart_v2.sh` | 重启服务 | 依次调用 stop 和 start |
| `status_v2.sh` | 检查状态 | 查看运行状态和端口 |

---

## 📊 服务状态检查

### 查看完整状态

```bash
./manage_v2.sh status
```

**输出示例**：
```
📌 进程状态:
   ✅ 运行中 (PID: 857472)

🔌 端口监听状态:
   ✅ Web 管理界面 (端口 8890)
   ✅ Prometheus 指标 (端口 9090)
   ✅ gRPC 服务 (端口 50051)

🌐 服务可访问性:
   ✅ Web 管理界面 (8890): 可访问
   ✅ Prometheus 指标: 可访问
```

### 各服务说明

| 服务 | 默认端口 | 用途 |
|------|---------|------|
| Web 管理界面 | 8890 (可配置) | v2.0.0 分布式管理界面 |
| gRPC 控制端 | 50051 | 代理节点通信 |
| Prometheus | 9090 | 指标收集和监控 |

---

## 🔄 常见操作场景

### 场景 1: 修改配置后重启

```bash
# 1. 编辑配置
nano goforward.yaml

# 2. 重启服务
./manage_v2.sh restart

# 3. 验证变更
./manage_v2.sh status
```

### 场景 2: 查看实时日志

```bash
# 显示实时日志（按 Ctrl+C 退出）
./manage_v2.sh logs
```

### 场景 3: 解决端口被占用问题

```bash
# 查看哪些端口被占用
./manage_v2.sh status

# 清理所有被占用的端口
./manage_v2.sh clean

# 重新启动
./manage_v2.sh start
```

### 场景 4: 监控 Prometheus 指标

```bash
# 查看 Prometheus 是否可访问
curl http://localhost:9090/metrics

# 或在浏览器打开
open http://localhost:9090/metrics
```

---

## 📁 日志文件

实时日志文件位置：

```bash
/tmp/goforward_v2.log
```

### 查看日志

```bash
# 实时查看
tail -f /tmp/goforward_v2.log

# 查看最后 100 行
tail -100 /tmp/goforward_v2.log

# 全文查看
cat /tmp/goforward_v2.log
```

---

## 💾 数据库

SQLite 数据库位置：

```bash
/tmp/goForward_control.db
```

### 数据库操作

```bash
# 查看数据库大小
du -h /tmp/goForward_control.db

# 备份数据库
cp /tmp/goForward_control.db /tmp/goForward_control.db.backup

# 还原数据库
cp /tmp/goForward_control.db.backup /tmp/goForward_control.db
```

---

## 🐛 故障排查

### Q: 服务启动失败

**查看日志**：
```bash
tail -100 /tmp/goforward_v2.log
```

**常见原因**：
- 端口被占用 → 使用 `./manage_v2.sh clean`
- 配置文件错误 → 检查 `goforward.yaml`
- 权限问题 → 运行 `chmod +x *.sh`

### Q: Prometheus 无法访问

```bash
# 检查状态
./manage_v2.sh status

# 测试连接
curl http://localhost:9090/metrics

# 查看日志
tail -50 /tmp/goforward_v2.log | grep -i prometheus
```

### Q: gRPC 端口被占用

```bash
# 清理端口
./manage_v2.sh clean

# 或手动查看占用情况
ss -tlnp | grep 50051
lsof -i :50051
```

### Q: 无法优雅关闭

```bash
# 强制清理所有端口
./manage_v2.sh clean

# 或手动强制杀死进程
pkill -9 -f "goForward_v2"
```

---

## ✅ 已修复的问题

### 1. Prometheus 指标无法访问 ✅

**原因**：`StartMetricsServer` 重复初始化指标导致注册错误

**修复**：
- 移除 `StartMetricsServer` 中重复的 `InitRecorder()` 调用
- main_v2.go 已在启动时初始化指标

**验证**：
```bash
./manage_v2.sh status  # 应显示 ✅ Prometheus 指标: 可访问
curl http://localhost:9090/metrics  # 应返回指标数据
```

### 2. gRPC 端口被占用问题 ✅

**解决方案**：
- `manage_v2.sh clean` 命令可清理被占用的端口
- `stop_v2.sh` 脚本确保彻底关闭进程

---

## 🎯 生产环境建议

### 后台运行

```bash
# 后台启动服务
nohup ./goForward_v2 > /tmp/goforward_v2.log 2>&1 &

# 或使用 manage 脚本
./manage_v2.sh start
```

### 自动重启 (systemd)

创建 `/etc/systemd/system/goforward-v2.service`：

```ini
[Unit]
Description=goForward v2.0.0 Distributed Control Plane
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/port-forward
ExecStart=/root/port-forward/goForward_v2
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启用：
```bash
systemctl enable goforward-v2
systemctl start goforward-v2
systemctl status goforward-v2
```

### 监控告警

结合 Prometheus 和 Grafana：

```bash
# Prometheus scrape config (prometheus.yml)
scrape_configs:
  - job_name: 'goforward'
    static_configs:
      - targets: ['localhost:9090']
```

---

## 📞 技术支持

遇到问题时：

1. 查看日志：`tail -f /tmp/goforward_v2.log`
2. 检查状态：`./manage_v2.sh status`
3. 清理重启：`./manage_v2.sh clean && ./manage_v2.sh start`
4. 检查配置：`cat goforward.yaml`

---

**最后更新**: 2025-11-19

