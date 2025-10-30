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
