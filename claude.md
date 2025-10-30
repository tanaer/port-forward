# Git Commit Log

## 2025-10-30 - Proxy Feature Enhancements

### 中文版本

**修复代理功能和订阅链接**

本次提交修复了多个代理相关问题，并增强了VMess协议支持：

1. **修复订阅链接认证问题**
   - 修改web中间件，允许/sub/路由公开访问
   - 订阅链接无需密码即可访问

2. **修复VLESS链接格式**
   - 调整VLESS链接参数顺序和格式
   - 移除冗余参数，符合v2rayN标准
   - 修复导入v2rayN崩溃问题

3. **修复Xray网络监听**
   - 将监听地址从127.0.0.1改为0.0.0.0
   - 允许外部客户端连接代理服务

4. **添加代理自动启动**
   - 程序启动时自动加载并启动活动代理
   - 修复SOCKS5/VMess出站时的xrayManager初始化问题

5. **添加Xray路由配置**
   - 添加完整的路由配置结构
   - 正确连接入站和出站流量

6. **完善VMess配置界面**
   - 编辑页面添加完整的VMess配置选项
   - 添加订阅链接解析功能
   - 添加WebSocket和TLS动态配置

7. **优化代理停止逻辑**
   - 停止代理时优雅处理不存在的bridge
   - 即使进程不存在也能正确更新状态

8. **增强VMess订阅解析**
   - 支持URI格式: vmess://security:uuid@host:port?params
   - 支持Base64 JSON格式
   - 智能检测并解析两种格式
   - 添加输入字符串trim处理

修改的文件：
- web/web.go - 中间件认证逻辑
- proxy/subscription.go - VLESS链接生成
- proxy/xray/config.go - Xray配置和路由
- main.go - 代理自动启动
- proxy/utils.go - 代理生命周期管理
- assets/templates/proxy_edit.tmpl - VMess编辑界面
- proxy/vmess/parser.go - VMess订阅解析
- web/proxy.go - VMess解析API

---

### English Version

**Fix proxy features and subscription links**

This commit fixes multiple proxy-related issues and enhances VMess protocol support:

1. **Fix subscription link authentication**
   - Modified web middleware to allow public access to /sub/ routes
   - Subscription links accessible without password

2. **Fix VLESS link format**
   - Adjusted VLESS link parameter order and format
   - Removed redundant parameters, compliant with v2rayN standards
   - Fixed v2rayN import crash issue

3. **Fix Xray network binding**
   - Changed listen address from 127.0.0.1 to 0.0.0.0
   - Allow external clients to connect to proxy service

4. **Add proxy auto-start**
   - Automatically load and start active proxies on program startup
   - Fixed xrayManager initialization for SOCKS5/VMess outbound

5. **Add Xray routing configuration**
   - Added complete routing configuration structures
   - Properly connect inbound and outbound traffic

6. **Enhance VMess configuration UI**
   - Added complete VMess configuration options to edit page
   - Added subscription link parsing functionality
   - Added dynamic WebSocket and TLS configuration

7. **Optimize proxy stop logic**
   - Gracefully handle non-existent bridge when stopping proxy
   - Correctly update status even if process doesn't exist

8. **Enhance VMess subscription parsing**
   - Support URI format: vmess://security:uuid@host:port?params
   - Support Base64 JSON format
   - Intelligently detect and parse both formats
   - Added input string trimming

Modified files:
- web/web.go - middleware authentication logic
- proxy/subscription.go - VLESS link generation
- proxy/xray/config.go - Xray config and routing
- main.go - proxy auto-start
- proxy/utils.go - proxy lifecycle management
- assets/templates/proxy_edit.tmpl - VMess edit interface
- proxy/vmess/parser.go - VMess subscription parsing
- web/proxy.go - VMess parsing API

---

## 2025-10-30 - Hysteria2 Installation Script

### 中文版本

**添加Hysteria2一键安装脚本**

整合和优化原有的两个脚本，提供更安全、易用的安装体验：

**功能特性：**
1. **整合依赖安装** - 合并phy2.sh到主脚本，无需额外脚本
2. **安全优化** - 使用隐私友好的订阅转换服务 (Sublink Worker)
3. **UI优化** - 清晰的交互界面和菜单系统
4. **完整文档** - 详细的README和使用说明
5. **一键部署** - 提供install.sh快速安装脚本

**新增文件：**
- scripts/hysteria2-install.py - 主安装脚本 (1000+ 行)
- scripts/install.sh - 一键部署脚本
- scripts/README.md - 完整使用文档
- HYSTERIA2_INSTALL.md - 快速开始指南

