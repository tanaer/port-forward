# Hysteria2 一键安装脚本

> 🚀 Hysteria2 代理服务器一键安装、配置和管理工具（优化版）

## 特性

✨ **安全优化**
- 移除第三方配置泄露风险
- 使用隐私友好的订阅转换服务
- 集成依赖安装，无需额外脚本

🎯 **功能完整**
- 支持自动ACME证书申请
- 支持自签名证书（无需域名）
- 支持端口跳跃、混淆、协议嗅探
- 自动生成 Clash、Sing-box、Xray 配置

📦 **易于使用**
- 交互式配置向导
- 清晰的菜单界面
- 完整的错误提示
- 一键安装部署

## 快速开始

### 方法一：一键安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/master/scripts/install.sh | bash
```

### 方法二：直接运行

```bash
# 下载脚本
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/master/scripts/hysteria2-install.py -o hy2.py

# 运行
sudo python3 hy2.py
```

### 方法三：克隆仓库

```bash
git clone https://github.com/tanaer/port-forward.git
cd port-forward/scripts
sudo python3 hysteria2-install.py
```

## 使用说明

### 安装 Hysteria2

1. 运行脚本后选择 `1. 安装/更新 Hysteria2`
2. 选择最新版本或指定版本
3. 按照配置向导完成设置

### 配置选项

#### 1. 证书选择

- **自动申请域名证书**：需要域名和邮箱，自动通过ACME协议申请证书
- **自签名证书**：无需域名，适合个人使用
- **手动指定证书**：使用已有证书

#### 2. 高级功能

- **Brutal 模式**：拥塞控制算法，可能提升速度
- **混淆模式**：使用 Salamander 混淆，降低特征
- **协议嗅探**：自动识别并路由不同协议
- **端口跳跃**：使用端口范围，增强抗封锁能力

### 服务管理

```bash
# 启动服务（开机自启）
sudo systemctl enable --now hysteria-server.service

# 停止服务
sudo systemctl stop hysteria-server.service

# 重启服务
sudo systemctl restart hysteria-server.service

# 查看状态
sudo systemctl status hysteria-server.service

# 查看日志
sudo journalctl -u hysteria-server.service -f
```

### 快捷命令

安装后可以使用快捷命令 `hy2` 直接启动管理界面：

```bash
sudo hy2
```

## 配置文件位置

- **主配置**：`/etc/hysteria/config.yaml`
- **订阅链接**：`/etc/hy2config/hy2_url_scheme.txt`
- **Clash 配置**：`/etc/hy2config/clash.yaml`
- **Sing-box 配置**：`/etc/hy2config/singbox.yaml`
- **Xray 配置**：`/etc/hy2config/xray.yaml`

## 客户端配置

### 方式一：扫描二维码

脚本会自动生成二维码，使用客户端扫描即可导入。

### 方式二：复制链接

复制生成的 `hysteria2://` 链接到客户端导入。

### 方式三：使用配置文件

在 `/etc/hy2config/` 目录下有完整的配置文件：
- Clash 用户使用 `clash.yaml`
- Sing-box 用户使用 `singbox.yaml`
- Xray/V2Ray 用户使用 `xray.yaml`

## 支持的客户端

- **Windows**: v2rayN, Clash for Windows, NekoRay
- **macOS**: ClashX, V2rayU
- **Linux**: v2ray-core, Clash
- **Android**: v2rayNG, NekoBox, Clash for Android
- **iOS**: Shadowrocket, Stash, Surge

## 订阅转换服务

本脚本使用隐私友好的订阅转换服务：

- **API**: https://sublink-worker.watrans.workers.dev
- **开源**: https://github.com/tanaer/sublink-worker
- **特点**: 无日志、无跟踪、部署在 Cloudflare Workers

## 系统要求

- **操作系统**:
  - Ubuntu 18.04+
  - Debian 9+
  - CentOS 8+
  - Rocky Linux 8+
  - Fedora 30+

- **权限**: Root 或 sudo

- **网络**: 需要访问外网下载依赖

## 安全建议

1. **使用强密码**：至少16位，包含大小写字母、数字、特殊字符
2. **定期更新**：及时更新 Hysteria2 到最新版本
3. **防火墙配置**：只开放必要端口
4. **证书管理**：使用正规证书，定期更新
5. **日志监控**：定期检查服务日志，发现异常

## 性能优化

脚本提供可选的 BBR 优化：

```bash
# 在配置菜单中选择 "4. 性能优化"
# 或手动安装 xanmod 内核
```

## 卸载

```bash
sudo python3 hy2.py
# 选择 "2. 卸载 Hysteria2"
```

完全卸载会删除：
- Hysteria2 程序
- 配置文件
- 系统服务
- iptables 规则
- 自签证书

## 常见问题

### 1. 提示权限不足

```bash
# 确保使用 root 或 sudo
sudo python3 hy2.py
```

### 2. 无法连接

- 检查服务是否运行：`systemctl status hysteria-server`
- 检查防火墙规则：`iptables -L -n`
- 检查端口是否被占用：`netstat -tulpn | grep <port>`

### 3. 证书申请失败

- 确保域名已正确解析到服务器IP
- 检查80/443端口是否被占用
- 尝试使用自签名证书

### 4. 配置文件生成失败

- 检查网络连接
- 确认订阅转换服务可访问
- 手动生成配置文件

## 更新日志

### v2.0 (2025-10-30)

- ✨ 整合依赖安装和主脚本
- 🔒 替换为隐私友好的订阅转换服务
- 🎨 优化交互界面和用户体验
- 🐛 修复多个已知问题
- 📝 完善文档和错误提示

### v1.0

- 初始版本

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License

## 免责声明

本工具仅供学习交流使用，请遵守当地法律法规。使用者需对自己的行为负责，作者不承担任何法律责任。

## 相关链接

- [Hysteria2 官网](https://v2.hysteria.network/)
- [Hysteria2 文档](https://v2.hysteria.network/docs/)
- [Sublink Worker](https://github.com/tanaer/sublink-worker)
- [问题反馈](https://github.com/tanaer/port-forward/issues)

---

**⭐ 如果觉得有用，请给个星标支持一下！**
