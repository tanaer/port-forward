package proxy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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

// SpeedTestResult 出站测速结果
type SpeedTestResult struct {
	Latency       time.Duration `json:"latency"`
	DownloadSpeed float64       `json:"downloadSpeed"`
	UploadSpeed   float64       `json:"uploadSpeed"`
	Success       bool          `json:"success"`
	Message       string        `json:"message"`
}

// TestOutboundSpeed 通过SOCKS5代理测试出站延迟、上传和下载速度
func TestOutboundSpeed(socks5Addr, socks5Port, user, pass string) *SpeedTestResult {
	result := &SpeedTestResult{}

	start := time.Now()
	deadline := start.Add(10 * time.Second)

	// 延迟测试
	latencyStart := time.Now()
	latencyConn, err := dialThroughSocks(socks5Addr, socks5Port, user, pass, "www.google.com", 443, deadline)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("延迟测试失败: %v", err)
		return result
	}
	result.Latency = time.Since(latencyStart)
	latencyConn.Close()

	// 上传速度测试（上传1KB数据）
	if time.Now().After(deadline) {
		result.Success = false
		result.Message = "测速超时"
		return result
	}
	uploadBody := bytes.Repeat([]byte("a"), 1024)
	_, uploadDuration, err := executeSocksHTTPRequest(socks5Addr, socks5Port, user, pass, "speed.cloudflare.com", "/__down?bytes=1000", "POST", uploadBody, deadline)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("上传测试失败: %v", err)
		return result
	}
	if uploadDuration <= 0 {
		uploadDuration = time.Millisecond
	}
	result.UploadSpeed = float64(len(uploadBody)) / uploadDuration.Seconds()

	// 下载速度测试（下载少量数据）
	if time.Now().After(deadline) {
		result.Success = false
		result.Message = "测速超时"
		return result
	}
	downloadBytes, downloadDuration, err := executeSocksHTTPRequest(socks5Addr, socks5Port, user, pass, "speed.cloudflare.com", "/__down?bytes=1000", "GET", nil, deadline)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("下载测试失败: %v", err)
		return result
	}
	if downloadDuration <= 0 {
		downloadDuration = time.Millisecond
	}
	result.DownloadSpeed = float64(downloadBytes) / downloadDuration.Seconds()

	result.Success = true
	result.Message = "测速完成"
	return result
}

func dialThroughSocks(socks5Addr, socks5Port, user, pass, targetHost string, targetPort int, deadline time.Time) (net.Conn, error) {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return nil, fmt.Errorf("已超出超时时间")
	}

	dialer := &net.Dialer{
		Timeout:  remaining,
		Deadline: deadline,
	}
	conn, err := dialer.Dial("tcp", net.JoinHostPort(socks5Addr, socks5Port))
	if err != nil {
		return nil, fmt.Errorf("连接SOCKS5代理失败: %w", err)
	}

	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, err
	}

	// SOCKS5握手
	if user != "" && pass != "" {
		_, err = conn.Write([]byte{0x05, 0x02, 0x00, 0x02})
	} else {
		_, err = conn.Write([]byte{0x05, 0x01, 0x00})
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5握手失败: %w", err)
	}

	buf := make([]byte, 2)
	_, err = conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5握手响应失败: %w", err)
	}

	if buf[1] == 0xFF {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5服务器不支持请求的认证方法")
	}

	if buf[1] == 0x02 {
		if user == "" || pass == "" {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5服务器要求认证但未提供凭据")
		}
		authReq := make([]byte, 0, 3+len(user)+len(pass))
		authReq = append(authReq, 0x01)
		authReq = append(authReq, byte(len(user)))
		authReq = append(authReq, []byte(user)...)
		authReq = append(authReq, byte(len(pass)))
		authReq = append(authReq, []byte(pass)...)

		if _, err = conn.Write(authReq); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5认证失败: %w", err)
		}

		authResp := make([]byte, 2)
		if _, err = conn.Read(authResp); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5认证响应失败: %w", err)
		}
		if authResp[1] != 0x00 {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5认证失败：用户名或密码错误")
		}
	}

	if len(targetHost) > 255 {
		targetHost = targetHost[:255]
	}

	req := make([]byte, 0, 7+len(targetHost))
	req = append(req, 0x05, 0x01, 0x00)
	req = append(req, 0x03)
	req = append(req, byte(len(targetHost)))
	req = append(req, []byte(targetHost)...)
	req = append(req, byte(targetPort>>8), byte(targetPort&0xff))

	if _, err = conn.Write(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5请求发送失败: %w", err)
	}

	resp := make([]byte, 10)
	if _, err = conn.Read(resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5响应读取失败: %w", err)
	}

	if len(resp) < 2 || resp[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("出站连接失败")
	}

	return conn, nil
}

