package proxy

import (
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

// TestHysteria2Connection 测试Hysteria2连接（通过SOCKS5代理测试出站）
func TestHysteria2Connection(socks5Addr string, socks5Port int) *TestResult {
	result := &TestResult{}

	// 测试通过SOCKS5代理访问外网的速度
	// 使用Google DNS作为测试目标
	testTarget := "8.8.8.8:53"

	// 拼接SOCKS5地址
	proxyAddr := net.JoinHostPort(socks5Addr, fmt.Sprintf("%d", socks5Port))

	// 开始计时
	start := time.Now()

	// 连接到SOCKS5代理
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("连接SOCKS5代理失败: %v", err)
		return result
	}
	defer conn.Close()

	// SOCKS5握手
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// 发送SOCKS5握手请求（无认证）
	_, err = conn.Write([]byte{0x05, 0x01, 0x00})
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("SOCKS5握手失败: %v", err)
		return result
	}

	// 读取握手响应
	buf := make([]byte, 2)
	_, err = conn.Read(buf)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("SOCKS5握手响应失败: %v", err)
		return result
	}

	// 发送连接请求到测试目标
	req := []byte{0x05, 0x01, 0x00, 0x01}
	req = append(req, []byte{8, 8, 8, 8}...) // 8.8.8.8
	req = append(req, []byte{0x00, 0x35}...) // port 53
	_, err = conn.Write(req)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("SOCKS5连接请求失败: %v", err)
		return result
	}

	// 读取连接响应
	resp := make([]byte, 10)
	_, err = conn.Read(resp)

	// 计算延迟
	result.Latency = time.Since(start)
	ms := float64(result.Latency.Microseconds()) / 1000.0
	if ms < 1 {
		result.RTT = fmt.Sprintf("%.2fms", ms)
	} else {
		result.RTT = fmt.Sprintf("%.0fms", ms)
	}

	if err != nil || resp[1] != 0x00 {
		result.Success = false
		result.Message = fmt.Sprintf("出站连接失败: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("出站连接成功 (通过代理到 %s)", testTarget)
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
	ms := float64(result.Latency.Microseconds()) / 1000.0
	if ms < 1 {
		result.RTT = fmt.Sprintf("%.2fms", ms)
	} else {
		result.RTT = fmt.Sprintf("%.0fms", ms)
	}

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
	ms := float64(result.Latency.Microseconds()) / 1000.0
	if ms < 1 {
		result.RTT = fmt.Sprintf("%.2fms", ms)
	} else {
		result.RTT = fmt.Sprintf("%.0fms", ms)
	}

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
