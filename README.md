# goForward

Go 端口转发与代理管理平台，提供 TCP/UDP 端口隧道、Web 控制台、统计与代理转发能力。

## 功能特性

- TCP/UDP 端口转发与规则热加载
- Web 管理面板与连接统计
- 端口白名单/黑名单与批量转发
- 代理转发新增出站类型：Hysteria2 / SOCKS5
- 监听端口随机生成（10000-65535），自动检测可用性
- 新增“代理转发”“环境配置”入口与出站类型标签展示

## 快速开始

构建：

```bash
go build -o goForward .
```

运行：

```bash
go run . -port 8899 -pass 123456
```

参数说明：

```bash
./goForward -h
```

## Web 控制台

- 端口隧道主页: `http://<host>:8899/`
- 代理转发: `http://<host>:8899/proxy`
- 环境配置: `http://<host>:8899/environment`

## 文档

更多说明与历史记录已整理至 `docs/`：

- `docs/NEW_FEATURES.md`
- `docs/QUICKSTART.md`
- `docs/USAGE_GUIDE.md`
- `docs/PROXY_GUIDE.md`
- `docs/GOFORWARD_YAML_CONFIG.md`

