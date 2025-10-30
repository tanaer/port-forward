# 🚀 goForward 代理中转系统

## 新功能：VLESS+Reality + Hysteria2 专线中转

在原有端口转发功能基础上，新增了强大的代理中转功能：

```
客户端 --[VLESS+Reality]--> 本系统 --[Hysteria2]--> 国际互联网
```

### ✨ 核心特性

- **🔐 VLESS+Reality 入站**: 提供强大的加密和流量伪装
- **🚀 Hysteria2 出站**: 高速、稳定的上游连接
- **🎯 一键配置**: Web 界面自动生成所有密钥和配置
- **📱 订阅系统**: 自动生成客户端订阅链接和二维码
- **📊 流量统计**: 实时监控每个代理的流量使用
- **🔄 热重载**: 无需重启即可更新配置

### 🎨 功能亮点

#### 1. 智能配置生成
- 一键生成 UUID、Reality 密钥对、ShortID
- 自动解析 Hysteria2 订阅链接
- 预设多个未被屏蔽的 Reality 回落域名

#### 2. 多协议支持
- **入站**: VLESS + Reality (xtls-rprx-vision)
- **出站**: Hysteria2 (支持混淆、自定义带宽)
- **传输**: TCP + TLS

#### 3. 便捷管理
- Web 界面统一管理所有代理
- 支持启动/停止/编辑/删除操作
- 实时查看运行状态

#### 4. 客户端友好
- 生成通用订阅链接（V2Ray、Clash 等）
- 生成 VLESS 直连链接
- 提供二维码扫描（移动端）

## 📦 快速开始

### 1. 安装依赖

```bash
# 使用自动安装脚本（推荐）
chmod +x install.sh
sudo ./install.sh
```

或手动安装：

```bash
# 安装 Xray-core
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# 安装 Hysteria2
bash <(curl -fsSL https://get.hy2.sh/)
```

### 2. 编译项目

```bash
# 更新依赖
go mod tidy

# 编译
go build -o goForward .
```

### 3. 启动服务

```bash
# 启动（默认端口 8889）
./goForward

# 指定端口和密码
./goForward -port 8899 -pass yourpassword
```

### 4. 访问管理界面

打开浏览器访问: `http://your-server-ip:8889`

## 🎯 使用流程

### 添加代理配置

1. **进入代理管理**
   - 点击顶部导航 "代理管理"

2. **添加新代理**
   - 点击 "添加代理" 按钮
   - 填写配置名称和备注

3. **配置 VLESS+Reality**
   - 点击 "一键生成密钥" 自动生成所有密钥
   - 选择 Reality 回落域名（推荐 microsoft.com）
   - SNI 会自动填充

4. **配置 Hysteria2**
   - 粘贴 Hysteria2 订阅链接
   - 点击 "解析订阅" 自动填充参数
   - 或手动填写服务器信息

5. **保存并启动**
   - 点击 "保存并启动"
   - 系统自动生成配置并启动代理

### 生成客户端订阅

1. 在代理列表中点击 "订阅" 按钮
2. 复制订阅链接或 VLESS 链接
3. 导入到客户端（v2rayN、ClashX、Shadowrocket 等）

## 📱 支持的客户端

- **Windows**: v2rayN, Clash for Windows
- **macOS**: V2RayX, ClashX, Surge
- **Linux**: v2ray, Clash
- **Android**: v2rayNG, Clash for Android
- **iOS**: Shadowrocket, Quantumult X, Surge

## 🏗️ 项目结构

```
port-forward/
├── proxy/                    # 代理模块
│   ├── xray/                # Xray 配置和管理
│   │   ├── config.go        # 配置生成
│   │   ├── manager.go       # 进程管理
│   │   └── reality.go       # Reality 密钥生成
│   ├── hysteria/            # Hysteria2 模块
│   │   ├── config.go        # 配置生成
│   │   ├── client.go        # 客户端管理
│   │   └── parser.go        # 订阅解析
│   ├── bridge.go            # 桥接管理
│   ├── subscription.go      # 订阅生成
│   └── utils.go             # 工具函数
├── conf/
│   └── proxy.go             # 代理配置结构
├── sql/
│   └── proxy.go             # 数据库操作
├── web/
│   └── proxy.go             # Web 路由
├── assets/templates/        # 页面模板
│   ├── proxy_list.tmpl      # 代理列表
│   ├── proxy_add.tmpl       # 添加代理
│   ├── proxy_edit.tmpl      # 编辑代理
│   └── proxy_subscription.tmpl # 订阅页面
├── install.sh               # 安装脚本
├── PROXY_GUIDE.md          # 详细使用指南
└── PROXY_README.md         # 本文件
```

## 🔧 配置说明

### Reality 回落域名

系统预设了多个未被 GFW 屏蔽的域名：
- microsoft.com (推荐)
- yahoo.com
- apple.com
- cloudflare.com
- aws.amazon.com
- tesla.com
- cisco.com
- oracle.com
- ibm.com
- samsung.com

### Hysteria2 配置

支持的配置项：
- 服务器地址和端口
- 认证密码
- 混淆类型（salamander）
- TLS SNI
- 上下行带宽限制
- SOCKS5 本地端口

### 端口说明

- **8889**: Web 管理界面（可自定义）
- **443**: VLESS+Reality 入站（可自定义）
- **10808**: Hysteria2 SOCKS5 本地端口（内部使用）

## 📊 数据流向

```
1. 客户端发起连接
   ↓
2. VLESS+Reality 入站（Xray）
   - 解密 VLESS 流量
   - Reality 伪装成正常 TLS
   ↓
3. 本地 SOCKS5 桥接
   - Xray 出站到 127.0.0.1:10808
   ↓
4. Hysteria2 客户端
   - 接收 SOCKS5 请求
   - 通过 Hysteria2 协议转发
   ↓
5. Hysteria2 服务器
   ↓
6. 目标网站
```

## 🛡️ 安全建议

1. **修改默认端口**: 不要使用默认的 8889 端口
2. **设置强密码**: 使用复杂密码保护管理界面
3. **启用防火墙**: 只开放必要的端口
4. **定期更新**: 及时更新 Xray 和 Hysteria2
5. **监控日志**: 定期检查异常访问

## 🐛 故障排查

### 代理无法启动

```bash
# 查看 Xray 日志
cat proxy_configs/logs/xray.log

# 查看 Hysteria2 日志
cat proxy_configs/logs/hysteria2.log

# 检查端口占用
netstat -tuln | grep 443
```

### 客户端无法连接

```bash
# 检查防火墙
firewall-cmd --list-ports

# 检查进程
ps aux | grep xray
ps aux | grep hysteria

# 检查端口监听
ss -tuln | grep 443
```

详细故障排查请查看 [PROXY_GUIDE.md](PROXY_GUIDE.md)

## 📚 文档

- [详细使用指南](PROXY_GUIDE.md) - 完整的安装和使用说明
- [开发文档](CLAUDE.md) - 代码架构和开发指南
- [贡献指南](AGENTS.md) - 项目规范和提交指南

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

本项目基于原 goForward 项目扩展开发。

## ⚠️ 免责声明

本工具仅供学习和研究使用，请遵守当地法律法规。使用本工具产生的任何后果由使用者自行承担。

## 🎉 致谢

- [Xray-core](https://github.com/XTLS/Xray-core) - 强大的代理工具
- [Hysteria2](https://github.com/apernet/hysteria) - 高性能代理协议
- [goForward](https://github.com/csznet/goForward) - 原始端口转发项目
