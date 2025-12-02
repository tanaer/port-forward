package proxy

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

const defaultHy2TestTarget = "8.8.8.8:53"

// TestResult 测试结果
type TestResult struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Latency time.Duration `json:"latency"`
	RTT     string        `json:"rtt"`
}

// TestHysteria2Connection 测试Hysteria2连接（通过SOCKS5代理测试出站）
func TestHysteria2Connection(socks5Addr string, socks5Port int) *TestResult {
	return TestHysteria2ConnectionWithTarget(socks5Addr, socks5Port, defaultHy2TestTarget)
}

// TestHysteria2ConnectionWithTarget 测试Hysteria2出站到指定目标的连通性
func TestHysteria2ConnectionWithTarget(socks5Addr string, socks5Port int, target string) *TestResult {
	result := &TestResult{}

	testTarget := target
	if testTarget == "" {
		testTarget = defaultHy2TestTarget
	}

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

	host, portStr, err := net.SplitHostPort(testTarget)
	if err != nil {
		host = "8.8.8.8"
		portStr = "53"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		port = 53
	}

	// 发送连接请求到测试目标
	req := buildSocks5ConnectRequest(host, port)
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

	if err != nil || len(resp) < 2 || resp[1] != 0x00 {
		result.Success = false
		if err != nil {
			result.Message = fmt.Sprintf("出站连接失败: %v", err)
		} else {
			result.Message = "出站连接失败: 目标不可达"
		}
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("出站连接成功 (通过代理到 %s)", testTarget)
	return result
}

func buildSocks5ConnectRequest(host string, port int) []byte {
	req := []byte{0x05, 0x01, 0x00}
	ip := net.ParseIP(host)

	switch {
	case ip != nil && ip.To4() != nil:
		req = append(req, 0x01)
		req = append(req, ip.To4()...)
	case ip != nil && ip.To16() != nil:
		req = append(req, 0x04)
		req = append(req, ip.To16()...)
	default:
		if len(host) > 255 {
			host = host[:255]
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, []byte(host)...)
	}

	req = append(req, byte(port>>8), byte(port&0xff))
	return req
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

// TestSOCKS5Connection 测试SOCKS5连接（仅测试TCP连接）
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

// TestSOCKS5WithTarget 通过SOCKS5代理测试到目标地址的连通性（测试完整链路）
func TestSOCKS5WithTarget(addr string, port int, user, pass, target string) *TestResult {
	result := &TestResult{}

	// 解析目标地址
	targetHost, targetPortStr, err := net.SplitHostPort(target)
	if err != nil {
		// 如果没有端口，默认使用443
		targetHost = target
		targetPortStr = "443"
	}
	targetPort, _ := strconv.Atoi(targetPortStr)
	if targetPort == 0 {
		targetPort = 443
	}

	// 拼接SOCKS5代理地址
	proxyAddr := net.JoinHostPort(addr, fmt.Sprintf("%d", port))

	// 开始计时
	start := time.Now()

	// 连接到SOCKS5代理
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("连接SOCKS5服务器失败: %v", err)
		return result
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// SOCKS5握手 - 根据是否有认证信息选择方法
	if user != "" && pass != "" {
		// 发送支持的认证方法：无认证(0x00) 和 用户名密码(0x02)
		_, err = conn.Write([]byte{0x05, 0x02, 0x00, 0x02})
	} else {
		// 发送支持的认证方法：仅无认证(0x00)
		_, err = conn.Write([]byte{0x05, 0x01, 0x00})
	}
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("SOCKS5握手发送失败: %v", err)
		return result
	}

	// 读取服务器选择的认证方法
	buf := make([]byte, 2)
	_, err = conn.Read(buf)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("SOCKS5握手响应失败: %v", err)
		return result
	}

	if buf[0] != 0x05 {
		result.Success = false
		result.Message = "SOCKS5协议版本错误"
		return result
	}

	selectedMethod := buf[1]
	if selectedMethod == 0xFF {
		result.Success = false
		result.Message = "SOCKS5服务器不支持请求的认证方法"
		return result
	}

	// 如果需要用户名密码认证
	if selectedMethod == 0x02 {
		if user == "" || pass == "" {
			result.Success = false
			result.Message = "SOCKS5服务器要求认证但未提供凭据"
			return result
		}
		// 发送用户名密码
		authReq := make([]byte, 0, 3+len(user)+len(pass))
		authReq = append(authReq, 0x01)           // 版本
		authReq = append(authReq, byte(len(user))) // 用户名长度
		authReq = append(authReq, []byte(user)...) // 用户名
		authReq = append(authReq, byte(len(pass))) // 密码长度
		authReq = append(authReq, []byte(pass)...) // 密码

		_, err = conn.Write(authReq)
		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("SOCKS5认证发送失败: %v", err)
			return result
		}

		// 读取认证响应
		authResp := make([]byte, 2)
		_, err = conn.Read(authResp)
		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("SOCKS5认证响应失败: %v", err)
			return result
		}

		if authResp[1] != 0x00 {
			result.Success = false
			result.Message = "SOCKS5认证失败：用户名或密码错误"
			return result
		}
	}

	// 发送CONNECT请求
	req := make([]byte, 0, 7+len(targetHost))
	req = append(req, 0x05, 0x01, 0x00) // VER, CMD(CONNECT), RSV
	req = append(req, 0x03)              // ATYP: 域名
	req = append(req, byte(len(targetHost)))
	req = append(req, []byte(targetHost)...)
	req = append(req, byte(targetPort>>8), byte(targetPort&0xff))

	_, err = conn.Write(req)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("SOCKS5请求发送失败: %v", err)
		return result
	}

	// 读取响应
	resp := make([]byte, 10)
	n, err := conn.Read(resp)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("SOCKS5响应读取失败: %v", err)
		return result
	}

	if n < 2 {
		result.Success = false
		result.Message = "SOCKS5响应不完整"
		return result
	}

	// 检查响应状态
	if resp[1] != 0x00 {
		errMsg := "未知错误"
		switch resp[1] {
		case 0x01:
			errMsg = "通用SOCKS服务器故障"
		case 0x02:
			errMsg = "连接不被规则集允许"
		case 0x03:
			errMsg = "网络不可达"
		case 0x04:
			errMsg = "主机不可达"
		case 0x05:
			errMsg = "连接被拒绝"
		case 0x06:
			errMsg = "TTL过期"
		case 0x07:
			errMsg = "不支持的命令"
		case 0x08:
			errMsg = "不支持的地址类型"
		}
		result.Success = false
		result.Message = fmt.Sprintf("出站连接失败: %s", errMsg)
		return result
	}

	// 计算延迟
	result.Latency = time.Since(start)
	ms := float64(result.Latency.Microseconds()) / 1000.0
	if ms < 1 {
		result.RTT = fmt.Sprintf("%.2fms", ms)
	} else {
		result.RTT = fmt.Sprintf("%.0fms", ms)
	}

	result.Success = true
	result.Message = fmt.Sprintf("通过SOCKS5代理连接 %s 成功", target)
	return result
}
