package web

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"goForward/conf"
	"goForward/proxy"
	"goForward/proxy/hysteria"
	"goForward/proxy/vmess"
	"goForward/proxy/xray"
	"goForward/sql"
)

// RegisterProxyRoutes 注册代理相关路由
func RegisterProxyRoutes(r *gin.Engine) {
	// 代理管理页面
	r.GET("/proxy", func(c *gin.Context) {
		c.HTML(http.StatusOK, "proxy_list.tmpl", gin.H{
			"proxyList": sql.GetProxyList(),
			"stats":     sql.GetProxyStats(),
		})
	})

	// 添加代理页面
	r.GET("/proxy/add", func(c *gin.Context) {
		c.HTML(http.StatusOK, "proxy_add.tmpl", gin.H{
			"realityDomains": xray.GetRealityDomainList(),
		})
	})

	// 生成密钥对
	r.GET("/proxy/generate-keys", func(c *gin.Context) {
		keys, err := xray.GenerateRealityKeys()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		shortId, err := xray.GenerateShortId()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		uuid, err := proxy.GenerateUUID()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"privateKey": keys.PrivateKey,
			"publicKey":  keys.PublicKey,
			"shortId":    shortId,
			"uuid":       uuid,
		})
	})

	// 解析Hysteria2订阅
	r.POST("/proxy/parse-hy2", func(c *gin.Context) {
		subUrl := c.PostForm("subscription")
		if subUrl == "" {
			c.JSON(400, gin.H{"error": "订阅链接不能为空"})
			return
		}

		hy2Config, err := hysteria.ParseSubscription(subUrl)
		if err != nil {
			c.JSON(400, gin.H{"error": fmt.Sprintf("解析失败: %v", err)})
			return
		}

		params := hysteria.Hy2ConfigToParams(hy2Config)
		c.JSON(200, gin.H{
			"server":      params.Server,
			"port":        params.Port,
			"password":    params.Password,
			"obfs":        params.Obfs,
			"obfsPass":    params.ObfsPass,
			"sni":         params.SNI,
			"insecure":    params.Insecure,
			"upMbps":      params.UpMbps,
			"downMbps":    params.DownMbps,
			"socks5Port":  params.Socks5Port,
		})
	})

	// 解析 VMess 订阅
	r.POST("/proxy/parse-vmess", func(c *gin.Context) {
		subUrl := strings.TrimSpace(c.PostForm("subscription"))

		if subUrl == "" {
			c.JSON(400, gin.H{"error": "订阅链接不能为空"})
			return
		}

		vmessConfig, err := vmess.ParseSubscription(subUrl)
		if err != nil {
			c.JSON(400, gin.H{"error": fmt.Sprintf("解析失败: %v", err)})
			return
		}

		c.JSON(200, gin.H{
			"server":     vmessConfig.Server,
			"port":       vmessConfig.Port,
			"uuid":       vmessConfig.UUID,
			"alterId":    vmessConfig.AlterID,
			"security":   vmessConfig.Security,
			"network":    vmessConfig.Network,
			"tls":        vmessConfig.TLS,
			"serverName": vmessConfig.ServerName,
			"wsPath":     vmessConfig.WsPath,
			"wsHost":     vmessConfig.WsHost,
		})
	})

	// 添加代理
	r.POST("/proxy/add", func(c *gin.Context) {
		// 解析表单
		port, _ := strconv.Atoi(c.PostForm("inboundPort"))
		upMbps, _ := strconv.Atoi(c.PostForm("hy2UpMbps"))
		downMbps, _ := strconv.Atoi(c.PostForm("hy2DownMbps"))
		socks5Port, _ := strconv.Atoi(c.PostForm("hy2Socks5Port"))
		insecure := c.PostForm("hy2Insecure") == "1"
		outboundType := c.PostForm("outboundType")
		socks5OutPort, _ := strconv.Atoi(c.PostForm("socks5Port"))

		// 默认值
		if port == 0 {
			port = proxy.GetRandomAvailablePort()
		}
		if outboundType == "" {
			outboundType = "hysteria2"
		}
		if upMbps == 0 {
			upMbps = 100
		}
		if downMbps == 0 {
			downMbps = 100
		}
		if socks5Port == 0 {
			socks5Port = 10808
		}

		// 检查端口是否可用
		if !sql.CheckProxyPortAvailable(port, 0) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("端口 %d 已被占用", port),
				"suc": false,
			})
			return
		}

		// 解析 VMess 参数
		vmessPort, _ := strconv.Atoi(c.PostForm("vmessPort"))
		vmessAlterID, _ := strconv.Atoi(c.PostForm("vmessAlterID"))
		vmessTLS := c.PostForm("vmessTLS") == "1"

		// 创建配置
		proxyConfig := conf.ProxyConfig{
			Name:              c.PostForm("name"),
			InboundPort:       port,
			UUID:              c.PostForm("uuid"),
			Flow:              "xtls-rprx-vision",
			RealityDest:       c.PostForm("realityDest"),
			RealityServerName: c.PostForm("realityServerName"),
			PrivateKey:        c.PostForm("privateKey"),
			PublicKey:         c.PostForm("publicKey"),
			ShortId:           c.PostForm("shortId"),
			OutboundType:      outboundType,
			Hy2Server:         c.PostForm("hy2Server"),
			Hy2Port:           c.PostForm("hy2Port"),
			Hy2Password:       c.PostForm("hy2Password"),
			Hy2Subscription:   c.PostForm("hy2Subscription"),
			Hy2Obfs:           c.PostForm("hy2Obfs"),
			Hy2ObfsPassword:   c.PostForm("hy2ObfsPassword"),
			Hy2SNI:            c.PostForm("hy2SNI"),
			Hy2Insecure:       insecure,
			Hy2UpMbps:         upMbps,
			Hy2DownMbps:       downMbps,
			Hy2Socks5Port:     socks5Port,
			Socks5Addr:        c.PostForm("socks5Addr"),
			Socks5Port:        socks5OutPort,
			Socks5User:        c.PostForm("socks5User"),
			Socks5Password:    c.PostForm("socks5Password"),
			VmessServer:       c.PostForm("vmessServer"),
			VmessPort:         vmessPort,
			VmessUUID:         c.PostForm("vmessUUID"),
			VmessAlterID:      vmessAlterID,
			VmessSecurity:     c.PostForm("vmessSecurity"),
			VmessNetwork:      c.PostForm("vmessNetwork"),
			VmessTLS:          vmessTLS,
			VmessServerName:   c.PostForm("vmessServerName"),
			VmessWsPath:       c.PostForm("vmessWsPath"),
			VmessWsHost:       c.PostForm("vmessWsHost"),
			VmessSubscription: c.PostForm("vmessSubscription"),
			Remark:            c.PostForm("remark"),
			Status:            0,
		}

		// 保存到数据库
		id := sql.AddProxy(proxyConfig)
		if id == 0 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "添加失败",
				"suc": false,
			})
			return
		}

		// 启动代理
		proxyConfig.Id = id
		pm := proxy.GetProxyManager()
		if err := pm.StartProxy(id); err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("添加成功但启动失败: %v", err),
				"suc": false,
			})
			return
		}

		c.HTML(200, "msg.tmpl", gin.H{
			"msg": "添加成功",
			"suc": true,
		})
	})

	// 编辑代理页面
	r.GET("/proxy/edit/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		proxyConfig := sql.GetProxy(id)
		if proxyConfig.Id == 0 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "代理配置不存在",
				"suc": false,
			})
			return
		}

		c.HTML(http.StatusOK, "proxy_edit.tmpl", gin.H{
			"proxy":          proxyConfig,
			"realityDomains": xray.GetRealityDomainList(),
		})
	})

	// 更新代理
	r.POST("/proxy/edit/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		existing := sql.GetProxy(id)
		if existing.Id == 0 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "代理配置不存在",
				"suc": false,
			})
			return
		}

		// 解析表单
		port, _ := strconv.Atoi(c.PostForm("inboundPort"))
		upMbps, _ := strconv.Atoi(c.PostForm("hy2UpMbps"))
		downMbps, _ := strconv.Atoi(c.PostForm("hy2DownMbps"))
		socks5Port, _ := strconv.Atoi(c.PostForm("hy2Socks5Port"))
		insecure := c.PostForm("hy2Insecure") == "1"
		outboundType := c.PostForm("outboundType")
		socks5OutPort, _ := strconv.Atoi(c.PostForm("socks5Port"))

		// 默认 outbound 类型
		if outboundType == "" {
			outboundType = "hysteria2"
		}

		// 检查端口是否可用
		if port != existing.InboundPort && !sql.CheckProxyPortAvailable(port, id) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("端口 %d 已被占用", port),
				"suc": false,
			})
			return
		}

		// 解析 VMess 参数
		vmessPortEdit, _ := strconv.Atoi(c.PostForm("vmessPort"))
		vmessAlterIDEdit, _ := strconv.Atoi(c.PostForm("vmessAlterID"))
		vmessTLSEdit := c.PostForm("vmessTLS") == "1"

		// 更新配置
		existing.Name = c.PostForm("name")
		existing.InboundPort = port
		existing.UUID = c.PostForm("uuid")
		existing.RealityDest = c.PostForm("realityDest")
		existing.RealityServerName = c.PostForm("realityServerName")
		existing.PrivateKey = c.PostForm("privateKey")
		existing.PublicKey = c.PostForm("publicKey")
		existing.ShortId = c.PostForm("shortId")
		existing.Hy2Server = c.PostForm("hy2Server")
		existing.Hy2Port = c.PostForm("hy2Port")
		existing.Hy2Password = c.PostForm("hy2Password")
		existing.Hy2Subscription = c.PostForm("hy2Subscription")
		existing.Hy2Obfs = c.PostForm("hy2Obfs")
		existing.Hy2ObfsPassword = c.PostForm("hy2ObfsPassword")
		existing.Hy2SNI = c.PostForm("hy2SNI")
		existing.Hy2Insecure = insecure
		existing.Hy2UpMbps = upMbps
		existing.Hy2DownMbps = downMbps
		existing.Hy2Socks5Port = socks5Port
		existing.Remark = c.PostForm("remark")
		existing.OutboundType = outboundType
		existing.Socks5Addr = c.PostForm("socks5Addr")
		existing.Socks5Port = socks5OutPort
		existing.Socks5User = c.PostForm("socks5User")
		existing.Socks5Password = c.PostForm("socks5Password")
		existing.VmessServer = c.PostForm("vmessServer")
		existing.VmessPort = vmessPortEdit
		existing.VmessUUID = c.PostForm("vmessUUID")
		existing.VmessAlterID = vmessAlterIDEdit
		existing.VmessSecurity = c.PostForm("vmessSecurity")
		existing.VmessNetwork = c.PostForm("vmessNetwork")
		existing.VmessTLS = vmessTLSEdit
		existing.VmessServerName = c.PostForm("vmessServerName")
		existing.VmessWsPath = c.PostForm("vmessWsPath")
		existing.VmessWsHost = c.PostForm("vmessWsHost")
		existing.VmessSubscription = c.PostForm("vmessSubscription")

		if !sql.UpdateProxy(existing) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "更新失败",
				"suc": false,
			})
			return
		}

		// 如果代理正在运行，重启
		pm := proxy.GetProxyManager()
		status := pm.GetProxyStatus(id)
		if running, ok := status["running"].(bool); ok && running {
			if err := pm.RestartProxy(id); err != nil {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": fmt.Sprintf("更新成功但重启失败: %v", err),
					"suc": false,
				})
				return
			}
		}

		c.HTML(200, "msg.tmpl", gin.H{
			"msg": "更新成功",
			"suc": true,
		})
	})

	// 启动/停止代理
	r.GET("/proxy/toggle/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		proxyConfig := sql.GetProxy(id)
		if proxyConfig.Id == 0 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "代理配置不存在",
				"suc": false,
			})
			return
		}

		pm := proxy.GetProxyManager()
		var err error

		if proxyConfig.Status == 0 {
			// 停止
			err = pm.StopProxy(id)
		} else {
			// 启动
			err = pm.StartProxy(id)
		}

		if err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("操作失败: %v", err),
				"suc": false,
			})
			return
		}

		c.HTML(200, "msg.tmpl", gin.H{
			"msg": "操作成功",
			"suc": true,
		})
	})

	// 删除代理
	r.GET("/proxy/delete/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		pm := proxy.GetProxyManager()

		if err := pm.DeleteProxy(id); err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("删除失败: %v", err),
				"suc": false,
			})
			return
		}

		c.HTML(200, "msg.tmpl", gin.H{
			"msg": "删除成功",
			"suc": true,
		})
	})

	// 生成订阅
	r.GET("/proxy/subscription/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		proxyConfig := sql.GetProxy(id)
		if proxyConfig.Id == 0 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "代理配置不存在",
				"suc": false,
			})
			return
		}

		// 获取服务器IP
		serverIP := c.Query("server")
		if serverIP == "" {
			serverIP = c.Request.Host
			if strings.Contains(serverIP, ":") {
				serverIP = strings.Split(serverIP, ":")[0]
			}
		}

		// 生成订阅
		pm := proxy.GetProxyManager()
		token, err := pm.GenerateSubscriptionForProxy(id, serverIP)
		if err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("生成订阅失败: %v", err),
				"suc": false,
			})
			return
		}

		// 生成订阅链接
		subUrl := fmt.Sprintf("http://%s/sub/%s", c.Request.Host, token)

		// 生成VLESS链接
		sg := proxy.SubscriptionGenerator{
			ServerIP:  serverIP,
			Port:      proxyConfig.InboundPort,
			UUID:      proxyConfig.UUID,
			PublicKey: proxyConfig.PublicKey,
			ShortId:   proxyConfig.ShortId,
			SNI:       proxyConfig.RealityServerName,
			Remark:    proxyConfig.Name,
		}
		vlessLink := sg.GenerateVLESSLink()

		c.HTML(http.StatusOK, "proxy_subscription.tmpl", gin.H{
			"proxy":     proxyConfig,
			"subUrl":    subUrl,
			"vlessLink": vlessLink,
			"token":     token,
		})
	})

	// 订阅接口
	r.GET("/sub/:token", func(c *gin.Context) {
		token := c.Param("token")
		sub := sql.GetSubscriptionByToken(token)
		if sub.Id == 0 {
			c.String(404, "订阅不存在")
			return
		}

		proxyConfig := sql.GetProxy(sub.ProxyId)
		if proxyConfig.Id == 0 {
			c.String(404, "代理配置不存在")
			return
		}

		// 获取服务器IP
		serverIP := c.Request.Host
		if strings.Contains(serverIP, ":") {
			serverIP = strings.Split(serverIP, ":")[0]
		}

		// 生成VLESS链接
		sg := proxy.SubscriptionGenerator{
			ServerIP:  serverIP,
			Port:      proxyConfig.InboundPort,
			UUID:      proxyConfig.UUID,
			PublicKey: proxyConfig.PublicKey,
			ShortId:   proxyConfig.ShortId,
			SNI:       proxyConfig.RealityServerName,
			Remark:    proxyConfig.Name,
		}
		vlessLink := sg.GenerateVLESSLink()

		// 返回base64编码的订阅
		content := base64.StdEncoding.EncodeToString([]byte(vlessLink))
		c.String(200, content)
	})

	// 获取代理状态
	r.GET("/proxy/status/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		pm := proxy.GetProxyManager()
		status := pm.GetProxyStatus(id)
		c.JSON(200, status)
	})

	// 测试代理连接
	r.GET("/proxy/test/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		proxyConfig := sql.GetProxy(id)
		if proxyConfig.Id == 0 {
			c.JSON(404, gin.H{"error": "代理配置不存在"})
			return
		}

		var result *proxy.TestResult

		switch proxyConfig.OutboundType {
		case "hysteria2":
			result = proxy.TestHysteria2Connection(proxyConfig.Hy2Server, proxyConfig.Hy2Port)
		case "vmess":
			result = proxy.TestVMessConnection(proxyConfig.VmessServer, proxyConfig.VmessPort)
		case "socks5":
			result = proxy.TestSOCKS5Connection(proxyConfig.Socks5Addr, proxyConfig.Socks5Port)
		default:
			c.JSON(400, gin.H{"error": "不支持的代理类型"})
			return
		}

		c.JSON(200, result)
	})
}
