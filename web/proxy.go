package web

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"goForward/conf"
	"goForward/proxy"
	"goForward/proxy/hysteria"
	"goForward/proxy/vmess"
	"goForward/proxy/xray"
	"goForward/quality"
	"goForward/sql"
	"goForward/version"
)

// RegisterProxyRoutes 注册代理相关路由
func RegisterProxyRoutes(r *gin.Engine) {
	// 代理管理页面
	r.GET("/proxy", func(c *gin.Context) {
		qualityCfg := conf.QualityMonitor
		intervalSeconds := int(qualityCfg.Interval / time.Second)
		if intervalSeconds <= 0 {
			intervalSeconds = int(conf.DefaultQualityMonitorConfig.Interval / time.Second)
		}

		proxies := sql.GetProxyList()
		for i := range proxies {
			up, down := sql.GetProxyTodayTraffic(proxies[i].Id)
			proxies[i].TodayTraffic = sql.FormatTraffic(up + down)
		}

		c.HTML(http.StatusOK, "proxy_list.tmpl", gin.H{
			"proxyList":              proxies,
			"stats":                  sql.GetProxyStats(),
			"version":                version.Version,
			"qualityConfig":          qualityCfg,
			"qualityProxySpec":       conf.FormatProxyIDs(qualityCfg.ProxyIDs),
			"qualityIntervalSeconds": intervalSeconds,
			"qualityRunning":         quality.IsRunning(),
		})
	})

	// 添加代理页面
	r.GET("/proxy/add", func(c *gin.Context) {
		c.HTML(http.StatusOK, "proxy_add.tmpl", gin.H{
			"realityDomains": xray.GetRealityDomainList(),
			"version":        version.Version,
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
			"server":     params.Server,
			"port":       params.Port,
			"password":   params.Password,
			"obfs":       params.Obfs,
			"obfsPass":   params.ObfsPass,
			"sni":        params.SNI,
			"insecure":   params.Insecure,
			"upMbps":     params.UpMbps,
			"downMbps":   params.DownMbps,
			"socks5Port": params.Socks5Port,
		})
	})

	// 导出代理配置
	r.POST("/proxy/export", func(c *gin.Context) {
		idsStr := c.PostForm("ids")
		var ids []int
		if idsStr != "" {
			// 解析ID列表 (逗号分隔)
			for _, idStr := range strings.Split(idsStr, ",") {
				id, _ := strconv.Atoi(strings.TrimSpace(idStr))
				if id > 0 {
					ids = append(ids, id)
				}
			}
		}

		jsonData, err := proxy.ExportProxies(ids)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// 返回JSON文件下载
		c.Header("Content-Disposition", "attachment; filename=proxies_export.json")
		c.Data(200, "application/json", []byte(jsonData))
	})

	// 导入代理配置
	r.POST("/proxy/import", func(c *gin.Context) {
		jsonData := c.PostForm("config")
		if jsonData == "" {
			c.JSON(400, gin.H{"error": "配置数据不能为空"})
			return
		}

		importedIds, err := proxy.ImportProxies(jsonData)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"success": true,
			"count":   len(importedIds),
			"ids":     importedIds,
		})
	})

	// 批量重启全部代理
	r.POST("/proxy/batch-restart", func(c *gin.Context) {
		// 获取所有代理
		proxies := sql.GetProxyList()

		if len(proxies) == 0 {
			c.JSON(400, gin.H{"error": "没有找到代理配置"})
			return
		}

		success := 0
		failed := 0
		var failures []string

		proxyManager := proxy.GetProxyManager()

		for _, proxyConfig := range proxies {
			// 先停止代理
			if err := proxyManager.StopProxy(proxyConfig.Id); err != nil {
				fmt.Printf("[批量重启] 停止代理 %d 失败: %v\n", proxyConfig.Id, err)
			}

			// 等待一秒
			time.Sleep(1 * time.Second)

			// 重新启动代理
			if err := proxyManager.StartProxy(proxyConfig.Id); err != nil {
				failed++
				failures = append(failures, fmt.Sprintf("代理 %d (%s): %v", proxyConfig.Id, proxyConfig.Name, err))
			} else {
				success++
			}
		}

		c.JSON(200, gin.H{
			"total":   len(proxies),
			"success": success,
			"failed":  failed,
			"details": failures,
		})
	})

	// 单个代理重启 (AJAX)
	r.POST("/proxy/restart/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"success": false, "error": "无效的代理ID"})
			return
		}

		proxyManager := proxy.GetProxyManager()
		if err := proxyManager.RestartProxy(id); err != nil {
			c.JSON(500, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"success": true, "message": "代理重启成功"})
	})

	// 一键重建运行环境
	r.POST("/proxy/batch-init", func(c *gin.Context) {
		// 获取所有代理
		proxies := sql.GetProxyList()

		if len(proxies) == 0 {
			c.JSON(400, gin.H{"error": "没有找到代理配置"})
			return
		}

		success := 0
		failed := 0
		var failures []string
		var warnings []string

		proxyManager := proxy.GetProxyManager()

		// 检查依赖 - 可执行文件检查会在启动代理时自动进行
		// 这里不需要提前检查，只在失败时返回错误信息

		for _, proxyConfig := range proxies {
			// 停止代理（如果正在运行）
			if err := proxyManager.StopProxy(proxyConfig.Id); err != nil {
				fmt.Printf("[一键重建] 停止代理 %d 失败: %v\n", proxyConfig.Id, err)
			}

			// 等待一秒
			time.Sleep(1 * time.Second)

			// 重新创建配置文件并启动代理
			if err := proxyManager.RestartProxy(proxyConfig.Id); err != nil {
				failed++
				failures = append(failures, fmt.Sprintf("代理 %d (%s): %v", proxyConfig.Id, proxyConfig.Name, err))
			} else {
				success++
			}
		}

		c.JSON(200, gin.H{
			"total":    len(proxies),
			"success":  success,
			"failed":   failed,
			"details":  failures,
			"warnings": warnings,
		})
	})

	// 测试代理出站连接（使用现有的 /proxy/test/:id 路由）

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
			// Hysteria2的SOCKS5端口：从10808开始分配，避免冲突
			socks5Port = proxy.GetRandomAvailablePortFromRange(10808, 10808+10000)
		}

		// 检查入站端口是否可用
		if !sql.CheckProxyPortAvailable(port, 0) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("端口 %d 已被占用", port),
				"suc": false,
			})
			return
		}

		// 检查SOCKS5端口是否被其他Hysteria2占用
		if outboundType == "hysteria2" && !sql.CheckHy2Socks5PortAvailable(socks5Port, 0) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("SOCKS5端口 %d 已被其他Hysteria2代理占用", socks5Port),
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

		c.Redirect(http.StatusFound, "/proxy")
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

		qualityLogs := sql.GetProxyQualityLogs(id, 20)
		latestQuality, hasQuality := sql.GetLatestProxyQualityLog(id)

		c.HTML(http.StatusOK, "proxy_edit.tmpl", gin.H{
			"proxy":           proxyConfig,
			"realityDomains":  xray.GetRealityDomainList(),
			"version":         version.Version,
			"qualityLogs":     qualityLogs,
			"qualityEnabled":  conf.QualityMonitor.Enabled,
			"lastQuality":     latestQuality,
			"qualityHasValue": hasQuality,
			"qualityConfig":   conf.QualityMonitor,
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

		// Hysteria2自动分配SOCKS5端口
		if outboundType == "hysteria2" && socks5Port == 0 {
			socks5Port = proxy.GetRandomAvailablePortFromRange(10808, 10808+10000)
		}

		// 检查入站端口是否可用
		if port != existing.InboundPort && !sql.CheckProxyPortAvailable(port, id) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("端口 %d 已被占用", port),
				"suc": false,
			})
			return
		}

		// 检查SOCKS5端口是否被其他Hysteria2占用
		if outboundType == "hysteria2" && socks5Port != existing.Hy2Socks5Port && !sql.CheckHy2Socks5PortAvailable(socks5Port, id) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("SOCKS5端口 %d 已被其他Hysteria2代理占用", socks5Port),
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

		c.Redirect(http.StatusFound, "/proxy")
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

	// 查询线路质量监控配置
	r.GET("/proxy/quality-config", func(c *gin.Context) {
		cfg := conf.QualityMonitor
		c.JSON(200, gin.H{
			"success":          true,
			"config":           cfg,
			"proxy_spec":       conf.FormatProxyIDs(cfg.ProxyIDs),
			"interval_seconds": int(cfg.Interval / time.Second),
			"running":          quality.IsRunning(),
		})
	})

	// 更新线路质量监控配置
	r.POST("/proxy/quality-config", func(c *gin.Context) {
		cfg, err := parseQualityMonitorConfigFromRequest(c)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		if err := sql.SaveQualityMonitorSetting(cfg); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("保存配置失败: %v", err)})
			return
		}

		quality.UpdateGlobalMonitorConfig(cfg)
		c.JSON(200, gin.H{
			"success": true,
			"running": quality.IsRunning(),
			"config":  cfg,
		})
	})

	// 获取线路质量数据
	r.GET("/proxy/quality/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		proxyConfig := sql.GetProxy(id)
		if proxyConfig.Id == 0 {
			c.JSON(404, gin.H{"error": "代理配置不存在"})
			return
		}

		sampleLimit := 20
		if limitParam := c.DefaultQuery("limit", "20"); limitParam != "" {
			if value, err := strconv.Atoi(limitParam); err == nil && value > 0 && value <= 500 {
				sampleLimit = value
			}
		}
		startTime := parseTimeQuery(c.DefaultQuery("start", ""))
		endTime := parseTimeQuery(c.DefaultQuery("end", ""))

		resolution := c.DefaultQuery("resolution", "minute")

		// 如果没有提供start和end时间，根据resolution和limit自动计算时间范围
		if startTime.IsZero() && endTime.IsZero() {
			now := time.Now()
			switch strings.ToLower(resolution) {
			case "minute":
				// 分钟级：最近N分钟
				endTime = now
				startTime = now.Add(-time.Duration(sampleLimit) * time.Minute)
			case "hour", "hourly":
				// 小时级：最近N小时
				endTime = now
				startTime = now.Add(-time.Duration(sampleLimit) * time.Hour)
			case "day", "daily":
				// 天级：最近N天
				endTime = now
				startTime = now.AddDate(0, 0, -sampleLimit)
			default:
				// 默认为分钟级
				endTime = now
				startTime = now.Add(-time.Duration(sampleLimit) * time.Minute)
			}
		}

		qualitySamples := sql.GetQualitySamples(id, startTime, endTime, sampleLimit)
		trafficSamples := sql.GetTrafficSamplesWithResolution(id, resolution, startTime, endTime, sampleLimit)
		targetSamples := sql.GetRecentTargetSamples(id, 20)

		limit := 20
		if limitParam := c.DefaultQuery("limit", "20"); limitParam != "" {
			if value, err := strconv.Atoi(limitParam); err == nil && value > 0 && value <= 200 {
				limit = value
			}
		}

		logs := sql.GetProxyQualityLogs(id, limit)
		latest, ok := sql.GetLatestProxyQualityLog(id)

		c.JSON(200, gin.H{
			"enabled":     conf.QualityMonitor.Enabled,
			"logs":        logs,
			"latest":      latest,
			"has_latest":  ok,
			"proxy":       proxyConfig,
			"monitor_cfg": conf.QualityMonitor,
			"resolution":  resolution,
			"samples": gin.H{
				"quality": qualitySamples,
				"traffic": trafficSamples,
				"targets": targetSamples,
			},
		})
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
			// Hysteria2通过本地SOCKS5代理测试出站
			result = proxy.TestHysteria2Connection("127.0.0.1", proxyConfig.Hy2Socks5Port)
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

func parseQualityMonitorConfigFromRequest(c *gin.Context) (conf.QualityMonitorConfig, error) {
	enabled := c.PostForm("enabled") == "1" || c.PostForm("enabled") == "on"
	proxySpec := c.PostForm("proxy_ids")
	target := strings.TrimSpace(c.PostForm("target"))

	intervalSec := parseIntDefault(c.PostForm("interval_seconds"), int(conf.DefaultQualityMonitorConfig.Interval/time.Second))
	probeCount := parseIntDefault(c.PostForm("probe_count"), conf.DefaultQualityMonitorConfig.ProbeCount)
	maxConcurrent := parseIntDefault(c.PostForm("max_concurrent"), conf.DefaultQualityMonitorConfig.MaxConcurrent)
	warnLatency := parseIntDefault(c.PostForm("warn_latency"), conf.DefaultQualityMonitorConfig.WarnLatencyMs)
	warnFailures := parseIntDefault(c.PostForm("warn_failures"), conf.DefaultQualityMonitorConfig.WarnConsecutiveFailure)
	retentionDays := parseIntDefault(c.PostForm("retention_days"), conf.DefaultQualityMonitorConfig.RetentionDays)

	warnLoss := conf.DefaultQualityMonitorConfig.WarnLossPercent
	if value := strings.TrimSpace(c.PostForm("warn_loss")); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			warnLoss = parsed
		}
	}

	if intervalSec < 15 {
		intervalSec = 15
	}
	if probeCount <= 0 {
		probeCount = conf.DefaultQualityMonitorConfig.ProbeCount
	}
	if maxConcurrent <= 0 {
		maxConcurrent = conf.DefaultQualityMonitorConfig.MaxConcurrent
	}
	if warnLatency <= 0 {
		warnLatency = conf.DefaultQualityMonitorConfig.WarnLatencyMs
	}
	if warnFailures <= 0 {
		warnFailures = conf.DefaultQualityMonitorConfig.WarnConsecutiveFailure
	}
	if retentionDays < 0 {
		retentionDays = conf.DefaultQualityMonitorConfig.RetentionDays
	}
	if target == "" {
		target = conf.DefaultQualityMonitorConfig.TestTarget
	}

	cfg := conf.QualityMonitorConfig{
		Enabled:                enabled,
		ProxyIDs:               conf.ParseProxyIDSpec(proxySpec),
		TestTarget:             target,
		Interval:               time.Duration(intervalSec) * time.Second,
		ProbeCount:             probeCount,
		MaxConcurrent:          maxConcurrent,
		WarnLatencyMs:          warnLatency,
		WarnLossPercent:        warnLoss,
		WarnConsecutiveFailure: warnFailures,
		RetentionDays:          retentionDays,
	}

	return cfg, nil
}

func parseIntDefault(value string, defaultValue int) int {
	val := strings.TrimSpace(value)
	if val == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(val); err == nil {
		return parsed
	}
	return defaultValue
}

func parseTimeQuery(val string) time.Time {
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02 15:04:05", val)
	if err == nil {
		return t
	}
	if ts, err := strconv.ParseInt(val, 10, 64); err == nil {
		return time.Unix(ts, 0)
	}
	return time.Time{}
}
