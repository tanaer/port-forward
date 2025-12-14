package web

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
	"goForward/conf"
	"goForward/proxy"
	"goForward/proxy/xray"
	"goForward/sql"
)

// RemoteInstallRequest 远程安装请求
type RemoteInstallRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Hy2Port  int    `json:"hy2Port"`
	NodeName string `json:"nodeName"`
}

// RegisterRemoteInstallRoutes 注册远程安装路由
func RegisterRemoteInstallRoutes(r *gin.Engine) {
	r.POST("/proxy/remote-install", handleRemoteInstall)
	r.POST("/proxy/ip-check", handleIPCheck)
	r.POST("/proxy/remote-restart", handleRemoteRestart)
	r.POST("/proxy/update-ssh", handleUpdateSSHInfo)
	r.GET("/proxy/ssh-info/:id", handleGetProxySSHInfo)
}

func handleRemoteInstall(c *gin.Context) {
	var req RemoteInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的请求参数"})
		return
	}

	// 验证必填字段
	if req.Host == "" || req.User == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请填写完整的服务器信息"})
		return
	}

	// 默认端口
	if req.Port == 0 {
		req.Port = 22
	}
	if req.Hy2Port == 0 {
		req.Hy2Port = 443
	}

	// 连接SSH
	sshConfig := &ssh.ClientConfig{
		User: req.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(req.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH连接失败: %v", err),
		})
		return
	}
	defer client.Close()

	// 生成密码
	password := generateRandomPassword(16)

	// 安装Hysteria2脚本
	installScript := fmt.Sprintf(`
set -e

# 检测系统
if [ -f /etc/debian_version ]; then
    apt-get update -qq
    apt-get install -y -qq curl wget openssl
elif [ -f /etc/redhat-release ]; then
    yum install -y -q curl wget openssl
fi

# 下载并安装 Hysteria2
curl -fsSL https://get.hy2.sh/ | bash

# 创建配置目录
mkdir -p /etc/hysteria

# 生成自签名证书
openssl req -x509 -nodes -newkey ec:<(openssl ecparam -name prime256v1) \
    -keyout /etc/hysteria/server.key -out /etc/hysteria/server.crt \
    -subj "/CN=bing.com" -days 36500 2>/dev/null

# 创建配置文件
cat > /etc/hysteria/config.yaml << 'HYSTERIA_CONFIG'
listen: :%d

tls:
  cert: /etc/hysteria/server.crt
  key: /etc/hysteria/server.key

auth:
  type: password
  password: %s

masquerade:
  type: proxy
  proxy:
    url: https://bing.com
    rewriteHost: true
HYSTERIA_CONFIG

# 创建 systemd 服务
cat > /etc/systemd/system/hysteria-server.service << 'SERVICE_EOF'
[Unit]
Description=Hysteria Server Service (config.yaml)
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/hysteria server --config /etc/hysteria/config.yaml
WorkingDirectory=/etc/hysteria
Restart=on-failure
RestartSec=10
LimitNOFILE=infinity

[Install]
WantedBy=multi-user.target
SERVICE_EOF

# 启动服务
systemctl daemon-reload
systemctl enable hysteria-server
systemctl restart hysteria-server

# 输出结果
echo "INSTALL_SUCCESS"
echo "PASSWORD:%s"
echo "PORT:%d"
`, req.Hy2Port, password, password, req.Hy2Port)

	// 执行安装脚本
	session, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("创建SSH会话失败: %v", err),
		})
		return
	}
	defer session.Close()

	output, err := session.CombinedOutput(installScript)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("安装失败: %v\n输出: %s", err, string(output)),
		})
		return
	}

	// 检查安装结果
	if !strings.Contains(string(output), "INSTALL_SUCCESS") {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("安装未完成: %s", string(output)),
		})
		return
	}

	// 生成节点名称
	nodeName := req.NodeName
	if nodeName == "" {
		nodeName = fmt.Sprintf("Hy2-%s", req.Host)
	}

	// 生成密钥对
	keys, err := xray.GenerateRealityKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("生成密钥失败: %v", err),
		})
		return
	}

	shortId, err := xray.GenerateShortId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("生成ShortId失败: %v", err),
		})
		return
	}

	uuid, err := proxy.GenerateUUID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("生成UUID失败: %v", err),
		})
		return
	}

	// 获取可用端口
	inboundPort := proxy.GetRandomAvailablePort()
	socks5Port := proxy.GetRandomAvailablePortFromRange(10808, 10808+10000)

	// 创建代理配置
	proxyConfig := conf.ProxyConfig{
		Name:              nodeName,
		InboundPort:       inboundPort,
		UUID:              uuid,
		Flow:              "xtls-rprx-vision",
		RealityDest:       "www.microsoft.com:443",
		RealityServerName: "www.microsoft.com",
		PrivateKey:        keys.PrivateKey,
		PublicKey:         keys.PublicKey,
		ShortId:           shortId,
		OutboundType:      "hysteria2",
		Hy2Server:         req.Host,
		Hy2Port:           fmt.Sprintf("%d", req.Hy2Port),
		Hy2Password:       password,
		Hy2SNI:            "bing.com",
		Hy2Insecure:       true,
		Hy2UpMbps:         100,
		Hy2DownMbps:       100,
		Hy2Socks5Port:     socks5Port,
		Remark:            fmt.Sprintf("远程安装于 %s", time.Now().Format("2006-01-02 15:04")),
		Status:            0,
		// 保存SSH信息
		SSHHost:     req.Host,
		SSHPort:     req.Port,
		SSHUser:     req.User,
		SSHPassword: req.Password,
	}

	// 保存到数据库
	id := sql.AddProxy(proxyConfig)
	if id == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "保存代理配置失败",
		})
		return
	}

	// 启动代理
	pm := proxy.GetProxyManager()
	if err := pm.StartProxy(id); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"nodeName": nodeName,
			"warning":  fmt.Sprintf("节点已添加但启动失败: %v", err),
			"proxyId":  id,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"nodeName": nodeName,
		"proxyId":  id,
		"server":   fmt.Sprintf("%s:%d", req.Host, req.Hy2Port),
	})
}

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// 如果加密随机失败，使用备用方案
			result[i] = charset[i%len(charset)]
		} else {
			result[i] = charset[num.Int64()]
		}
	}
	return string(result)
}

