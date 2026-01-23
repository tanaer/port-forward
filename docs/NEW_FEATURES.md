# 🎉 新功能说明

## 功能更新

### 1. 随机端口生成 ✅
- **监听端口默认随机生成**（10000-65535）
- 自动检测端口可用性
- 避免端口冲突

### 2. 出站类型选择 ✅
- **Hysteria2 出站**：通过 Hysteria2 高速协议连接上游
- **SOCKS5 出站**：直接连接到 SOCKS5 代理服务器
- **Web界面支持**：添加和编辑代理时可选择出站类型

### 3. 界面优化 ✅
- 首页改名为 **"端口隧道"**（原端口转发功能）
- 新增 **"代理转发"** 入口按钮
- 新增 **"环境配置"** 入口按钮
- 代理列表显示出站类型标签（Hysteria2/SOCKS5）

## 使用说明

### 添加代理配置

#### 方式一：使用 Hysteria2 出站（默认）

1. 进入代理管理
2. 点击"添加代理"
3. **入站配置**：
   - 监听端口：留空自动生成随机端口（10000+）
   - 一键生成密钥
   - 选择 Reality 域名
4. **出站配置**：
   - 出站类型：选择 **Hysteria2**
   - 粘贴 Hysteria2 订阅链接
   - 点击"解析订阅"自动填充
5. 保存并启动

**数据流向**：
```
客户端 → VLESS+Reality(Xray) → Hysteria2客户端 → Hysteria2服务器 → 互联网
```

#### 方式二：使用 SOCKS5 出站

1. 进入代理管理
2. 点击"添加代理"
3. **入站配置**：
   - 监听端口：留空自动生成
   - 一键生成密钥
   - 选择 Reality 域名
4. **出站配置**：
   - 出站类型：选择 **SOCKS5**
   - SOCKS5 地址：填写 SOCKS5 服务器地址
   - SOCKS5 端口：填写端口（默认 1080）
5. 保存并启动

**数据流向**：
```
客户端 → VLESS+Reality(Xray) → SOCKS5代理 → 互联网
```

### 随机端口说明

- **自动生成范围**：10000-65535
- **检测机制**：自动检测端口是否被占用
- **重试机制**：随机尝试100次，失败后顺序查找
- **手动指定**：也可以手动输入端口号

### 出站类型对比

| 特性 | Hysteria2 | SOCKS5 |
|------|-----------|--------|
| 速度 | 极快（UDP优化） | 一般 |
| 稳定性 | 高 | 中 |
| 配置复杂度 | 需要订阅或手动配置 | 简单 |
| 适用场景 | 高速专线中转 | 连接现有SOCKS5代理 |
| 依赖 | 需要 Hysteria2 客户端 | 无额外依赖 |

### 使用场景

#### 场景一：高速专线中转
```
使用 Hysteria2 出站
- 适合：需要高速、稳定的代理
- 要求：有 Hysteria2 服务器或订阅
```

#### 场景二：连接现有代理
```
使用 SOCKS5 出站
- 适合：已有 SOCKS5 代理服务器
- 要求：SOCKS5 服务器地址和端口
- 示例：连接到 Shadowsocks、V2Ray 等的 SOCKS5 端口
```

#### 场景三：多层代理
```
VLESS+Reality → SOCKS5 → 其他代理 → 互联网
- 适合：需要多层加密或绕过特定限制
```

## 界面导航

### 首页（端口隧道）
```
http://your-server:8889/

功能：
- 查看和管理 TCP/UDP 端口转发
- 快速入口：
  [🚀 代理转发] - 进入代理管理
  [⚙️ 环境配置] - 检查和安装依赖
```

### 代理转发
```
http://your-server:8889/proxy

功能：
- 管理 VLESS+Reality 代理
- 添加/编辑/删除代理
- 生成订阅链接
```

### 环境配置
```
http://your-server:8889/environment

功能：
- 检查 Xray 和 Hysteria2 安装状态
- 一键安装依赖
- 查看版本信息
```

## 配置示例

### 示例 1：Hysteria2 出站

```yaml
入站配置：
  监听端口: 12345 (自动生成)
  UUID: xxx-xxx-xxx
  Reality域名: microsoft.com

出站配置：
  类型: Hysteria2
  服务器: hy2.example.com
  端口: 443
  密码: your-password
```

### 示例 2：SOCKS5 出站

```yaml
入站配置：
  监听端口: 15678 (自动生成)
  UUID: xxx-xxx-xxx
  Reality域名: yahoo.com

出站配置：
  类型: SOCKS5
  地址: 127.0.0.1
  端口: 1080
```

## 技术实现

### 随机端口生成
```go
// 生成 10000-65535 范围内的随机可用端口
port := proxy.GetRandomAvailablePort()
```

### 出站类型判断
```go
if cfg.OutboundType == "socks5" {
    // 直接连接 SOCKS5
    // 只启动 Xray
} else {
    // 使用 Hysteria2
    // 启动 Xray + Hysteria2
}
```

### Xray 配置生成
```go
xray.GenerateVLESSRealityConfig(xray.VLESSRealityConfig{
    Port:         cfg.InboundPort,
    OutboundType: cfg.OutboundType,  // "hysteria2" 或 "socks5"
    Socks5Addr:   cfg.Socks5Addr,
    Socks5Port:   cfg.Socks5Port,
})
```

## 数据库变更

新增字段：
```sql
ALTER TABLE proxy_configs ADD COLUMN outbound_type VARCHAR(20) DEFAULT 'hysteria2';
ALTER TABLE proxy_configs ADD COLUMN socks5_addr VARCHAR(100);
ALTER TABLE proxy_configs ADD COLUMN socks5_port INT DEFAULT 1080;
```

## 注意事项

1. **端口冲突**：
   - 随机端口会自动检测可用性
   - 如果所有端口都被占用，会使用默认端口 10443

2. **SOCKS5 出站**：
   - 不需要安装 Hysteria2
   - 只需要 Xray
   - SOCKS5 服务器必须可访问

3. **Hysteria2 出站**：
   - 需要安装 Hysteria2 客户端
   - 需要有效的 Hysteria2 服务器或订阅

4. **性能考虑**：
   - Hysteria2：适合高带宽场景
   - SOCKS5：适合低延迟场景

## 升级说明

### 从旧版本升级

1. **重新编译**：
   ```bash
   go build -o goForward .
   ```

2. **数据库自动迁移**：
   - 启动时自动添加新字段
   - 现有配置默认使用 Hysteria2 出站

3. **无需额外配置**：
   - 现有代理继续正常工作
   - 新功能向后兼容

## 常见问题

**Q: 随机端口范围可以修改吗？**
A: 可以，修改 `proxy/port.go` 中的范围参数。

**Q: 可以同时使用两种出站类型吗？**
A: 可以，不同的代理配置可以使用不同的出站类型。

**Q: SOCKS5 出站支持认证吗？**
A: 当前版本暂不支持，后续版本会添加。

**Q: 如何查看使用的是哪种出站？**
A: 在代理列表中会显示出站类型。

## 更新日志

### v1.1.0 (2025-01-XX)
- ✅ 新增随机端口生成功能
- ✅ 新增 SOCKS5 出站支持
- ✅ 优化界面导航和命名
- ✅ 改进配置流程

---

**立即体验新功能！** 🚀