func executeSocksHTTPRequest(socks5Addr, socks5Port, user, pass, host, path, method string, body []byte, deadline time.Time) (int, time.Duration, error) {
	conn, err := dialThroughSocks(socks5Addr, socks5Port, user, pass, host, 443, deadline)
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{ServerName: host})
	if err := tlsConn.Handshake(); err != nil {
		return 0, 0, fmt.Errorf("TLS握手失败: %w", err)
	}
	defer tlsConn.Close()

	start := time.Now()

	if method == "" {
		method = "GET"
	}

	requestHeader := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", method, path, host, len(body))
	if _, err := tlsConn.Write([]byte(requestHeader)); err != nil {
		return 0, 0, fmt.Errorf("请求发送失败: %w", err)
	}

	if len(body) > 0 {
		if _, err := tlsConn.Write(body); err != nil {
			return 0, 0, fmt.Errorf("请求体发送失败: %w", err)
		}
	}

	respData, err := io.ReadAll(tlsConn)
	if err != nil {
		return 0, 0, fmt.Errorf("响应读取失败: %w", err)
	}

	duration := time.Since(start)
	separator := []byte("\r\n\r\n")
	index := bytes.Index(respData, separator)
	if index == -1 {
		return 0, duration, fmt.Errorf("响应格式错误")
	}

	bodyData := respData[index+4:]
	return len(bodyData), duration, nil
}

// TestHysteria2Socks5 测试已运行的Hysteria2客户端的SOCKS5端口连通性
func TestHysteria2Socks5(socks5Addr string, socks5Port int) *TestResult {
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
		authReq = append(authReq, 0x01)            // 版本
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
	req = append(req, 0x03)             // ATYP: 域名
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

// TestHysteria2Connection 启动临时Hysteria2客户端测试连接
// 参数: server, port, password - Hysteria2服务器信息
//       sni - TLS SNI
//       insecure - 是否跳过证书验证
func TestHysteria2Connection(server, port, password, sni string, insecure bool) *SpeedTestResult {
	result := &SpeedTestResult{}

	// 验证参数
	if server == "" || port == "" || password == "" {
		result.Message = "Hysteria2配置不完整"
		return result
	}

	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		result.Message = fmt.Sprintf("获取程序路径失败: %v", err)
		return result
	}
	baseDir := filepath.Dir(exePath)
	hy2Binary := filepath.Join(baseDir, "bin", "hysteria2")

	// 检查 hysteria2 二进制文件是否存在
	if _, err := os.Stat(hy2Binary); os.IsNotExist(err) {
		result.Message = "Hysteria2客户端不存在"
		return result
	}

	// 生成随机SOCKS5端口 (60000-65000)
	rand.Seed(time.Now().UnixNano())
	socks5Port := 60000 + rand.Intn(5000)

	// 创建临时配置文件
	configDir := filepath.Join(baseDir, "proxy_configs")
	configFile := filepath.Join(configDir, fmt.Sprintf("hy2_test_%d.yaml", socks5Port))

	insecureStr := "false"
	if insecure {
		insecureStr = "true"
	}
	if sni == "" {
		sni = server
	}

	configContent := fmt.Sprintf(`server: %s:%s
auth: %s
tls:
    sni: %s
    insecure: %s
bandwidth:
    up: 100 mbps
    down: 100 mbps
socks5:
    listen: 127.0.0.1:%d
`, server, port, password, sni, insecureStr, socks5Port)

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		result.Message = fmt.Sprintf("创建配置文件失败: %v", err)
		return result
	}
	defer os.Remove(configFile) // 清理配置文件

	// 启动 Hysteria2 客户端
	cmd := exec.Command(hy2Binary, "client", "-c", configFile)
	if err := cmd.Start(); err != nil {
		result.Message = fmt.Sprintf("启动Hysteria2客户端失败: %v", err)
		return result
	}

	// 确保进程被清理
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	// 等待 SOCKS5 端口就绪 (最多等待5秒)
	socks5Addr := fmt.Sprintf("127.0.0.1:%d", socks5Port)
	portReady := false
	for i := 0; i < 50; i++ {
		conn, err := net.DialTimeout("tcp", socks5Addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			portReady = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !portReady {
		result.Message = "Hysteria2客户端启动超时"
		return result
	}

	// 只测试延迟，不测速（加快测试速度）
	start := time.Now()
	deadline := start.Add(10 * time.Second)
	latencyConn, err := dialThroughSocks("127.0.0.1", strconv.Itoa(socks5Port), "", "", "www.google.com", 443, deadline)
	if err != nil {
		result.Message = fmt.Sprintf("延迟测试失败: %v", err)
		return result
	}
	result.Latency = time.Since(start)
	latencyConn.Close()

	result.Success = true
	result.Message = "连接成功"
	return result
}