// IPCheckRequest IP检测请求
type IPCheckRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	ProxyId  int    `json:"proxyId"` // 如果有，保存结果到对应代理
}

// handleIPCheck 处理IP质量检测
func handleIPCheck(c *gin.Context) {
	var req IPCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的请求参数"})
		return
	}

	if req.Host == "" || req.User == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请填写完整的服务器信息"})
		return
	}

	if req.Port == 0 {
		req.Port = 22
	}

	// 连接SSH
	sshConfig := &ssh.ClientConfig{
		User: req.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(req.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH连接失败: %v", err),
		})
		return
	}
	defer client.Close()

	// 执行IP检测 - 使用多种方式获取IP信息
	session, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("创建SSH会话失败: %v", err),
		})
		return
	}
	defer session.Close()

	// 先尝试简单的IP信息获取，再尝试完整检测
	ipCheckScript := `
#!/bin/bash
# 获取基本IP信息
ip_info=$(curl -s --connect-timeout 10 ipinfo.io 2>/dev/null)
if [ -n "$ip_info" ]; then
    echo "$ip_info"
    exit 0
fi

# 备用：使用 ip-api.com
ip_info=$(curl -s --connect-timeout 10 "http://ip-api.com/json?lang=zh-CN" 2>/dev/null)
if [ -n "$ip_info" ]; then
    echo "$ip_info"
    exit 0
fi

# 最后尝试 IP.Check.Place
bash <(curl -sL --connect-timeout 15 IP.Check.Place) -j 2>/dev/null || echo '{"error":"检测失败"}'
`

	output, err := session.CombinedOutput(ipCheckScript)
	outputStr := string(output)

	// 即使有错误也尝试解析输出
	jsonOutput := cleanJSONOutput(outputStr)

	// 检查是否获取到有效的JSON
	if jsonOutput == "" || jsonOutput == outputStr {
		// 如果没有获取到JSON，返回原始输出作为错误信息
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"result":  fmt.Sprintf(`{"error":"检测完成但无有效数据","raw":"%s"}`, strings.ReplaceAll(outputStr, "\"", "'")),
			})
			return
		}
	}

	// 如果有proxyId，保存结果到数据库
	if req.ProxyId > 0 {
		proxyConfig := sql.GetProxy(req.ProxyId)
		if proxyConfig.Id != 0 {
			proxyConfig.IPCheckResult = jsonOutput
			proxyConfig.IPCheckTime = time.Now()
			// 同时更新SSH信息
			proxyConfig.SSHHost = req.Host
			proxyConfig.SSHPort = req.Port
			proxyConfig.SSHUser = req.User
			proxyConfig.SSHPassword = req.Password
			sql.UpdateProxy(proxyConfig)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  jsonOutput,
	})
}

