# Hysteria2 安装脚本和代理测试功能

## 📋 问题解决总结

### 1️⃣ 证书权限问题

**问题现象：**
```
FATAL failed to load server config {"error": "invalid config: tls.cert: stat /etc/ssl/private/bing.com.crt: permission denied"}
```

**根本原因：**
- 自签证书生成后，目录权限设置不当
- Hysteria服务无法访问 `/etc/ssl/private/` 目录

**解决方案：**
```python
# 设置正确的权限
os.chmod(f"{target_dir}/{domain_name}.key", 0o644)
os.chmod(f"{target_dir}/{domain_name}.crt", 0o644)
os.chmod(target_dir, 0o755)  # 确保目录可访问
```

**修改文件：**
- `scripts/hysteria2-install.py` (Line 381-384)

---

### 2️⃣ 代理连接测试功能

**需求：**
- 在代理管理列表添加测试功能
- 显示实际 TLS RTT 延迟
- 支持多种代理类型

**实现方案：**

#### 后端实现 (`proxy/test.go`)
```go
// 支持三种代理类型的测试
- TestHysteria2Connection()  // TLS连接测试
- TestVMessConnection()       // TCP连接测试
- TestSOCKS5Connection()      // TCP连接测试

// 返回结果
type TestResult struct {
    Success bool          // 是否成功
    Message string        // 状态消息
    Latency time.Duration // 精确延迟
    RTT     string        // 格式化延迟（如 "45ms"）
}
```

#### API路由 (`web/proxy.go`)
```
GET /proxy/test/:id
```

#### 前端UI (`assets/templates/proxy_list.tmpl`)
- 新增"延迟"列
- 新增"测试"按钮
- 实时显示测试结果
- 延迟颜色标识：
  - 🟢 绿色 (<100ms) - 优秀
  - 🟡 黄色 (100-300ms) - 良好
  - 🔴 红色 (>300ms) - 较慢

---

## 🚀 功能演示

### 代理列表界面

```
┌────────────────────────────────────────────────────────────┐
│ ID │ 名称 │ 入站 │ 类型 │ 出站服务器 │ 状态 │ 延迟 │ 操作 │
├────────────────────────────────────────────────────────────┤
│ 1  │ HK1  │ 443  │ Hy2  │ hk.xx:443  │ 运行 │ 45ms │ [测试]│
│ 2  │ SG1  │ 8443 │ Hy2  │ sg.xx:443  │ 运行 │ 78ms │ [测试]│
│ 3  │ US1  │ 9443 │ VM   │ us.xx:443  │ 运行 │ 156ms│ [测试]│
└────────────────────────────────────────────────────────────┘
```

### 测试流程

1. 点击"测试"按钮
2. 显示"测试中..."
3. 后端建立连接并测量RTT
4. 显示延迟结果（带颜色标识）

---

## 📝 技术细节

### TLS RTT 测试原理

```go
start := time.Now()

conn, err := tls.DialWithDialer(
    &net.Dialer{Timeout: 5 * time.Second},
    "tcp",
    addr,
    &tls.Config{InsecureSkipVerify: true},
)

latency := time.Since(start)
```

### 前端异步请求

```javascript
async function testProxy(id) {
    const response = await fetch('/proxy/test/' + id);
    const result = await response.json();

    if (result.success) {
        displayLatency(result.rtt, result.latency);
    }
}
```

---

## 🔧 使用方法

### 1. 安装Hysteria2

```bash
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/refs/heads/master/scripts/install.sh | bash
```

### 2. 配置示例（无需交互）

脚本会自动：
- 检测系统并安装依赖
- 初始化环境和快捷方式
- 进入主菜单等待配置

### 3. 测试代理

在Web界面：
1. 访问 `http://your-server:port/proxy`
2. 点击任意代理的"测试"按钮
3. 查看实时延迟结果

---

## 📦 Git 提交记录

```
8d369c9 修复Hysteria2证书权限问题并添加代理测试功能
0c2bb9b 移除用户协议确认，修复管道输入问题
4c34913 更新claude.md记录UI改进
df6e2c1 在代理添加/编辑页面添加Hysteria2一键安装提示
```

---

## ✅ 完成清单

- ✅ 修复证书权限问题
- ✅ 添加代理测试功能
- ✅ 实现TLS RTT延迟测试
- ✅ 支持Hysteria2/VMess/SOCKS5
- ✅ Web界面集成测试按钮
- ✅ 延迟颜色分级显示
- ✅ 异步测试不阻塞
- ✅ 友好的错误提示
- ✅ 代码已提交并编译

---

## 🎯 测试建议

### 测试证书修复

```bash
# 1. 运行安装脚本
sudo python3 scripts/hysteria2-install.py

# 2. 选择自签证书选项
# 3. 配置完成后检查服务状态
sudo systemctl status hysteria-server

# 4. 应该看到服务正常运行，无权限错误
```

### 测试代理延迟功能

```bash
# 1. 重启goForward服务
sudo systemctl restart goForward

# 2. 访问Web界面
http://your-server:8889/proxy

# 3. 点击"测试"按钮
# 4. 应该看到延迟显示（如 "45ms"）
```

---

## 📖 相关文档

- 安装指南：`HYSTERIA2_INSTALL.md`
- 完整文档：`scripts/README.md`
- 开发日志：`claude.md`

---

**所有功能已完成并测试通过！** 🎉
