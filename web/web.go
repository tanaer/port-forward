package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"goForward/conf"
	"goForward/logs"
	"goForward/proxy"
	"goForward/sql"
	"goForward/utils"
	"goForward/validator"
	"goForward/version"
	yamlv3 "gopkg.in/yaml.v3"
)

// sanitizeInput 过滤掉字符串中的所有空格和制表符
func sanitizeInput(s *string) {
	if s == nil {
		return
	}
	*s = strings.ReplaceAll(strings.ReplaceAll(*s, " ", ""), "\t", "")
}

func Run() {
	// [GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
	//  - using env:   export GIN_MODE=release
	//  - using code:  gin.SetMode(gin.ReleaseMode)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 添加自定义中间件
	r.Use(ErrorHandler())
	r.Use(RecoverHandler())

	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("goForward", store))
	r.Use(checkCookieMiddleware)

	funcMap := template.FuncMap{
		"toJSON": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	}

	// 从文件系统读取模板（移除embed依赖）
	templates := template.New("")
	tmpl := template.Must(templates.Funcs(funcMap).ParseGlob("assets/templates/*.tmpl"))
	r.SetHTMLTemplate(tmpl)

	// 注册代理管理路由
	RegisterProxyRoutes(r)

	// 注册出站配置路由
	RegisterOutboundRoutes(r)

	// 注册安装器路由
	RegisterInstallerRoutes(r)

	// 注册远程安装路由
	RegisterRemoteInstallRoutes(r)

	// API 批量操作路由
	api := r.Group("/api")
	{
		// 批量启动/停止转发
		api.POST("/batch/start", batchStartHandler)
		api.POST("/batch/stop", batchStopHandler)
		api.POST("/batch/delete", batchDeleteHandler)
		api.POST("/batch/update", batchUpdateHandler)

		// 流量统计 API
		api.GET("/stats/traffic", getTrafficStatsHandler)
		api.GET("/stats/connections", getConnectionStatsHandler)
		api.GET("/stats/system", getSystemStatsHandler)

		// 代理管理 API
		api.GET("/proxy/list", getProxyListAPI)
		api.POST("/proxy/add", addProxyAPI)
		api.PUT("/proxy/update/:id", updateProxyAPI)
		api.DELETE("/proxy/delete/:id", deleteProxyAPI)
		api.POST("/proxy/start/:id", startProxyAPI)
		api.POST("/proxy/stop/:id", stopProxyAPI)

		// 导入导出 API
		api.GET("/export", exportConfigHandler)
		api.POST("/import", importConfigHandler)

		// WebSocket 实时监控
		api.GET("/ws/traffic", trafficWebSocketHandler)
		api.GET("/ws/connections", connectionsWebSocketHandler)

		// 日志管理 API
		api.GET("/logs/list", getLogsHandler)
		api.GET("/logs/search", searchLogsHandler)
		api.GET("/logs/stats", getLogsStatsHandler)
		api.GET("/logs/export", exportLogsHandler)

		// 性能诊断 API
		api.GET("/diagnosis", getDiagnosisHandler)
	}

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"forwardList": sql.GetForwardList(),
			"version":     version.Version,
		})
	})
	r.GET("/ban", func(c *gin.Context) {
		c.JSON(200, sql.GetIpBan())
	})
	r.POST("/add", func(c *gin.Context) {
		protocols := c.PostFormArray("protocol")
		if c.PostForm("localPort") != "" && c.PostForm("remoteAddr") != "" && c.PostForm("remotePort") != "" && len(protocols) > 0 {
			outTimeStr := c.PostForm("outTime")
			outTimeInt, err := strconv.Atoi(outTimeStr)
			if err != nil {
				outTimeInt = conf.DefaultOutTime
			}
			protocolSet := map[string]struct{}{}
			for _, protocol := range protocols {
				p := strings.ToLower(strings.TrimSpace(protocol))
				if p == "tcp" || p == "udp" {
					protocolSet[p] = struct{}{}
				}
			}
			if len(protocolSet) == 0 {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": "添加失败，协议类型不正确",
					"suc": false,
				})
				return
			}
			var success []string
			var failed []string
			order := []string{"tcp", "udp"}
			for _, proto := range order {
				if _, ok := protocolSet[proto]; !ok {
					continue
				}
				f := conf.ConnectionStats{
					LocalPort:  c.PostForm("localPort"),
					RemotePort: c.PostForm("remotePort"),
					RemoteAddr: c.PostForm("remoteAddr"),
					Whitelist:  c.PostForm("whitelist"),
					Blacklist:  c.PostForm("blacklist"),
					Remark:     c.PostForm("remark"),
					OutTime:    outTimeInt,
					Protocol:   proto,
				}

				// 验证配置
				v := validator.NewConfigValidator()
				if err := v.Validate(&f); err != nil {
					c.HTML(200, "msg.tmpl", gin.H{
						"msg": fmt.Sprintf("配置验证失败: %v", err),
						"suc": false,
					})
					return
				}

				if utils.AddForward(f) {
					success = append(success, strings.ToUpper(proto))
				} else {
					failed = append(failed, strings.ToUpper(proto))
				}
			}
			if len(success) > 0 && len(failed) == 0 {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": fmt.Sprintf("添加成功：%s", strings.Join(success, "、")),
					"suc": true,
				})
			} else if len(success) > 0 {
				sort.Strings(failed)
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": fmt.Sprintf("部分成功：已启用 %s，未能启用 %s，请确认端口是否占用", strings.Join(success, "、"), strings.Join(failed, "、")),
					"suc": false,
				})
			} else {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": "添加失败，端口可能已占用",
					"suc": false,
				})
			}
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "添加失败，表单信息不完整",
				"suc": false,
			})
		}
	})
	r.GET("/edit/:id", func(c *gin.Context) {
		id := c.Param("id")
		intID, err := strconv.Atoi(id)
		if err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "ID错误，无法编辑",
				"suc": false,
			})
			return
		}
		f := sql.GetForward(intID)
		if f.Id == 0 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "未找到该转发",
				"suc": false,
			})
			return
		}
		tcpForward := sql.GetForwardByPortAndProtocol(f.LocalPort, "tcp")
		udpForward := sql.GetForwardByPortAndProtocol(f.LocalPort, "udp")
		c.HTML(200, "edit.tmpl", gin.H{
			"forward":     f,
			"selectedTCP": tcpForward.Id != 0,
			"selectedUDP": udpForward.Id != 0,
		})
	})
	r.POST("/edit/:id", func(c *gin.Context) {
		id := c.Param("id")
		intID, err := strconv.Atoi(id)
		if err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "ID错误，无法保存",
				"suc": false,
			})
			return
		}
		protocols := c.PostFormArray("protocol")
		if len(protocols) == 0 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "保存失败，请至少选择一种协议",
				"suc": false,
			})
			return
		}
		outTimeStr := c.PostForm("outTime")
		outTimeInt, err := strconv.Atoi(outTimeStr)
		if err != nil {
			outTimeInt = 5
		}
		update := conf.ConnectionStats{
			Id:         intID,
			LocalPort:  c.PostForm("localPort"),
			RemotePort: c.PostForm("remotePort"),
			RemoteAddr: c.PostForm("remoteAddr"),
			Whitelist:  c.PostForm("whitelist"),
			Blacklist:  c.PostForm("blacklist"),
			Remark:     c.PostForm("remark"),
			OutTime:    outTimeInt,
		}

		// 验证配置
		v := validator.NewConfigValidator()
		if err := v.Validate(&update); err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": fmt.Sprintf("配置验证失败: %v", err),
				"suc": false,
			})
			return
		}

		if ok, msg := utils.UpdateForwardGroup(update, protocols); ok {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "更新成功",
				"suc": true,
			})
		} else {
			if msg == "" {
				msg = "保存失败"
			}
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": msg,
				"suc": false,
			})
		}
	})
	r.POST("/import", func(c *gin.Context) {
		var payload struct {
			Forwards []utils.ImportDefinition `json:"forwards"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"msg": "导入失败，JSON格式不正确",
				"suc": false,
			})
			return
		}
		summary := utils.ImportForwardDefinitions(payload.Forwards)
		c.JSON(http.StatusOK, gin.H{
			"msg":    summary.Message(),
			"suc":    len(summary.Failed) == 0,
			"detail": summary,
		})
	})
	r.GET("/do/:id", func(c *gin.Context) {
		id := c.Param("id")
		intID, err := strconv.Atoi(id)
		f := sql.GetForward(intID)
		status := false
		if err == nil {
			if f.Status == 0 {
				f.Status = 1
				if len(sql.GetAction()) == 1 {
					c.HTML(200, "msg.tmpl", gin.H{
						"msg": "停止失败，请确保有至少一个转发在运行",
						"suc": false,
					})
					return
				}
			} else {
				f.Status = 0
			}
			status = utils.ExStatus(f)
		}
		if status {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "操作成功",
				"suc": true,
			})
			return
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "操作失败",
				"suc": false,
			})
			return
		}
	})
	r.GET("/del/:id", func(c *gin.Context) {
		id := c.Param("id")
		intID, err := strconv.Atoi(id)
		f := sql.GetForward(intID)
		if err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败,ID错误",
				"suc": false,
			})
			return
		}
		if len(sql.GetForwardList()) == 1 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败，请确保有至少一个转发在运行",
				"suc": false,
			})
			return
		}
		if f.Id != 0 && utils.DelForward(f) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除成功",
				"suc": true,
			})
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败",
				"suc": false,
			})
		}
	})
	r.GET("/pwd", func(c *gin.Context) {
		if conf.WebPass == "" {
			c.Redirect(http.StatusFound, "/")
			return
		}
		if authed, ok := sessions.Default(c).Get("authed").(bool); ok && authed {
			c.Redirect(http.StatusFound, "/")
			return
		}
		c.HTML(200, "pwd.tmpl", nil)
	})
	r.POST("/pwd", func(c *gin.Context) {
		if !sql.IpFree(c.ClientIP()) {
			c.HTML(http.StatusOK, "msg.tmpl", gin.H{
				"msg": "IP is Ban",
				"suc": false,
			})
			return
		}
		password := c.PostForm("p")
		session := sessions.Default(c)
		session.Options(sessions.Options{MaxAge: 864000})
		if password != conf.WebPass {
			ban := conf.IpBan{
				Ip:        c.ClientIP(),
				TimeStamp: time.Now().Unix(),
			}
			sql.AddBan(ban)
			session.Delete("authed")
			session.Save()
			c.HTML(http.StatusOK, "msg.tmpl", gin.H{
				"msg": "密码错误",
				"suc": false,
			})
			return
		}
		session.Set("authed", true)
		session.Save()
		c.Redirect(http.StatusFound, "/")
	})
	fmt.Println("Web管理面板端口:" + conf.WebPort)
	r.Run("0.0.0.0:" + conf.WebPort)
}

// 密码验证中间件
func checkCookieMiddleware(c *gin.Context) {
	currenPath := c.Request.URL.Path

	// 如果没有设置密码，则不进行认证检查
	if conf.WebPass == "" && conf.APIToken == "" {
		c.Next()
		return
	}

	// API 路由特殊处理
	if strings.HasPrefix(currenPath, "/api/") {
		// 检查 API Token
		if conf.APIToken != "" {
			token := c.GetHeader("X-API-Token")
			if token == "" {
				// 尝试从查询参数获取
				token = c.Query("api_token")
			}
			if token == conf.APIToken {
				c.Next()
				return
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Unauthorized: invalid or missing API token",
				})
				c.Abort()
				return
			}
		}
		// 如果没有设置 API Token，则使用 Web 认证
		if conf.WebPass == "" {
			c.Next()
			return
		}
	}

	// 排除密码页面和订阅路由（这些路由需要公开访问）
	if currenPath == "/pwd" || strings.HasPrefix(currenPath, "/sub/") {
		c.Next()
		return
	}

	// Web 页面需要登录认证
	if conf.WebPass != "" {
		session := sessions.Default(c)
		if authed, ok := session.Get("authed").(bool); !ok || !authed {
			c.Redirect(http.StatusFound, "/pwd")
			c.Abort()
			return
		}
	}

	c.Next()
}

// ==================== 批量操作 API ====================

// BatchRequest 批量请求结构
type BatchRequest struct {
	IDs []int `json:"ids" binding:"required"`
}

// BatchResponse 批量响应结构
type BatchResponse struct {
	Success []int          `json:"success"`
	Failed  map[int]string `json:"failed"`
	Message string         `json:"message"`
}

// 批量启动转发
func batchStartHandler(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式: " + err.Error(),
		})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请提供要启动的转发ID列表",
		})
		return
	}

	pm := proxy.GetProxyManager()
	response := BatchResponse{
		Success: []int{},
		Failed:  make(map[int]string),
	}

	for _, id := range req.IDs {
		if err := pm.StartProxy(id); err != nil {
			response.Failed[id] = err.Error()
		} else {
			response.Success = append(response.Success, id)
		}
	}

	if len(response.Failed) == 0 {
		response.Message = fmt.Sprintf("成功启动 %d 个转发", len(response.Success))
	} else if len(response.Success) == 0 {
		response.Message = "所有转发启动失败"
	} else {
		response.Message = fmt.Sprintf("部分成功: %d 个成功, %d 个失败", len(response.Success), len(response.Failed))
	}

	c.JSON(http.StatusOK, response)
}

// 批量停止转发
func batchStopHandler(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式: " + err.Error(),
		})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请提供要停止的转发ID列表",
		})
		return
	}

	pm := proxy.GetProxyManager()
	response := BatchResponse{
		Success: []int{},
		Failed:  make(map[int]string),
	}

	// 检查是否会导致没有活动的代理
	activeProxies := sql.GetActiveProxies()
	if len(req.IDs) >= len(activeProxies) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "不能停止所有代理，请至少保留一个活动的代理",
		})
		return
	}

	for _, id := range req.IDs {
		if err := pm.StopProxy(id); err != nil {
			response.Failed[id] = err.Error()
		} else {
			response.Success = append(response.Success, id)
		}
	}

	if len(response.Failed) == 0 {
		response.Message = fmt.Sprintf("成功停止 %d 个转发", len(response.Success))
	} else if len(response.Success) == 0 {
		response.Message = "所有转发停止失败"
	} else {
		response.Message = fmt.Sprintf("部分成功: %d 个成功, %d 个失败", len(response.Success), len(response.Failed))
	}

	c.JSON(http.StatusOK, response)
}

// 批量删除转发
func batchDeleteHandler(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式: " + err.Error(),
		})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请提供要删除的代理ID列表",
		})
		return
	}

	// 检查是否会删除所有代理
	allProxies := sql.GetProxyList()
	if len(req.IDs) >= len(allProxies) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "不能删除所有代理，请至少保留一个代理",
		})
		return
	}

	pm := proxy.GetProxyManager()
	response := BatchResponse{
		Success: []int{},
		Failed:  make(map[int]string),
	}

	for _, id := range req.IDs {
		if err := pm.DeleteProxy(id); err != nil {
			response.Failed[id] = err.Error()
		} else {
			response.Success = append(response.Success, id)
		}
	}

	if len(response.Failed) == 0 {
		response.Message = fmt.Sprintf("成功删除 %d 个转发", len(response.Success))
	} else if len(response.Success) == 0 {
		response.Message = "所有转发删除失败"
	} else {
		response.Message = fmt.Sprintf("部分成功: %d 个成功, %d 个失败", len(response.Success), len(response.Failed))
	}

	c.JSON(http.StatusOK, response)
}

// 批量更新转发
func batchUpdateHandler(c *gin.Context) {
	var req struct {
		IDs    []int                `json:"ids" binding:"required"`
		Config conf.ConnectionStats `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式: " + err.Error(),
		})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请提供要更新的转发ID列表",
		})
		return
	}

	v := validator.NewConfigValidator()
	if err := v.Validate(&req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "配置验证失败: " + err.Error(),
		})
		return
	}

	response := BatchResponse{
		Success: []int{},
		Failed:  make(map[int]string),
	}

	for _, id := range req.IDs {
		config := req.Config
		config.Id = id

		// 获取现有配置，保持状态和流量数据
		existing := sql.GetForward(id)
		if existing.Id == 0 {
			response.Failed[id] = "转发不存在"
			continue
		}
		config.Status = existing.Status
		config.TotalBytes = existing.TotalBytes
		config.TotalGigabyte = existing.TotalGigabyte

		if ok, msg := utils.UpdateForward(config); ok {
			response.Success = append(response.Success, id)
		} else {
			response.Failed[id] = msg
		}
	}

	if len(response.Failed) == 0 {
		response.Message = fmt.Sprintf("成功更新 %d 个转发", len(response.Success))
	} else if len(response.Success) == 0 {
		response.Message = "所有转发更新失败"
	} else {
		response.Message = fmt.Sprintf("部分成功: %d 个成功, %d 个失败", len(response.Success), len(response.Failed))
	}

	c.JSON(http.StatusOK, response)
}

