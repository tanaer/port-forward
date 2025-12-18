package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"goForward/conf"
	"goForward/proxy/hysteria"
	"goForward/proxy/vmess"
	"goForward/sql"
)

// RegisterOutboundRoutes 注册出站配置路由
func RegisterOutboundRoutes(r *gin.Engine) {
	// 获取出站配置列表
	r.GET("/outbound/list", func(c *gin.Context) {
		list, err := sql.GetOutboundList()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": list,
		})
	})

	// 获取单个出站配置
	r.GET("/outbound/get/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		outbound, err := sql.GetOutbound(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": outbound,
		})
	})

	// 获取当前启用的出站配置
	r.GET("/outbound/active", func(c *gin.Context) {
		outbound, err := sql.GetActiveOutbound()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": nil,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": outbound,
		})
	})

	// 添加出站配置
	r.POST("/outbound/add", func(c *gin.Context) {
		outbound := parseOutboundForm(c)

		id, err := sql.AddOutbound(&outbound)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "添加成功",
			"id":   id,
		})
	})

	// 更新出站配置
	r.POST("/outbound/update/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))

		outbound := parseOutboundForm(c)
		outbound.Id = id

		err := sql.UpdateOutbound(&outbound)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "更新成功",
		})
	})

	// 删除出站配置
	r.POST("/outbound/delete/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))

		err := sql.DeleteOutbound(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "删除成功",
		})
	})

	// 启用出站配置
	r.POST("/outbound/activate/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))

		err := sql.SetOutboundActive(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "启用成功",
		})
	})

	// 停用出站配置
	r.POST("/outbound/deactivate/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))

		err := sql.SetOutboundInactive(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "停用成功",
		})
	})

	// 解析 Hysteria2 订阅
	r.POST("/outbound/parse-hy2", func(c *gin.Context) {
		subscription := c.PostForm("subscription")
		result := parseHy2Subscription(subscription)
		c.JSON(http.StatusOK, result)
	})

	// 解析 VMess 订阅
	r.POST("/outbound/parse-vmess", func(c *gin.Context) {
		subscription := c.PostForm("subscription")
		result := parseVmessSubscription(subscription)
		c.JSON(http.StatusOK, result)
	})
}

// parseHy2Subscription 解析 Hysteria2 订阅链接
func parseHy2Subscription(subscription string) gin.H {
	subscription = strings.TrimSpace(subscription)
	if subscription == "" {
		return gin.H{"success": false, "error": "订阅链接不能为空"}
	}

	// 使用 hysteria 包解析
	config, err := hysteria.ParseSubscription(subscription)
	if err != nil {
		return gin.H{"success": false, "error": err.Error()}
	}

	// 转换为参数格式
	params := hysteria.Hy2ConfigToParams(config)

	return gin.H{
		"success":  true,
		"server":   params.Server,
		"port":     params.Port,
		"password": params.Password,
		"obfs":     params.Obfs,
		"obfsPass": params.ObfsPass,
		"sni":      params.SNI,
		"insecure": params.Insecure,
		"upMbps":   params.UpMbps,
		"downMbps": params.DownMbps,
	}
}

// parseVmessSubscription 解析 VMess 订阅链接
func parseVmessSubscription(subscription string) gin.H {
	subscription = strings.TrimSpace(subscription)
	if subscription == "" {
		return gin.H{"success": false, "error": "订阅链接不能为空"}
	}

	// 使用 vmess 包解析
	config, err := vmess.ParseSubscription(subscription)
	if err != nil {
		return gin.H{"success": false, "error": err.Error()}
	}

	return gin.H{
		"success":    true,
		"server":     config.Server,
		"port":       config.Port,
		"uuid":       config.UUID,
		"alterID":    config.AlterID,
		"security":   config.Security,
		"network":    config.Network,
		"tls":        config.TLS,
		"serverName": config.ServerName,
		"wsPath":     config.WsPath,
		"wsHost":     config.WsHost,
	}
}

// parseOutboundForm 解析出站配置表单
func parseOutboundForm(c *gin.Context) conf.OutboundConfig {
	status, _ := strconv.Atoi(c.PostForm("status"))

	outbound := conf.OutboundConfig{
		Name:   strings.TrimSpace(c.PostForm("name")),
		Type:   c.PostForm("type"),
		Status: status,
	}

	switch outbound.Type {
	case "hysteria2":
		outbound.Hy2Server = strings.TrimSpace(c.PostForm("hy2Server"))
		outbound.Hy2Port = strings.TrimSpace(c.PostForm("hy2Port"))
		outbound.Hy2Password = strings.TrimSpace(c.PostForm("hy2Password"))
		outbound.Hy2Subscription = strings.TrimSpace(c.PostForm("hy2Subscription"))
		outbound.Hy2Obfs = c.PostForm("hy2Obfs")
		outbound.Hy2ObfsPassword = c.PostForm("hy2ObfsPassword")
		outbound.Hy2SNI = c.PostForm("hy2SNI")
		outbound.Hy2Insecure = c.PostForm("hy2Insecure") == "1" || c.PostForm("hy2Insecure") == "true"
		outbound.Hy2UpMbps, _ = strconv.Atoi(c.PostForm("hy2UpMbps"))
		outbound.Hy2DownMbps, _ = strconv.Atoi(c.PostForm("hy2DownMbps"))
		if outbound.Hy2UpMbps == 0 {
			outbound.Hy2UpMbps = 100
		}
		if outbound.Hy2DownMbps == 0 {
			outbound.Hy2DownMbps = 100
		}

	case "socks5":
		outbound.Socks5Addr = strings.TrimSpace(c.PostForm("socks5Addr"))
		outbound.Socks5Port, _ = strconv.Atoi(c.PostForm("socks5Port"))
		outbound.Socks5User = strings.TrimSpace(c.PostForm("socks5User"))
		outbound.Socks5Password = strings.TrimSpace(c.PostForm("socks5Password"))
		if outbound.Socks5Port == 0 {
			outbound.Socks5Port = 1080
		}

	case "vmess":
		outbound.VmessServer = strings.TrimSpace(c.PostForm("vmessServer"))
		outbound.VmessPort, _ = strconv.Atoi(c.PostForm("vmessPort"))
		outbound.VmessUUID = strings.TrimSpace(c.PostForm("vmessUUID"))
		outbound.VmessAlterID, _ = strconv.Atoi(c.PostForm("vmessAlterID"))
		outbound.VmessSecurity = c.PostForm("vmessSecurity")
		outbound.VmessNetwork = c.PostForm("vmessNetwork")
		outbound.VmessTLS = c.PostForm("vmessTLS") == "1" || c.PostForm("vmessTLS") == "true"
		outbound.VmessServerName = c.PostForm("vmessServerName")
		outbound.VmessWsPath = c.PostForm("vmessWsPath")
		outbound.VmessWsHost = c.PostForm("vmessWsHost")
		outbound.VmessSubscription = strings.TrimSpace(c.PostForm("vmessSubscription"))
		if outbound.VmessPort == 0 {
			outbound.VmessPort = 443
		}
		if outbound.VmessSecurity == "" {
			outbound.VmessSecurity = "auto"
		}
		if outbound.VmessNetwork == "" {
			outbound.VmessNetwork = "tcp"
		}
	}

	return outbound
}
