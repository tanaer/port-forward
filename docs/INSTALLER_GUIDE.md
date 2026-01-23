# 依赖自动安装功能说明

## 🎉 新功能：一键安装依赖

现在系统已经集成了依赖自动安装功能，无需手动安装 Xray 和 Hysteria2！

## 📋 功能特性

### 1. 环境自动检测
- ✅ 自动检测 Xray-core 是否已安装
- ✅ 自动检测 Hysteria2 是否已安装
- ✅ 显示版本信息和安装路径
- ✅ 实时显示环境就绪状态

### 2. 一键安装
- ✅ 从 GitHub 自动下载最新版本
- ✅ 支持 Linux amd64 和 arm64 架构
- ✅ 自动解压和配置执行权限
- ✅ 安装到项目 bin/ 目录

### 3. 可视化界面
- ✅ 清晰的状态展示
- ✅ 环境就绪/未就绪横幅提示
- ✅ 单独安装或一键安装所有依赖
- ✅ 安装进度提示

## 🚀 使用方法

### 方式一：Web 界面安装（推荐）

1. **启动服务**
   ```bash
   ./goForward -port 8889 -pass yourpassword
   ```

2. **访问环境配置页面**
   - 打开浏览器访问: `http://your-server:8889/environment`
   - 或在代理管理页面点击 "🔧 环境配置" 按钮

3. **查看环境状态**
   - 系统会自动检测依赖安装情况
   - 绿色横幅 = 环境已就绪
   - 红色横幅 = 环境未就绪

4. **一键安装**
   - 如果环境未就绪，点击 **"🚀 一键安装所有依赖"** 按钮
   - 或单独安装某个依赖（点击对应的安装按钮）
   - 等待 2-3 分钟完成安装
   - 刷新页面查看安装结果

### 方式二：命令行安装

依然支持传统的命令行安装方式：

```bash
# 使用自动安装脚本
chmod +x install.sh
sudo ./install.sh

# 或手动安装 Xray
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# 或手动安装 Hysteria2
bash <(curl -fsSL https://get.hy2.sh/)
```

## 📊 环境检测逻辑

系统会按以下顺序检测依赖：

1. **检查项目 bin/ 目录**
   - `bin/xray`
   - `bin/hysteria2` 或 `bin/hysteria`

2. **检查系统 PATH**
   - 使用 `which xray` 查找
   - 使用 `which hysteria2` 或 `which hysteria` 查找

3. **获取版本信息**
   - 执行 `xray version` 获取版本
   - 执行 `hysteria2 version` 获取版本

## 🎯 安装流程

### 自动安装 Xray

1. 从 GitHub API 获取最新版本号
2. 根据系统架构确定下载地址
3. 下载 ZIP 压缩包
4. 解压 xray 可执行文件到 bin/ 目录
5. 设置执行权限 (chmod 755)
6. 清理临时文件

### 自动安装 Hysteria2

1. 从 GitHub API 获取最新版本号
2. 根据系统架构确定下载地址
3. 下载二进制文件
4. 保存到 bin/hysteria2
5. 设置执行权限 (chmod 755)

## 📂 文件位置

所有依赖安装到：
```
/path/to/goForward/bin/
├── xray          # Xray-core 可执行文件
└── hysteria2     # Hysteria2 可执行文件
```

## 🔄 状态说明

### ✅ 环境已就绪
- 所有依赖已安装
- 显示版本和路径信息
- 可以正常使用代理功能

### ⚠️ 环境未就绪
- 缺少一个或多个依赖
- 显示缺失的依赖列表
- 提供一键安装按钮

## 🛠️ 故障排查

### 问题 1: 下载失败

**原因**: 网络连接问题或 GitHub 访问受限

**解决方案**:
1. 检查网络连接
2. 使用代理访问 GitHub
3. 手动下载并放到 bin/ 目录：
   ```bash
   mkdir -p bin
   # 下载并放置文件
   wget https://github.com/XTLS/Xray-core/releases/download/xxx/Xray-linux-64.zip
   unzip Xray-linux-64.zip -d bin/
   chmod +x bin/xray
   ```

### 问题 2: 安装后仍显示未安装

**原因**: 需要刷新页面

**解决方案**:
- 点击 "🔄 刷新状态" 按钮
- 或手动刷新浏览器页面

### 问题 3: 安装超时

**原因**: 下载速度慢

**解决方案**:
- 等待更长时间（最多 5 分钟）
- 或使用手动安装方式

### 问题 4: 权限问题

**原因**: 没有写入权限

**解决方案**:
```bash
# 确保 bin 目录有写入权限
chmod 755 bin/
```

## 💡 高级功能

### API 接口

系统提供了 RESTful API 接口：

```bash
# 检查环境状态
curl http://localhost:8889/api/environment/check

# 安装 Xray
curl -X POST http://localhost:8889/api/environment/install-xray

# 安装 Hysteria2
curl -X POST http://localhost:8889/api/environment/install-hy2

# 一键安装所有依赖
curl -X POST http://localhost:8889/api/environment/install-all
```

### 响应格式

```json
{
  "ready": false,
  "dependencies": [
    {
      "name": "Xray-core",
      "installed": false,
      "version": "",
      "path": "",
      "required": true
    },
    {
      "name": "Hysteria2",
      "installed": true,
      "version": "v2.0.0",
      "path": "/usr/local/bin/hysteria2",
      "required": true
    }
  ],
  "message": "环境未就绪，请安装缺失的依赖"
}
```

## 🔐 安全说明

### 下载安全
- 所有文件从官方 GitHub 仓库下载
- 使用 HTTPS 加密传输
- 验证下载状态码

### 文件权限
- 可执行文件设置为 755 权限
- 仅当前用户可写入
- 所有用户可读取和执行

### 网络安全
- 使用 GitHub API 获取版本信息
- 支持设置超时时间（5分钟）
- 自动重试机制

## 📚 相关文档

- [快速开始](QUICKSTART.md) - 5分钟快速部署
- [使用指南](PROXY_GUIDE.md) - 详细配置说明
- [实现总结](IMPLEMENTATION_SUMMARY.md) - 技术架构

## 🎓 注意事项

1. **网络要求**: 需要能访问 GitHub
2. **磁盘空间**: 需要约 50MB 空间
3. **架构支持**: 仅支持 Linux amd64/arm64
4. **安装时间**: 根据网络速度，通常 1-3 分钟
5. **并发安装**: 可以同时安装多个依赖

## 🆕 版本要求

- **Xray-core**: 自动安装最新稳定版
- **Hysteria2**: 自动安装最新稳定版
- **系统**: Linux (Ubuntu/Debian/CentOS/RHEL)
- **架构**: x86_64 (amd64) 或 ARM64

## 🎉 优势

相比手动安装，自动安装具有以下优势：

✅ **简单**: 只需点击一个按钮
✅ **快速**: 自动下载和配置
✅ **准确**: 自动选择正确的版本和架构
✅ **可靠**: 从官方源下载
✅ **可视**: 实时查看安装状态
✅ **灵活**: 支持单独或批量安装

---

**立即体验**: 启动 goForward 并访问 `/environment` 页面！
