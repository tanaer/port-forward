# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

goForward is a TCP/UDP port forwarding tool written in Go with a web management interface. It supports hot-reloading of forwarding rules, traffic statistics, idle connection timeouts, IP whitelist/blacklist filtering, and batch port forwarding.

## Architecture

### Core Components

- **main.go**: Entry point that initializes the web server, loads forwarding rules from the database, and spawns concurrent forwarders via `forward.Run`. Each active forward runs in its own goroutine coordinated by `conf.Wg`.

- **conf/**: Holds runtime configuration and shared primitives:
  - `ConnectionStats`: Core struct for forward definitions (ports, addresses, whitelist/blacklist, timeout settings)
  - `Ch`: Global channel for stopping specific forwards (identified by `localPort+protocol`)
  - `Wg`: Global WaitGroup tracking all active forwarder goroutines

- **forward/**: Implements TCP and UDP forwarding logic:
  - `Run()`: Main dispatcher that listens on local ports and spawns handlers per connection
  - `handleTCPConnection()`: Bidirectional TCP proxy with whitelist/blacklist enforcement
  - `handleUDPConnection()`: UDP forwarding with message framing
  - `printStats()`: Periodic (5s) traffic reporting and idle connection cleanup
  - `bufPool`: sync.Pool for 8KB buffers to reduce allocations

- **sql/**: SQLite persistence layer using GORM:
  - Database file: `goForward.db` (stored alongside the binary)
  - `GetAction()`: Returns forwards with `status=0` (active)
  - `AddForward()`: Inserts or reactivates forwards; checks port availability via `netstat`
  - `UpdateForwardConfig()`: Updates forward settings without changing traffic counters
  - Port conflict detection via `checkPortWithNetstat()` and database queries

- **web/**: Gin-based HTTP API and HTML dashboard:
  - Runs on `0.0.0.0:<WebPort>` (default 8889)
  - Session-based authentication when `-pass` is set
  - Routes: `/` (list), `/add`, `/edit/:id`, `/do/:id` (toggle), `/del/:id`, `/import`
  - IP ban system: 3 failed login attempts within 24 hours triggers a ban

- **utils/**: Helper functions for forward lifecycle management (add, update, delete, status toggle)

- **assets/**: Embedded HTML templates and static resources

### Data Flow

1. **Startup**: `main.go` calls `sql.GetAction()` to load active forwards, wraps each in `forward.ConnectionStats`, and launches `forward.Run()` goroutines
2. **Hot Reload**: Web UI changes send stop signals via `conf.Ch`, then call `sql.AddForward()` or `sql.UpdateForwardStatus()` to persist changes
3. **Traffic Accounting**: `forward.printStats()` updates `TotalBytes` in memory every 5s and persists to DB; rolls over to `TotalGigabyte` at 1GB boundaries
4. **Connection Tracking**: TCP connections are stored in `TCPConnections` map with timestamps for idle timeout enforcement

## Build and Development Commands

### Build
```bash
go build -o goForward .
```
Produces the `goForward` binary for deployment.

### Run Locally
```bash
go run . -port 8899 -pass 123456
```
Starts the web dashboard on port 8899 with password authentication.

### Run with Debug Logging
```bash
go run . -debug
```
Prints detailed connection information (source/destination IPs and ports).

### Testing
```bash
go test ./...              # Run all tests
go test ./forward          # Test forwarding logic only
go test ./conf             # Test configuration package only
```

## Key Implementation Details

### Port Forwarding Lifecycle

- **Adding a Forward**: `utils.AddForward()` → `sql.AddForward()` → spawns new `forward.Run()` goroutine
- **Stopping a Forward**: Send `localPort+protocol` to `conf.Ch` → listener closes → goroutine exits
- **Editing a Forward**: Stop existing forward, update DB, restart with new config

### Whitelist/Blacklist Format

Semicolon-separated list supporting individual IPs and CIDR notation:
```
192.168.1.10;10.0.0.0/8;172.16.0.0/12
```
Checked in `forward.ContainsIp()` before establishing TCP connections.

### Idle Connection Timeout

Configured per-forward via `OutTime` (seconds). `printStats()` checks `TCPConnections` map every 5s and closes connections idle longer than `OutTime`.

### Database Schema

- **connection_stats**: id, local_port, remote_addr, remote_port, protocol, whitelist, blacklist, remark, status, out_time, total_bytes, total_gigabyte
- **ip_bans**: id, ip, time_stamp

### Batch Port Forwarding

UI accepts comma-separated local ports (e.g., `80,443,3306`). `sql.GetList()` expands these into individual forward entries with matching remote ports.

## Common Development Patterns

### Adding a New Forward Feature

1. Update `conf.ConnectionStats` struct if new fields are needed
2. Add database migration in `sql/sql.go` init function
3. Implement logic in `forward/forward.go` (e.g., in `handleTCPConnection` or `printStats`)
4. Update web UI forms in `assets/templates/` and handlers in `web/web.go`

### Debugging Connection Issues

- Use `-debug` flag to see all connection events
- Check `forward.printStats()` output for connection counts and traffic
- Verify whitelist/blacklist rules in `forward.ContainsIp()`
- Confirm port availability with `netstat -tuln | grep <port>`

## Code Style

- Use `gofmt -w .` and `goimports` before committing
- Exported functions use PascalCase with doc comments
- Keep logs concise: `[protocol] event` format
- Prefer early returns over nested conditionals
- Use table-driven tests for port scenarios

## Important Notes

- **Minimum Active Forwards**: Web UI prevents deleting or stopping the last active forward to ensure the service remains manageable
- **Database Location**: `goForward.db` is created in the same directory as the binary (determined by `os.Executable()`)
- **Concurrent Safety**: All access to `TCPConnections` map and `TotalBytes` is protected by `TotalBytesLock`
- **Channel Semantics**: `conf.Ch` uses a pass-through pattern where unmatched stop signals are re-sent to avoid blocking

## 版本管理 (Version Management)

### 版本号规则
- **主版本号**: v1.7.x.x - 重大功能更新
- **功能更新**: v1.7.1.0 - 新功能发布
- **BUG修复**: v1.7.0.x - 缺陷修复
- **示例**:
  - v1.7.0 → v1.7.0.1 (BUG修复)
  - v1.7.0.1 → v1.7.1.0 (新功能)
  - v1.7.1.0 → v1.7.1.1 (BUG修复)

### 版本历史记录

#### v1.7.0.1 (2025-11-11) - BUG修复版本
- **修复代理7 SOCKS5认证问题**: 移除配置文件中用户名尾部制表符 (`proxy_configs/xray_7.json:48`)
- **实现输入过滤功能**: 添加 `sanitizeInput()` 函数自动过滤空格和制表符 (`web/web.go:29-35`)
- **修复编译脚本**: 更新 `scripts/devops_check.sh` 使用 `go build -o goForward .` 生成二进制文件
- **UI统一**: 统一所有页面 `.top-nav` 导航栏样式
- **修复页面版本号显示**: 将模板中硬编码版本号替换为动态变量
  - `assets/templates/proxy_list.tmpl:297` - 使用 `{{.version}}` 模板变量
  - `assets/templates/index.tmpl:344` - 使用 `{{.version}}` 模板变量
- **关键文件**:
  - `version/version.go` - 更新版本信息
  - `web/web.go` - 输入过滤逻辑
  - `scripts/devops_check.sh` - 编译命令修复
  - `assets/templates/*.tmpl` - 导航栏样式统一和版本号动态化
  - `proxy_configs/xray_7.json` - SOCKS5配置修复

#### v1.7.0 (2025-11-09) - Phase 3 功能增强版本
- 完整功能增强和API接口增强
- 监控面板完善
- 详细架构和开发模式文档
