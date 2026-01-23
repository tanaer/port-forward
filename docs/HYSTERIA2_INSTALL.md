# Hysteria2 一键安装

> 🚀 安全、快速、易用的 Hysteria2 代理服务器安装工具

## 快速开始

### 一键安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/master/scripts/install.sh | bash
```

### 或者直接运行

```bash
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/master/scripts/hysteria2-install.py -o hy2.py
sudo python3 hy2.py
```

## 特性亮点

- ✅ **安全优化** - 使用隐私友好的订阅转换服务，无配置泄露风险
- ✅ **一键安装** - 自动安装依赖，交互式配置向导
- ✅ **多客户端支持** - 自动生成 Clash、Sing-box、Xray 配置
- ✅ **功能完整** - 支持 ACME 证书、自签证书、端口跳跃、混淆等

## 快捷命令

安装完成后可使用快捷命令：

```bash
sudo hy2
```

## 详细文档

查看完整文档：[scripts/README.md](scripts/README.md)

## 支持的系统

- Ubuntu 18.04+
- Debian 9+
- CentOS 8+
- Rocky Linux 8+
- Fedora 30+

## 安全说明

本工具已优化安全性：
- 使用开源的订阅转换服务（Sublink Worker）
- 无第三方跟踪和日志
- 支持完全离线配置

## 问题反馈

遇到问题？[提交 Issue](https://github.com/tanaer/port-forward/issues)

---

**⚠️ 免责声明**：本工具仅供学习交流使用，请遵守当地法律法规。
