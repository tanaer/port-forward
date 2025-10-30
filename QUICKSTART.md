# 🚀 快速开始指南

## 5分钟部署代理中转系统

### 前提条件

- 一台 Linux 服务器（Ubuntu/Debian/CentOS）
- Root 权限
- 一个 Hysteria2 订阅链接或服务器信息

### 步骤 1: 下载并安装

```bash
# 克隆项目（或下载源码）
cd /root
git clone <your-repo-url> port-forward
cd port-forward

# 运行自动安装脚本
chmod +x install.sh
sudo ./install.sh
```

安装脚本会自动：
- ✅ 安装系统依赖
- ✅ 下载 Xray-core
- ✅ 下载 Hysteria2
- ✅ 安装 Go 环境（如需要）
- ✅ 配置防火墙
- ✅ 创建 systemd 服务

### 步骤 2: 编译项目

```bash
# 更新依赖
go mod tidy

# 编译
go build -o goForward .
```

### 步骤 3: 启动服务

```bash
# 方式1: 直接启动（前台运行）
./goForward -port 8889 -pass yourpassword

# 方式2: 使用 systemd（推荐）
sudo systemctl start goForward
sudo systemctl enable goForward

# 查看状态
sudo systemctl status goForward
```

### 步骤 4: 访问管理界面

打开浏览器访问：
```
http://your-server-ip:8889
```

使用设置的密码登录。

### 步骤 5: 添加代理配置

#### 5.1 进入代理管理

点击顶部导航的 **"代理管理"** 按钮。

#### 5.2 添加新代理

1. 点击 **"添加代理"** 按钮

2. **填写基本信息**
   - 配置名称: `美国节点1`
   - 备注: `测试节点`

3. **配置 VLESS+Reality**
   - 点击 **"一键生成密钥"** 按钮
   - 选择 Reality 回落域名: `microsoft.com:443`
   - 其他字段会自动填充

4. **配置 Hysteria2**

   **方式A: 使用订阅链接（推荐）**
   - 粘贴你的 Hysteria2 订阅链接
   - 点击 **"解析订阅"** 按钮
   - 所有参数自动填充

   **方式B: 手动填写**
   - 服务器地址: `your-hy2-server.com`
   - 端口: `443`
   - 密码: `your-password`
   - 其他参数根据需要填写

5. 点击 **"保存并启动"**

等待几秒，代理会自动启动。

### 步骤 6: 生成客户端订阅

1. 在代理列表中，找到刚创建的代理
2. 点击 **"订阅"** 按钮
3. 复制 **"通用订阅链接"** 或 **"VLESS 链接"**

### 步骤 7: 配置客户端

#### Windows (v2rayN)

1. 下载并安装 [v2rayN](https://github.com/2dust/v2rayN/releases)
2. 打开 v2rayN
3. 点击 **"订阅"** → **"订阅设置"**
4. 点击 **"添加"**，粘贴订阅链接
5. 点击 **"确定"**，然后点击 **"更新订阅"**
6. 在服务器列表中选择节点
7. 右键点击托盘图标，选择 **"系统代理"** → **"自动配置系统代理"**

#### macOS (ClashX)

1. 下载并安装 [ClashX](https://github.com/yichengchen/clashX/releases)
2. 打开 ClashX
3. 点击 **"配置"** → **"托管配置"** → **"管理"**
4. 点击 **"添加"**，粘贴订阅链接
5. 点击 **"更新"**
6. 在菜单栏选择节点

#### Android (v2rayNG)

1. 下载并安装 [v2rayNG](https://github.com/2dust/v2rayNG/releases)
2. 打开 v2rayNG
3. 点击右上角 **"+"** → **"从剪贴板导入"**
4. 或点击 **"扫描二维码"** 扫描订阅页面的二维码
5. 选择节点，点击右下角连接按钮

#### iOS (Shadowrocket)

1. 从 App Store 购买并安装 Shadowrocket
2. 打开 Shadowrocket
3. 点击右上角 **"+"**
4. 类型选择 **"Subscribe"**
5. URL 粘贴订阅链接
6. 点击 **"完成"**
7. 返回首页，下拉更新订阅
8. 选择节点，开启连接

### 步骤 8: 测试连接

1. 打开浏览器
2. 访问 https://www.google.com
3. 如果能正常访问，说明配置成功！

## 🎉 完成！

你已经成功部署了代理中转系统！

## 📊 管理操作

### 查看代理状态

在代理列表中可以看到：
- 运行状态（运行中/已停止）
- 流量统计
- 配置信息

### 启动/停止代理

点击对应代理的 **"启动"** 或 **"停止"** 按钮。

### 编辑代理

1. 点击 **"编辑"** 按钮
2. 修改配置
3. 保存后自动重启（如果正在运行）

### 删除代理

点击 **"删除"** 按钮，确认后删除。

## 🔧 常见问题

### Q: 代理无法启动？

**A: 检查端口占用**
```bash
netstat -tuln | grep 443
```

**A: 查看日志**
```bash
cat proxy_configs/logs/xray.log
cat proxy_configs/logs/hysteria2.log
```

### Q: 客户端无法连接？

**A: 检查防火墙**
```bash
# 开放端口
sudo firewall-cmd --add-port=443/tcp --permanent
sudo firewall-cmd --reload
```

**A: 检查服务状态**
```bash
ps aux | grep xray
ps aux | grep hysteria
```

### Q: Hysteria2 连接失败？

**A: 确认信息正确**
- 服务器地址和端口
- 密码
- 混淆配置（如果有）

**A: 测试 Hysteria2 服务器**
```bash
# 手动测试连接
./bin/hysteria2 client -c proxy_configs/hy2_1.yaml
```

### Q: Reality 握手失败？

**A: 尝试更换回落域名**
- microsoft.com
- yahoo.com
- apple.com

**A: 确认密钥正确**
- 公钥和私钥必须匹配
- Short ID 必须正确

## 🛠️ 高级配置

### 修改监听端口

编辑代理配置，修改 **"监听端口"** 字段。

### 调整带宽限制

在 Hysteria2 配置中调整：
- 上行带宽: 根据实际上传速度
- 下行带宽: 根据实际下载速度

### 启用混淆

如果 Hysteria2 服务器启用了混淆：
1. 混淆类型选择 `salamander`
2. 填写混淆密码

### 配置 IP 白名单

在原有端口转发功能中可以配置 IP 白名单/黑名单。

## 📚 更多文档

- [详细使用指南](PROXY_GUIDE.md) - 完整的配置和故障排查
- [实现总结](IMPLEMENTATION_SUMMARY.md) - 技术架构和实现细节
- [项目说明](PROXY_README.md) - 功能介绍和特性说明

## 🆘 获取帮助

- 查看日志文件
- 阅读详细文档
- 提交 GitHub Issue

## 🎯 下一步

- 配置多个代理节点
- 设置流量监控
- 优化性能参数
- 配置自动备份

---

**祝你使用愉快！** 🎉
