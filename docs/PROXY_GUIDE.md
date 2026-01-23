# 代理中转系统使用指南

## 系统架构

```
客户端 --[VLESS+Reality]--> 本系统(Xray) --[Hysteria2]--> 目标服务器
```

本系统实现了一个专线中转方案：
- **入站**: VLESS+Reality 协议，提供加密和伪装
- **出站**: Hysteria2 协议，高速连接到上游服务器
- **管理**: Web 界面统一管理，一键生成订阅

## 安装步骤

### 1. 安装依赖

#### 安装 Xray-core

```bash
# 方法1: 使用官方脚本
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# 方法2: 手动下载
wget https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip
unzip Xray-linux-64.zip
mkdir -p bin
mv xray bin/

# 验证安装
xray version
```

#### 安装 Hysteria2

```bash
# 方法1: 使用官方脚本
bash <(curl -fsSL https://get.hy2.sh/)

# 方法2: 手动下载
wget https://github.com/apernet/hysteria/releases/latest/download/hysteria-linux-amd64
chmod +x hysteria-linux-amd64
mkdir -p bin
mv hysteria-linux-amd64 bin/hysteria2

# 验证安装
hysteria2 version
```

### 2. 编译项目

```bash
# 更新依赖
go mod tidy

# 编译
go build -o goForward .

# 或使用现有的构建命令
go build -o goForward .
```

### 3. 启动服务

```bash
# 启动 (默认端口 8889)
./goForward

# 指定端口和密码
./goForward -port 8899 -pass yourpassword

# 后台运行
nohup ./goForward -port 8899 -pass yourpassword > goForward.log 2>&1 &
```

### 4. 配置开机自启

```bash
# 创建 systemd 服务
sudo nano /etc/systemd/system/goForward.service
```

内容如下：

```ini
[Unit]
Description=goForward Proxy Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/port-forward
ExecStart=/root/port-forward/goForward -port 8889 -pass yourpassword
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable goForward
sudo systemctl start goForward
sudo systemctl status goForward
```

## 使用指南

### 1. 访问 Web 管理界面

打开浏览器访问: `http://your-server-ip:8889`

如果设置了密码，需要先登录。

### 2. 添加代理配置

#### 方式一: 快速配置（推荐）

1. 点击 **"代理管理"** 进入代理列表
2. 点击 **"添加代理"**
3. 填写基本信息：
   - 配置名称: 例如 "美国节点1"
   - 备注: 可选

4. **VLESS+Reality 配置**:
   - 点击 **"一键生成密钥"** 按钮，自动生成 UUID、密钥对和 ShortID
   - 选择 Reality 回落域名（推荐: microsoft.com, yahoo.com, apple.com）
   - SNI 会自动填充

5. **Hysteria2 配置**:
   - 粘贴你的 Hysteria2 订阅链接
   - 点击 **"解析订阅"** 按钮，自动填充所有参数
   - 或手动填写服务器地址、端口、密码等

6. 点击 **"保存并启动"**

#### 方式二: 手动配置

**VLESS+Reality 入站**:
- 监听端口: 443 (或其他端口)
- UUID: 使用 UUID 生成器或点击生成按钮
- Reality 回落域名: 选择未被屏蔽的域名
- 私钥/公钥: 点击生成按钮自动生成
- Short ID: 点击生成按钮自动生成

**Hysteria2 出站**:
- 服务器地址: 你的 Hysteria2 服务器
- 端口: 通常是 443
- 密码: Hysteria2 服务器密码
- 混淆: 可选，如果服务器启用了混淆
- SNI: 可选，服务器域名
- 带宽: 根据实际情况调整

### 3. 生成客户端订阅

1. 在代理列表中，点击对应代理的 **"订阅"** 按钮
2. 页面会显示：
   - 通用订阅链接（适用于大多数客户端）
   - VLESS 直连链接
   - 二维码（手机扫描）

3. 复制订阅链接或 VLESS 链接到客户端

### 4. 客户端配置

#### Windows (v2rayN)

1. 打开 v2rayN
2. 点击 **"订阅"** -> **"订阅设置"**
3. 添加订阅地址，粘贴订阅链接
4. 点击 **"更新订阅"**
5. 选择节点，启动代理

#### macOS (ClashX)

1. 打开 ClashX
2. 点击 **"配置"** -> **"托管配置"** -> **"管理"**
3. 添加订阅链接
4. 更新配置
5. 选择节点

