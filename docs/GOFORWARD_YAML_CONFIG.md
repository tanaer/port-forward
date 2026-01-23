# goforward.yaml 配置文件使用指南

## 概述

`goforward.yaml` 是 goForward v2.0.0 的配置文件，用于控制启动参数和系统行为。该文件**完全可用**，所有修改都会在启动时生效。

## 快速测试

### 验证配置是否生效

1. **修改端口配置**：
```yaml
# 修改 goforward.yaml 中的
server:
  port: "8890"  # 改为 8890
```

2. **启动 v2.0.0**：
```bash
./start_v2.sh
```

3. **查看启动日志**，应该看到：
```
✅ 配置加载成功
   - 服务端口: 8890         ← 这证明配置被读取了！
   - 回滚系统: true
   - Prometheus: true (端口 9090)

📍 服务地址:
   🌐 Web管理界面: http://localhost:8890   ← Web 在 8890 启动
```

## 配置文件结构

### server 部分 - 服务器设置
```yaml
server:
  port: "8890"           # Web 管理界面监听端口 (默认: 8889)
  password: ""           # Web 管理界面密码 (可选，不设置则无需密码)
```

**示例**：设置 Web 密码
```yaml
server:
  port: "8889"
  password: "your-secure-password"
```

### rollback 部分 - 回滚系统配置
```yaml
rollback:
  enabled: true                    # 启用/禁用回滚系统
  max_retries: 5                  # 最大重试次数
  processing_timeout: 600s        # 处理超时（10分钟）
  stalled_scan_interval: 60s      # 扫描间隔（1分钟）
```

### metrics 部分 - Prometheus 指标
```yaml
metrics:
  enabled: true           # 启用/禁用 Prometheus 指标导出
  port: "9090"           # Prometheus /metrics 端口
  scrape_interval: 15s   # 采集间隔
```

**示例**：禁用 Prometheus
```yaml
metrics:
  enabled: false
```

### logging 部分 - 日志配置
```yaml
logging:
  level: "info"          # 日志级别：debug, info, warn, error
  format: "text"         # 日志格式：json, text
  output: "stdout"       # 日志输出：stdout, file
```

## 配置优先级（从高到低）

1. **环境变量** (最高优先级，会覆盖其他设置)
2. **YAML 文件** (goforward.yaml)
3. **默认值** (最低优先级)

## 使用环境变量覆盖配置

如果需要用环境变量覆盖 YAML 中的设置：

```bash
# 覆盖端口
export GOFORWARD_PORT=9999

# 覆盖密码
export GOFORWARD_PASSWORD=mypassword

# 禁用 Prometheus
export GOFORWARD_METRICS_ENABLED=false

# 设置日志级别为 debug
export GOFORWARD_LOG_LEVEL=debug

# 然后启动
./goForward_v2
```

## 常见配置场景

### 场景 1: 改变 Web 管理界面端口

**需求**：使用端口 9999 代替 8889

**修改 goforward.yaml**：
```yaml
server:
  port: "9999"
```

**启动**：
```bash
./start_v2.sh
```

**访问**：http://localhost:9999

### 场景 2: 保护 Web 界面

**需求**：为 Web 界面设置密码

**修改 goforward.yaml**：
```yaml
server:
  port: "8889"
  password: "secure123456"
```

### 场景 3: 禁用 Prometheus 指标

**需求**：关闭 Prometheus 数据收集

**修改 goforward.yaml**：
```yaml
metrics:
  enabled: false
```

### 场景 4: 启用调试日志

**需求**：查看详细的调试信息

**修改 goforward.yaml**：
```yaml
logging:
  level: "debug"
  format: "text"
  output: "stdout"
```

## 文件位置和加载顺序

1. v2.0.0 启动时在**当前目录**查找 `goforward.yaml`
2. 如果找到文件，按照 YAML 格式解析配置
3. 文件不存在时使用默认配置（不报错）
4. 环境变量会覆盖文件中的所有配置

## 故障排查

### Q: 修改了 goforward.yaml 但没有生效

A: 检查以下几点：

1. **确认文件已保存**
   ```bash
   cat goforward.yaml | grep "port:"
   ```

2. **确认 v2.0.0 有读取配置**
   ```bash
   ./goForward_v2 2>&1 | head -20
   ```
   看输出中是否显示了 `配置加载成功` 和正确的端口号

3. **检查文件格式**
   YAML 对缩进敏感，使用**空格**而非 TAB：
   ```yaml
   server:
     port: "8890"   # 注意缩进是空格
   ```

4. **检查环境变量**
   如果设置了环境变量，它会覆盖 YAML 文件：
   ```bash
   echo $GOFORWARD_PORT  # 如果有值，它会覆盖 YAML
   ```

### Q: 如何重置到默认配置

A: 删除或重命名 `goforward.yaml` 文件：
```bash
mv goforward.yaml goforward.yaml.bak
./start_v2.sh  # 将使用全部默认配置
```

## 实际验证

本配置已在以下环境验证：

✅ v2.0.0 启动成功
✅ goforward.yaml 配置正确读取
✅ 修改 port 到 "8890" 生效
✅ Web 服务器在配置的端口启动
✅ 所有其他配置项正常工作

**测试日期**: 2025-11-19

---

## 更多帮助

- 详细的 v2.0.0 使用指南: `V2.0.0_USAGE.md`
- 项目完整文档: `V2.0.0_PROJECT_SUMMARY.md`
- 启动脚本: `start_v2.sh`
