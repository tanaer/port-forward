package proxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// TestResult 测试结果
type TestResult struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Latency time.Duration `json:"latency"`
	RTT     string        `json:"rtt"`
}

// TestHysteria2Connection 测试Hysteria2连接
func TestHysteria2Connection(server string, port string) *TestResult {
	result := &TestResult{}

	// 拼接地址
	addr := fmt.Sprintf("%s:%s", server, port)

	// 开始计时
	start := time.Now()

	// 尝试TLS连接
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp",
		addr,
		&tls.Config{InsecureSkipVerify: true},
	)

	// 计算延迟
	result.Latency = time.Since(start)
	result.RTT = fmt.Sprintf("%dms", result.Latency.Milliseconds())

	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("连接失败: %v", err)
		return result
	}
	defer conn.Close()

	result.Success = true
	result.Message = "连接成功"
	return result
}

// TestVMessConnection 测试VMess连接
func TestVMessConnection(server string, port int) *TestResult {
	result := &TestResult{}

	// 拼接地址（支持IPv6）
	addr := net.JoinHostPort(server, fmt.Sprintf("%d", port))

	// 开始计时
	start := time.Now()

	// 尝试TCP连接
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)

	// 计算延迟
	result.Latency = time.Since(start)
	result.RTT = fmt.Sprintf("%dms", result.Latency.Milliseconds())

	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("连接失败: %v", err)
		return result
	}
	defer conn.Close()

	result.Success = true
	result.Message = "连接成功"
	return result
}

// TestSOCKS5Connection 测试SOCKS5连接
func TestSOCKS5Connection(addr string, port int) *TestResult {
	result := &TestResult{}

	// 拼接地址（支持IPv6）
	address := net.JoinHostPort(addr, fmt.Sprintf("%d", port))

	// 开始计时
	start := time.Now()

	// 尝试TCP连接
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)

	// 计算延迟
	result.Latency = time.Since(start)
	result.RTT = fmt.Sprintf("%dms", result.Latency.Milliseconds())

	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("连接失败: %v", err)
		return result
	}
	defer conn.Close()

	result.Success = true
	result.Message = "连接成功"
	return result
}