#### Android (v2rayNG)

1. 打开 v2rayNG
2. 点击右上角 **"+"** -> **"从剪贴板导入"**
3. 或扫描二维码
4. 选择节点，启动

#### iOS (Shadowrocket)

1. 打开 Shadowrocket
2. 点击右上角 **"+"**
3. 类型选择 **"Subscribe"**
4. 粘贴订阅链接
5. 更新订阅，选择节点

## 管理操作

### 启动/停止代理

在代理列表中，点击对应代理的 **"启动"** 或 **"停止"** 按钮。

### 编辑代理

1. 点击 **"编辑"** 按钮
2. 修改配置
3. 保存后会自动重启代理（如果正在运行）

### 删除代理

点击 **"删除"** 按钮，确认后删除。注意：此操作不可恢复。

### 查看流量统计

代理列表中会显示每个代理的流量使用情况。

## 故障排查

### 1. 代理无法启动

**检查端口占用**:
```bash
netstat -tuln | grep 443
```

**查看日志**:
```bash
# 查看 Xray 日志
cat proxy_configs/logs/xray.log

# 查看 Hysteria2 日志
cat proxy_configs/logs/hysteria2.log

# 查看主程序日志
tail -f goForward.log
```

### 2. 客户端无法连接

**检查防火墙**:
```bash
# 开放端口
firewall-cmd --add-port=443/tcp --permanent
firewall-cmd --reload

# 或使用 iptables
iptables -A INPUT -p tcp --dport 443 -j ACCEPT
```

**检查服务状态**:
```bash
# 检查进程
ps aux | grep xray
ps aux | grep hysteria

# 检查端口监听
ss -tuln | grep 443
```

### 3. Hysteria2 连接失败

- 确认 Hysteria2 服务器地址和端口正确
- 确认密码正确
- 检查服务器是否在线
- 尝试手动测试 Hysteria2 连接

### 4. Reality 握手失败

- 确认回落域名可访问
- 确认公钥/私钥匹配
- 确认 Short ID 正确
- 尝试更换回落域名

## 性能优化

### 1. 调整带宽限制

在 Hysteria2 配置中，根据实际网络情况调整上行和下行带宽：

```
上行带宽: 100 Mbps (根据实际上传速度)
下行带宽: 100 Mbps (根据实际下载速度)
```

### 2. 系统优化

```bash
# 增加文件描述符限制
ulimit -n 65535

# 优化 TCP 参数
sysctl -w net.core.rmem_max=16777216
sysctl -w net.core.wmem_max=16777216
sysctl -w net.ipv4.tcp_rmem='4096 87380 16777216'
sysctl -w net.ipv4.tcp_wmem='4096 65536 16777216'
```

### 3. 启用 BBR

```bash
# 检查是否支持 BBR
lsmod | grep bbr

# 启用 BBR
echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
sysctl -p
```

## 安全建议

1. **修改默认端口**: 不要使用 8889，改用其他端口
2. **设置强密码**: 使用复杂密码保护 Web 界面
3. **启用防火墙**: 只开放必要的端口
4. **定期更新**: 及时更新 Xray 和 Hysteria2
5. **监控日志**: 定期检查异常访问
6. **限制访问**: 使用 IP 白名单限制管理界面访问

## 常见问题

**Q: 可以同时运行多个代理吗？**
A: 可以，只要端口不冲突即可。

**Q: 支持 IPv6 吗？**
A: 支持，确保服务器和 Hysteria2 上游都支持 IPv6。

**Q: 流量统计准确吗？**
A: 统计的是通过本系统的流量，不包括 Hysteria2 到目标服务器的流量。

**Q: 可以用于生产环境吗？**
A: 可以，但建议先在测试环境充分测试。

**Q: 如何备份配置？**
A: 备份 `goForward.db` 数据库文件和 `proxy_configs/` 目录。

## 技术支持

- GitHub Issues: https://github.com/your-repo/issues
- 文档: 查看 CLAUDE.md 和 AGENTS.md
- 日志: 查看 `proxy_configs/logs/` 目录

## 更新日志

### v1.0.0 (2025-01-XX)
- 初始版本
- 支持 VLESS+Reality 入站
- 支持 Hysteria2 出站
- Web 管理界面
- 订阅系统
- 流量统计
