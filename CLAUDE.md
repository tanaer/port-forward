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