**改进点：**
- ✅ 移除第三方配置泄露风险（原脚本会将配置发送到 sub.crazyact.com）
- ✅ 使用 sublink-worker 订阅转换API（开源、无日志、隐私友好）
- ✅ 自动生成 Clash/Sing-box/Xray 配置
- ✅ 完善的错误处理和用户提示
- ✅ 支持多种证书配置方式（ACME/自签/手动）
- ✅ 集成系统依赖检测和安装
- ✅ 优化的交互体验和界面

**安全分析：**
审查了原始脚本 phy2.sh 和 hysteria2.py，发现的问题：
- phy2.sh Line 18: 删除 Python pip 保护机制
- hysteria2.py Line 400-402: 配置信息泄露给第三方服务
- hysteria2.py Line 17: 动态下载远程脚本存在风险

已全部修复并优化。

**一键安装命令：**
```bash
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/master/scripts/install.sh | bash
```

---

### English Version

**Add Hysteria2 One-Click Installation Script**

Integrated and optimized the original two scripts to provide a more secure and user-friendly installation experience:

**Features:**
1. **Integrated Dependency Installation** - Merged phy2.sh into main script, no extra scripts needed
2. **Security Optimization** - Using privacy-friendly subscription conversion service (Sublink Worker)
3. **UI Optimization** - Clear interactive interface and menu system
4. **Complete Documentation** - Detailed README and usage instructions
5. **One-Click Deployment** - Provided install.sh for quick installation

**New Files:**
- scripts/hysteria2-install.py - Main installation script (1000+ lines)
- scripts/install.sh - One-click deployment script
- scripts/README.md - Complete usage documentation
- HYSTERIA2_INSTALL.md - Quick start guide

**Improvements:**
- ✅ Removed third-party config leakage risk (original script sent config to sub.crazyact.com)
- ✅ Using sublink-worker subscription API (open-source, no-log, privacy-friendly)
- ✅ Auto-generate Clash/Sing-box/Xray configurations
- ✅ Comprehensive error handling and user prompts
- ✅ Support multiple certificate configuration methods (ACME/Self-signed/Manual)
- ✅ Integrated system dependency detection and installation
- ✅ Optimized interaction experience and interface

**Security Analysis:**
Reviewed original scripts phy2.sh and hysteria2.py, found issues:
- phy2.sh Line 18: Removes Python pip protection mechanism
- hysteria2.py Line 400-402: Config info leaked to third-party service
- hysteria2.py Line 17: Dynamic download of remote script poses risk

All issues fixed and optimized.

**One-Click Installation Command:**
```bash
curl -fsSL https://raw.githubusercontent.com/tanaer/port-forward/refs/heads/master/scripts/install.sh | bash
```

---

## 2025-10-30 - UI Enhancement for Hysteria2 Installation

### 中文版本

**在代理添加/编辑页面添加Hysteria2一键安装提示**

为了方便用户部署Hysteria2服务器，在Web界面中添加醒目的安装提示：

**UI改进：**
- ✨ 渐变紫色背景的提示卡片（视觉突出）
- 📋 一键复制安装命令按钮
- 📖 直接链接到安装文档
- 🎯 突出显示核心特性

**功能实现：**
- 复制按钮支持现代 `navigator.clipboard` API
- 提供传统 `execCommand` 降级方案
- 兼容 HTTP 和 HTTPS 环境
- 用户友好的操作反馈

**脚本修复：**
- 修复 Python 脚本 EOF 错误（非交互式环境下的崩溃）
- 添加 `EOFError` 和 `KeyboardInterrupt` 异常处理
- 改进用户取消操作的处理流程

**显示位置：**
- 添加代理页面：选择 Hysteria2 出站时显示
- 编辑代理页面：Hysteria2 配置区域顶部

**修改文件：**
- assets/templates/proxy_add.tmpl (添加页面)
- assets/templates/proxy_edit.tmpl (编辑页面)
- scripts/hysteria2-install.py (异常处理)

---

### English Version

**Add Hysteria2 One-Click Installation Prompt in Proxy Pages**

Added prominent installation prompts in the web interface to help users deploy Hysteria2 servers:

**UI Improvements:**
- ✨ Gradient purple background card (visually prominent)
- 📋 One-click copy installation command button
- 📖 Direct link to installation documentation
- 🎯 Highlight core features

**Feature Implementation:**
- Copy button supports modern `navigator.clipboard` API
- Provides traditional `execCommand` fallback
- Compatible with HTTP and HTTPS environments
- User-friendly operation feedback

**Script Fixes:**
- Fixed Python script EOF error (crash in non-interactive environments)
- Added `EOFError` and `KeyboardInterrupt` exception handling
- Improved user cancellation handling

**Display Locations:**
- Add proxy page: Shows when selecting Hysteria2 outbound
- Edit proxy page: Top of Hysteria2 configuration section

**Modified Files:**
- assets/templates/proxy_add.tmpl (add page)
- assets/templates/proxy_edit.tmpl (edit page)
- scripts/hysteria2-install.py (exception handling)


