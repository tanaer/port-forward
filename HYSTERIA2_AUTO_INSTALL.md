# Hysteria2 全自动化安装指南

## 🚀 一键安装

完全自动化安装，无需任何手动配置，直接输出订阅信息！

```bash
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/refs/heads/master/scripts/install.sh | bash
```

## ✨ 功能特性

### 完全自动化
- ✅ 自动检测系统并安装依赖
- ✅ 自动生成自签名证书
- ✅ 自动获取服务器IP地址（支持IPv4/IPv6）
- ✅ 自动安装Hysteria2最新版本
- ✅ 自动生成配置文件
- ✅ 自动启动服务并设置开机自启
- ✅ 自动输出订阅链接和二维码

### 安全隐私
- 🔒 使用隐私友好的订阅转换服务 (sublink-worker)
- 🔒 自动生成高强度随机密码
- 🔒 正确的证书权限设置
- 🔒 本地生成配置，不泄露隐私

### 多客户端支持
- 📱 自动生成 Clash 配置
- 📱 自动生成 Sing-box 配置
- 📱 自动生成 Xray 配置
- 📱 支持 v2rayN、NekoBox、v2rayNG、NekoRay

## 📋 默认配置

安装脚本使用以下默认配置：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| 监听端口 | 443 | HTTPS 标准端口，不容易被封 |
| 证书类型 | 自签名 | 使用 bing.com 作为域名 |
| 用户名 | user | 订阅链接标识 |
| 密码 | 随机生成 | 16位字母+数字组合 |
| 伪装网站 | https://www.bing.com | 流量伪装目标 |
| Brutal模式 | 关闭 | 适合大多数场景 |
| 协议嗅探 | 开启 | 提高兼容性 |

## 🎯 使用流程

### 1. 执行安装命令

```bash
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/refs/heads/master/scripts/install.sh | bash
```

### 2. 等待安装完成

安装脚本会自动执行以下步骤：

```
✓ 检测系统并安装依赖
✓ 生成自签名证书
✓ 获取服务器IP地址
✓ 安装 Hysteria2
✓ 生成配置文件
✓ 启动服务
✓ 输出订阅信息
```

### 3. 获取订阅信息

安装完成后会自动显示：

- 🔗 Hysteria2 订阅链接
- 📱 客户端二维码
- 📁 配置文件位置

示例输出：

```
============================================================
安装完成！订阅信息如下：
============================================================

【Hysteria2 链接】
hysteria2://abc123def456@1.2.3.4:443?sni=bing.com&insecure=1#user

支持客户端: v2rayN, NekoBox, v2rayNG, NekoRay

【客户端二维码】
[二维码显示]

✓ Clash 配置已生成: /etc/hy2config/clash.yaml
✓ Sing-box 配置已生成: /etc/hy2config/singbox.yaml
✓ Xray 配置已生成: /etc/hy2config/xray.yaml

配置文件已保存到: /etc/hy2config/hy2_url_scheme.txt

============================================================
安装成功完成！服务已自动启动。
============================================================
```

## 📱 客户端配置

### 方式一：扫码导入（推荐）

使用支持的客户端扫描终端显示的二维码，自动导入配置。

### 方式二：复制链接

复制 `hysteria2://` 开头的订阅链接到客户端导入。

### 方式三：使用配置文件

根据客户端类型，使用对应的配置文件：

- **Clash**: `/etc/hy2config/clash.yaml`
- **Sing-box**: `/etc/hy2config/singbox.yaml`
- **Xray**: `/etc/hy2config/xray.yaml`

## 🔧 管理命令

安装完成后，可以使用以下命令管理服务：

### 服务管理

```bash
# 查看服务状态
systemctl status hysteria-server

# 启动服务
systemctl start hysteria-server

# 停止服务
systemctl stop hysteria-server

# 重启服务
systemctl restart hysteria-server

# 查看日志
journalctl -u hysteria-server -f
```

### 快捷命令

安装脚本会创建两个快捷命令：

```bash
# 交互式管理界面
hy2

# 重新运行自动安装
hy2-auto
```

### 查看配置信息

```bash
# 查看订阅链接
cat /etc/hy2config/hy2_url_scheme.txt

# 查看配置文件
cat /etc/hysteria/config.yaml

# 查看客户端配置
ls /etc/hy2config/
```

## 🌐 Web界面快速入口

在 goForward 的代理管理页面，点击"🚀 快速安装Hysteria2"按钮，可以：

1. 查看自动化安装说明
2. 一键复制安装命令
3. 了解功能特性

## 🔍 故障排查

### 服务启动失败

```bash
# 查看详细日志
journalctl -u hysteria-server -n 50

# 检查配置文件
cat /etc/hysteria/config.yaml

# 检查证书权限
ls -la /etc/ssl/private/bing.com.*
```

### 端口被占用

```bash
# 查看443端口占用情况
netstat -tuln | grep 443

# 停止占用端口的服务
systemctl stop <service-name>
```

### 连接测试失败

```bash
# 测试本地连接
curl -I http://localhost:443

# 检查防火墙
ufw status
iptables -L -n
```

## 📖 高级配置

如果需要自定义配置，可以使用交互式安装：

```bash
# 下载脚本
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/refs/heads/master/scripts/hysteria2-install.py -o /tmp/hy2.py

# 交互式运行
python3 /tmp/hy2.py
```

交互式模式支持：

- 自定义端口
- 域名证书（ACME自动申请）
- 手动指定证书路径
- 开启Brutal模式
- 开启混淆
- 配置端口跳跃

## 🔄 与 goForward 集成

安装完成后，在 goForward 中配置代理：

1. 访问代理管理页面
2. 点击"添加代理"
3. 选择 Hysteria2 出站类型
4. 填入服务器地址和密码
5. 保存并启动

goForward 会创建 VLESS+Reality 入站，将流量转发到 Hysteria2 出站。

## 📝 技术细节

### 证书生成

使用 OpenSSL 生成 ECC 证书：

```bash
openssl ecparam -name prime256v1 -out ec_param.pem
openssl req -x509 -nodes -newkey ec:ec_param.pem \
  -keyout bing.com.key \
  -out bing.com.crt \
  -subj "/CN=bing.com" \
  -days 36500
```

### IP地址检测

- IPv4: 使用 ip-api.com API
- IPv6: 使用 api.ip.sb API
- 自动检测 Cloudflare WARP

### 订阅转换

使用 sublink-worker API：

```
https://sublink-worker.watrans.workers.dev/clash?config={hysteria2_url}
https://sublink-worker.watrans.workers.dev/singbox?config={hysteria2_url}
https://sublink-worker.watrans.workers.dev/xray?config={hysteria2_url}
```

## 🆘 获取帮助

- 查看安装文档: `/HYSTERIA2_INSTALL.md`
- 查看使用指南: `/scripts/README.md`
- 提交问题: https://github.com/tanaer/port-forward/issues

## 📊 版本信息

- **脚本版本**: 2.0
- **支持系统**: Ubuntu, Debian, CentOS, Rocky Linux, Fedora
- **Python版本**: 3.6+
- **Hysteria2版本**: 自动安装最新版

---

**享受快速、安全的代理服务！** 🎉
