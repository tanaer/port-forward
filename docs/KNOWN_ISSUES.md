# 已知安全与性能问题

- `web/web.go:26` 使用硬编码 Cookie 密钥 `secret`，任意人可伪造 session，以管理员身份访问后台。
- `web/web.go:148` 登录口令以明文写入客户端 Session，配合弱密钥会泄露真实密码并被重放。
- `web/web.go:72`、`web/web.go:106`、`assets/templates/index.tmpl:74` 管理操作使用 GET 且缺少 CSRF 防护，登陆用户可被钓鱼触发删除或状态切换。
- `forward/forward.go:242`、`forward/forward.go:265` 将 `[]byte` 指针放回 `sync.Pool`，再次获取即 panic。
- `forward/forward.go:284`、`forward/forward.go:287` 多协程修改 `TotalBytes` 与 `TCPConnections` 未加锁，存在数据竞争。
- `forward/forward.go:303` 在 EOF 时未删除连接记录，长时间运行会堆积陈旧条目占用内存。
- `forward/forward.go:32` 全局变量 `Timestr` 被并发读写，触发数据竞争并污染日志时间戳。
