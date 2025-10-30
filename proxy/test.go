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

// TestHysteria2Connection 测试Hysteria2连接（UDP协议）
func TestHysteria2Connection(server string, port string) *TestResult {
	result := &TestResult{}

	// 拼接地址（支持IPv6）
	addr := net.JoinHostPort(server, port)

	// 开始计时
	start := time.Now()

	// Hysteria2使用UDP协议，尝试UDP连接测试
	conn, err := net.DialTimeout("udp", addr, 5*time.Second)

	// 计算延迟
	result.Latency = time.Since(start)
	result.RTT = fmt.Sprintf("%dms", result.Latency.Milliseconds())

	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("UDP连接失败: %v", err)
		return result
	}
	defer conn.Close()

	// 设置读写超时
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	// 尝试发送测试数据包（简单的可达性测试）
	testData := []byte{0x00, 0x01, 0x02, 0x03}
	_, err = conn.Write(testData)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("发送测试包失败: %v", err)
		return result
	}

	// 尝试读取响应（可能会超时，这是正常的）
	buffer := make([]byte, 1024)
	_, readErr := conn.Read(buffer)

	// 只要UDP连接建立成功，就认为服务器可达
	// 读取超时不算失败，因为服务器可能不响应未认证的包
	result.Success = true
	if readErr != nil {
		result.Message = "UDP端口可达（未收到响应，可能需要认证）"
	} else {
		result.Message = "UDP连接成功"
	}

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