// cleanJSONOutput 清理JSON输出，移除ANSI转义码
func cleanJSONOutput(output string) string {
	// 查找JSON开始位置
	start := strings.Index(output, "{")
	if start == -1 {
		return output
	}
	return output[start:]
}

// RemoteRestartRequest 远程重启请求
type RemoteRestartRequest struct {
	ProxyId int `json:"proxyId"`
}

// handleRemoteRestart 处理远程重启Hysteria2
func handleRemoteRestart(c *gin.Context) {
	var req RemoteRestartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的请求参数"})
		return
	}

	if req.ProxyId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供代理ID"})
		return
	}

	// 获取代理配置
	proxyConfig := sql.GetProxy(req.ProxyId)
	if proxyConfig.Id == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "代理配置不存在"})
		return
	}

	if proxyConfig.SSHHost == "" || proxyConfig.SSHUser == "" || proxyConfig.SSHPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "该线路没有保存SSH信息"})
		return
	}

	// 连接SSH
	sshConfig := &ssh.ClientConfig{
		User: proxyConfig.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(proxyConfig.SSHPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	port := proxyConfig.SSHPort
	if port == 0 {
		port = 22
	}

	addr := fmt.Sprintf("%s:%d", proxyConfig.SSHHost, port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH连接失败: %v", err),
		})
		return
	}
	defer client.Close()

	// 执行重启命令
	session, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("创建SSH会话失败: %v", err),
		})
		return
	}
	defer session.Close()

	output, err := session.CombinedOutput("systemctl restart hysteria-server")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("重启失败: %v, 输出: %s", err, string(output)),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "远程Hysteria2服务已重启",
	})
}

// UpdateSSHInfoRequest 更新SSH信息请求
type UpdateSSHInfoRequest struct {
	ProxyId  int    `json:"proxyId"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// handleUpdateSSHInfo 更新代理的SSH信息
func handleUpdateSSHInfo(c *gin.Context) {
	var req UpdateSSHInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的请求参数"})
		return
	}

	if req.ProxyId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供代理ID"})
		return
	}

	proxyConfig := sql.GetProxy(req.ProxyId)
	if proxyConfig.Id == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "代理配置不存在"})
		return
	}

	proxyConfig.SSHHost = req.Host
	proxyConfig.SSHPort = req.Port
	proxyConfig.SSHUser = req.User
	proxyConfig.SSHPassword = req.Password

	sql.UpdateProxy(proxyConfig)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SSH信息已更新",
	})
}

// GetProxySSHInfoHandler 获取代理的SSH信息
func handleGetProxySSHInfo(c *gin.Context) {
	idStr := c.Param("id")
	id := 0
	fmt.Sscanf(idStr, "%d", &id)

	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的代理ID"})
		return
	}

	proxyConfig := sql.GetProxy(id)
	if proxyConfig.Id == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "代理配置不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"ssh": gin.H{
			"host":     proxyConfig.SSHHost,
			"port":     proxyConfig.SSHPort,
			"user":     proxyConfig.SSHUser,
			"password": proxyConfig.SSHPassword,
		},
		"ipCheck": gin.H{
			"result":    proxyConfig.IPCheckResult,
			"checkTime": proxyConfig.IPCheckTime,
		},
	})
}