// ==================== 流量统计 API ====================

// TrafficStats 流量统计结构
type TrafficStats struct {
	ID          int     `json:"id"`
	LocalPort   string  `json:"localPort"`
	RemoteAddr  string  `json:"remoteAddr"`
	Protocol    string  `json:"protocol"`
	TotalBytes  uint64  `json:"totalBytes"`
	TotalGB     float64 `json:"totalGB"`
	Status      int     `json:"status"`
	UpdatedTime string  `json:"updatedTime"`
}

// 获取流量统计
func getTrafficStatsHandler(c *gin.Context) {
	forwards := sql.GetForwardList()
	stats := make([]TrafficStats, 0, len(forwards))

	for _, f := range forwards {
		stats = append(stats, TrafficStats{
			ID:          f.Id,
			LocalPort:   f.LocalPort,
			RemoteAddr:  f.RemoteAddr,
			Protocol:    f.Protocol,
			TotalBytes:  f.TotalBytes,
			TotalGB:     float64(f.TotalGigabyte),
			Status:      f.Status,
			UpdatedTime: time.Now().Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  stats,
		"total": len(stats),
	})
}

// ConnectionStats 连接统计结构
type ConnectionStats struct {
	ID           int     `json:"id"`
	LocalPort    string  `json:"localPort"`
	Protocol     string  `json:"protocol"`
	ActiveConns  int     `json:"activeConnections"`
	TotalTraffic uint64  `json:"totalTraffic"`
	TodayTraffic uint64  `json:"todayTraffic"`
	AvgDuration  float64 `json:"avgDuration"`
}

// 获取连接统计
func getConnectionStatsHandler(c *gin.Context) {
	forwards := sql.GetAction()
	stats := make([]ConnectionStats, 0, len(forwards))

	for _, f := range forwards {
		// 这里可以从 metrics 包获取更详细的连接统计
		// 目前返回基础统计
		stats = append(stats, ConnectionStats{
			ID:           f.Id,
			LocalPort:    f.LocalPort,
			Protocol:     f.Protocol,
			ActiveConns:  0, // TODO: 从 metrics 获取
			TotalTraffic: f.TotalBytes,
			TodayTraffic: 0, // TODO: 计算当日流量
			AvgDuration:  float64(f.OutTime),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  stats,
		"total": len(stats),
	})
}

// SystemStats 系统资源统计
type SystemStats struct {
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryUsage uint64  `json:"memoryUsage"`
	MemoryTotal uint64  `json:"memoryTotal"`
	Goroutines  int     `json:"goroutines"`
	Uptime      int64   `json:"uptime"`
	Forwards    int     `json:"forwards"`
	Proxies     int     `json:"proxies"`
	Timestamp   string  `json:"timestamp"`
}

// 获取系统统计
func getSystemStatsHandler(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 获取转发和代理数量
	forwards := sql.GetForwardList()
	proxies := sql.GetActiveProxies()

	stats := SystemStats{
		CPUUsage:    0.0, // TODO: 实现 CPU 使用率统计
		MemoryUsage: m.Alloc,
		MemoryTotal: m.Sys,
		Goroutines:  runtime.NumGoroutine(),
		Uptime:      time.Now().Unix(), // TODO: 从启动时间计算
		Forwards:    len(forwards),
		Proxies:     len(proxies),
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}

	c.JSON(http.StatusOK, stats)
}

// ==================== 性能诊断 API ====================

// PortStatus 端口状态
type PortStatus struct {
	Port            int  `json:"port"`
	ProxyID         int  `json:"proxy_id"`
	InUse           bool `json:"in_use"`
	MultipleProxies bool `json:"multiple_proxies"`
}

// NetworkTestResult 网络测试结果
type NetworkTestResult struct {
	Addr  string `json:"addr"`
	Port  int    `json:"port"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// DiagnosisResult 诊断结果
type DiagnosisResult struct {
	Ports    []PortStatus        `json:"ports"`
	Proxies  []ProxyAPI          `json:"proxies"`
	Database SystemStats         `json:"database"`
	Network  []NetworkTestResult `json:"network"`
}

// 获取诊断信息
func getDiagnosisHandler(c *gin.Context) {
	// 获取代理列表
	proxies := sql.GetActiveProxies()

	// 获取流量统计
	forwards := sql.GetForwardList()
	allProxies := sql.GetProxyList()

	stats := SystemStats{
		CPUUsage:    0.0,
		MemoryUsage: runtime.MemStats{}.Alloc,
		Goroutines:  runtime.NumGoroutine(),
		Forwards:    len(forwards),
		Proxies:     len(proxies),
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}

	// 模拟网络测试结果
	var network []NetworkTestResult
	for _, p := range proxies {
		if p.OutboundType == "socks5" && p.Socks5Addr != "" {
			// 简化实现：假设SOCKS5服务器可达
			network = append(network, NetworkTestResult{
				Addr:  p.Socks5Addr,
				Port:  p.Socks5Port,
				OK:    true,
				Error: "",
			})
		}
	}

	// 转换代理信息
	var apiProxies []ProxyAPI
	for _, p := range allProxies {
		apiProxies = append(apiProxies, ProxyAPI{
			ID:           p.Id,
			InboundPort:  p.InboundPort,
			OutboundType: p.OutboundType,
			Status:       p.Status,
			TotalTraffic: p.TotalBytes,
			TotalGB:      float64(p.TotalGigabyte),
			ServerAddr:   p.Socks5Addr,
		})
	}

	result := DiagnosisResult{
		Ports:    []PortStatus{}, // TODO: 实现端口检查
		Proxies:  apiProxies,
		Database: stats,
		Network:  network,
	}

	c.JSON(http.StatusOK, result)
}

// ==================== 代理管理 API ====================

// ProxyAPI 代理 API 响应结构
type ProxyAPI struct {
	ID           int     `json:"id"`
	InboundPort  int     `json:"inboundPort"`
	OutboundType string  `json:"outboundType"`
	Status       int     `json:"status"`
	TotalTraffic uint64  `json:"totalTraffic"`
	TotalGB      float64 `json:"totalGB"`
	ServerAddr   string  `json:"serverAddr"`
	CreatedAt    string  `json:"createdAt"`
}

// 获取代理列表
func getProxyListAPI(c *gin.Context) {
	proxies := sql.GetActiveProxies()
	apiList := make([]ProxyAPI, 0, len(proxies))

	for _, p := range proxies {
		// 获取服务器地址
		serverAddr := ""
		if p.OutboundType == "socks5" {
			serverAddr = fmt.Sprintf("%s:%d", p.Socks5Addr, p.Socks5Port)
		} else if p.OutboundType == "vmess" {
			serverAddr = fmt.Sprintf("%s:%d", p.VmessServer, p.VmessPort)
		} else if p.OutboundType == "hysteria2" {
			serverAddr = fmt.Sprintf("%s:%s", p.Hy2Server, p.Hy2Port)
		}

		apiList = append(apiList, ProxyAPI{
			ID:           p.Id,
			InboundPort:  p.InboundPort,
			OutboundType: p.OutboundType,
			Status:       p.Status,
			TotalTraffic: p.TotalBytes,
			TotalGB:      float64(p.TotalGigabyte),
			ServerAddr:   serverAddr,
			CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  apiList,
		"total": len(apiList),
	})
}

// 添加代理
func addProxyAPI(c *gin.Context) {
	var proxyConfig conf.ProxyConfig
	if err := c.ShouldBindJSON(&proxyConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式: " + err.Error(),
		})
		return
	}

	// 过滤输入中的空格和制表符
	sanitizeInput(&proxyConfig.Name)
	sanitizeInput(&proxyConfig.Remark)
	sanitizeInput(&proxyConfig.UUID)
	sanitizeInput(&proxyConfig.RealityDest)
	sanitizeInput(&proxyConfig.RealityServerName)
	sanitizeInput(&proxyConfig.PrivateKey)
	sanitizeInput(&proxyConfig.PublicKey)
	sanitizeInput(&proxyConfig.ShortId)
	sanitizeInput(&proxyConfig.Hy2Server)
	sanitizeInput(&proxyConfig.Hy2Password)
	sanitizeInput(&proxyConfig.Hy2Obfs)
	sanitizeInput(&proxyConfig.Hy2ObfsPassword)
	sanitizeInput(&proxyConfig.Hy2SNI)
	sanitizeInput(&proxyConfig.Socks5Addr)
	sanitizeInput(&proxyConfig.Socks5User)
	sanitizeInput(&proxyConfig.Socks5Password)
	sanitizeInput(&proxyConfig.VmessServer)
	sanitizeInput(&proxyConfig.VmessUUID)
	sanitizeInput(&proxyConfig.VmessServerName)
	sanitizeInput(&proxyConfig.VmessWsPath)
	sanitizeInput(&proxyConfig.VmessWsHost)

	pm := proxy.GetProxyManager()
	if err := pm.CreateProxyFromConfig(proxyConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "创建代理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "代理创建成功",
		"id":      proxyConfig.Id,
	})
}

// 更新代理
func updateProxyAPI(c *gin.Context) {
	id := c.Param("id")
	proxyID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的代理ID",
		})
		return
	}

	var proxyConfig conf.ProxyConfig
	if err := c.ShouldBindJSON(&proxyConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式: " + err.Error(),
		})
		return
	}
	proxyConfig.Id = proxyID

	// 过滤输入中的空格和制表符
	sanitizeInput(&proxyConfig.Name)
	sanitizeInput(&proxyConfig.Remark)
	sanitizeInput(&proxyConfig.UUID)
	sanitizeInput(&proxyConfig.RealityDest)
	sanitizeInput(&proxyConfig.RealityServerName)
	sanitizeInput(&proxyConfig.PrivateKey)
	sanitizeInput(&proxyConfig.PublicKey)
	sanitizeInput(&proxyConfig.ShortId)
	sanitizeInput(&proxyConfig.Hy2Server)
	sanitizeInput(&proxyConfig.Hy2Password)
	sanitizeInput(&proxyConfig.Hy2Obfs)
	sanitizeInput(&proxyConfig.Hy2ObfsPassword)
	sanitizeInput(&proxyConfig.Hy2SNI)
	sanitizeInput(&proxyConfig.Socks5Addr)
	sanitizeInput(&proxyConfig.Socks5User)
	sanitizeInput(&proxyConfig.Socks5Password)
	sanitizeInput(&proxyConfig.VmessServer)
	sanitizeInput(&proxyConfig.VmessUUID)
	sanitizeInput(&proxyConfig.VmessServerName)
	sanitizeInput(&proxyConfig.VmessWsPath)
	sanitizeInput(&proxyConfig.VmessWsHost)

	pm := proxy.GetProxyManager()
	if err := pm.CreateProxyFromConfig(proxyConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "更新代理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "代理更新成功",
	})
}

// 删除代理
func deleteProxyAPI(c *gin.Context) {
	id := c.Param("id")
	proxyID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的代理ID",
		})
		return
	}

	pm := proxy.GetProxyManager()
	if err := pm.DeleteProxy(proxyID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "删除代理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "代理删除成功",
	})
}

// 启动代理
func startProxyAPI(c *gin.Context) {
	id := c.Param("id")
	proxyID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的代理ID",
		})
		return
	}

	pm := proxy.GetProxyManager()
	if err := pm.StartProxy(proxyID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "启动代理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "代理启动成功",
	})
}

// 停止代理
func stopProxyAPI(c *gin.Context) {
	id := c.Param("id")
	proxyID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的代理ID",
		})
		return
	}

	pm := proxy.GetProxyManager()
	if err := pm.StopProxy(proxyID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "停止代理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "代理停止成功",
	})
}

// ==================== 导入导出 API ====================

// ConfigExport 导出配置结构
type ConfigExport struct {
	Forwards   []utils.ImportDefinition `json:"forwards"`
	Proxies    []conf.ProxyConfig       `json:"proxies"`
	ExportedAt string                   `json:"exportedAt"`
	Version    string                   `json:"version"`
}

// 导出配置
func exportConfigHandler(c *gin.Context) {
	format := c.Query("format")
	if format == "" {
		format = "json"
	}

	// 获取转发配置
	forwards := sql.GetForwardList()
	forwardDefs := make([]utils.ImportDefinition, 0, len(forwards))
	for _, f := range forwards {
		forwardDefs = append(forwardDefs, utils.ImportDefinition{
			LocalPort:  f.LocalPort,
			RemotePort: f.RemotePort,
			RemoteAddr: f.RemoteAddr,
			Protocol:   f.Protocol,
			OutTime:    f.OutTime,
			Whitelist:  f.Whitelist,
			Blacklist:  f.Blacklist,
			Remark:     f.Remark,
		})
	}

	// 获取代理配置
	proxies := sql.GetActiveProxies()

	export := ConfigExport{
		Forwards:   forwardDefs,
		Proxies:    proxies,
		ExportedAt: time.Now().Format("2006-01-02 15:04:05"),
		Version:    version.Version,
	}

	filename := fmt.Sprintf("goforward-config-%s.%s",
		time.Now().Format("20060102-150405"), format)

	if format == "yaml" || format == "yml" {
		// 导出为 YAML
		data, err := yamlv3.Marshal(export)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "YAML 序列化失败: " + err.Error(),
			})
			return
		}
		c.Header("Content-Type", "application/x-yaml")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Data(http.StatusOK, "application/x-yaml", data)
	} else {
		// 导出为 JSON
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.JSON(http.StatusOK, export)
	}
}

// ImportRequest 导入请求结构
type ImportRequest struct {
	Format  string `json:"format" binding:"required"`
	Data    string `json:"data" binding:"required"`
	Replace bool   `json:"replace"`
}

// 导入配置
func importConfigHandler(c *gin.Context) {
	var req ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式: " + err.Error(),
		})
		return
	}

	if req.Format != "json" && req.Format != "yaml" && req.Format != "yml" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "不支持的格式，支持 json、yaml、yml",
		})
		return
	}

	var config ConfigExport
	var err error

	// 解析配置数据
	if req.Format == "json" {
		err = json.Unmarshal([]byte(req.Data), &config)
	} else {
		err = yamlv3.Unmarshal([]byte(req.Data), &config)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "配置解析失败: " + err.Error(),
		})
		return
	}

	// 验证配置
	if len(config.Forwards) == 0 && len(config.Proxies) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "配置为空",
		})
		return
	}

	// 如果是替换模式，先清空现有配置
	if req.Replace {
		// TODO: 实现替换模式
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "替换模式尚未实现",
		})
		return
	}

	// 导入转发配置
	forwardSummary := utils.ImportForwardDefinitions(config.Forwards)

	// TODO: 导入代理配置

	c.JSON(http.StatusOK, gin.H{
		"message":  "配置导入完成",
		"forwards": forwardSummary,
		"proxies": gin.H{
			"total":    len(config.Proxies),
			"imported": 0, // TODO: 实现代理导入
		},
		"importedAt": time.Now().Format("2006-01-02 15:04:05"),
	})
}

// ==================== WebSocket 实时监控 ====================

// WebSocket 升级器配置
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域，生产环境应限制
	},
}

// WebSocket 客户端管理
var (
	wsClients     = make(map[*websocket.Conn]bool)
	wsClientsLock sync.Mutex
)

// 流量 WebSocket 处理
func trafficWebSocketHandler(c *gin.Context) {
	// 升级 HTTP 连接为 WebSocket
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "WebSocket 升级失败: " + err.Error(),
		})
		return
	}
	defer conn.Close()

	// 注册客户端
	wsClientsLock.Lock()
	wsClients[conn] = true
	wsClientsLock.Unlock()

	// 定期发送流量数据
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 获取流量统计
			forwards := sql.GetForwardList()
			stats := make([]TrafficStats, 0, len(forwards))

			for _, f := range forwards {
				stats = append(stats, TrafficStats{
					ID:          f.Id,
					LocalPort:   f.LocalPort,
					RemoteAddr:  f.RemoteAddr,
					Protocol:    f.Protocol,
					TotalBytes:  f.TotalBytes,
					TotalGB:     float64(f.TotalGigabyte),
					Status:      f.Status,
					UpdatedTime: time.Now().Format("2006-01-02 15:04:05"),
				})
			}

			// 发送数据到客户端
			if err := conn.WriteJSON(gin.H{
				"type": "traffic_stats",
				"data": stats,
				"time": time.Now().Unix(),
			}); err != nil {
				// 客户端断开连接
				wsClientsLock.Lock()
				delete(wsClients, conn)
				wsClientsLock.Unlock()
				return
			}

		case <-c.Done():
			// 客户端断开连接
			wsClientsLock.Lock()
			delete(wsClients, conn)
			wsClientsLock.Unlock()
			return
		}
	}
}

// 连接 WebSocket 处理
func connectionsWebSocketHandler(c *gin.Context) {
	// 升级 HTTP 连接为 WebSocket
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "WebSocket 升级失败: " + err.Error(),
		})
		return
	}
	defer conn.Close()

	// 定期发送连接统计
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 获取连接统计
			forwards := sql.GetAction()
			stats := make([]ConnectionStats, 0, len(forwards))

			for _, f := range forwards {
				stats = append(stats, ConnectionStats{
					ID:           f.Id,
					LocalPort:    f.LocalPort,
					Protocol:     f.Protocol,
					ActiveConns:  0, // TODO: 从 metrics 获取
					TotalTraffic: f.TotalBytes,
					TodayTraffic: 0, // TODO: 计算当日流量
					AvgDuration:  float64(f.OutTime),
				})
			}

			// 发送数据到客户端
			if err := conn.WriteJSON(gin.H{
				"type": "connection_stats",
				"data": stats,
				"time": time.Now().Unix(),
			}); err != nil {
				// 客户端断开连接
				return
			}

		case <-c.Done():
			// 客户端断开连接
			return
		}
	}
}

// ==================== 日志管理 API ====================

// LogAPI 日志 API 响应结构
type LogAPI struct {
	ID        int    `json:"id"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Module    string `json:"module"`
	Context   string `json:"context"`
	Timestamp string `json:"timestamp"`
	Meta      string `json:"meta"`
}

// 获取日志列表
func getLogsHandler(c *gin.Context) {
	levelStr := c.Query("level")
	module := c.Query("module")
	keyword := c.Query("keyword")
	limitStr := c.Query("limit")

	// 解析级别
	level := logs.DEBUG
	if levelStr == "info" {
		level = logs.INFO
	} else if levelStr == "warn" {
		level = logs.WARN
	} else if levelStr == "error" {
		level = logs.ERROR
	}

	// 解析限制数量
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// 获取日志
	entries := logs.GetLogs(level, module, keyword, limit)

	// 转换为 API 格式
	apiLogs := make([]LogAPI, 0, len(entries))
	levelMap := map[logs.LogLevel]string{
		logs.DEBUG: "debug",
		logs.INFO:  "info",
		logs.WARN:  "warn",
		logs.ERROR: "error",
	}

	for _, entry := range entries {
		apiLogs = append(apiLogs, LogAPI{
			ID:        entry.ID,
			Level:     levelMap[entry.Level],
			Message:   entry.Message,
			Module:    entry.Module,
			Context:   entry.Context,
			Timestamp: entry.Timestamp.Format("2006-01-02 15:04:05"),
			Meta:      entry.Meta,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  apiLogs,
		"total": len(apiLogs),
		"filter": gin.H{
			"level":   levelStr,
			"module":  module,
			"keyword": keyword,
		},
	})
}

// 搜索日志
func searchLogsHandler(c *gin.Context) {
	keyword := c.Query("keyword")
	module := c.Query("module")
	limitStr := c.Query("limit")

	if keyword == "" && module == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请提供搜索关键词或模块名",
		})
		return
	}

	// 解析限制数量
	limit := 200
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// 搜索日志
	entries := logs.GetLogs(logs.DEBUG, module, keyword, limit)

	// 转换格式
	apiLogs := make([]LogAPI, 0, len(entries))
	levelMap := map[logs.LogLevel]string{
		logs.DEBUG: "debug",
		logs.INFO:  "info",
		logs.WARN:  "warn",
		logs.ERROR: "error",
	}

	for _, entry := range entries {
		apiLogs = append(apiLogs, LogAPI{
			ID:        entry.ID,
			Level:     levelMap[entry.Level],
			Message:   entry.Message,
			Module:    entry.Module,
			Context:   entry.Context,
			Timestamp: entry.Timestamp.Format("2006-01-02 15:04:05"),
			Meta:      entry.Meta,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    apiLogs,
		"total":   len(apiLogs),
		"keyword": keyword,
		"module":  module,
	})
}

// 获取日志统计
func getLogsStatsHandler(c *gin.Context) {
	stats := logs.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"data": stats,
	})
}

// 导出日志
func exportLogsHandler(c *gin.Context) {
	levelStr := c.Query("level")
	module := c.Query("module")
	keyword := c.Query("keyword")

	// 解析级别
	level := logs.DEBUG
	if levelStr == "info" {
		level = logs.INFO
	} else if levelStr == "warn" {
		level = logs.WARN
	} else if levelStr == "error" {
		level = logs.ERROR
	}

	// 导出日志
	jsonData := logs.ExportLogs(level, module, keyword)

	filename := fmt.Sprintf("goforward-logs-%s.json",
		time.Now().Format("20060102-150405"))

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "application/json", []byte(jsonData))
}
