package web

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"goForward/control/server"
)

// WebServer Web管理界面服务器
type WebServer struct {
	router     *gin.Engine
	controlSrv *server.ControlServer
	wsHub      *WebSocketHub
}

// NewWebServer 创建新的Web服务器
func NewWebServer(controlSrv *server.ControlServer) *WebServer {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 创建WebSocket中心
	wsHub := NewWebSocketHub()

	// 创建WebServer实例
	webServer := &WebServer{
		router:     router,
		controlSrv: controlSrv,
		wsHub:      wsHub,
	}

	// 设置路由
	webServer.setupRoutes()

	// 启动WebSocket中心（后台运行）
	go wsHub.Start()

	return webServer
}

// NewWebServerWithControlServer 创建Web服务器并集成控制服务器
func NewWebServerWithControlServer() (*WebServer, *server.ControlServer) {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 创建WebSocket中心
	wsHub := NewWebSocketHub()

	// 创建带WebSocket的控制服务器
	controlSrv := server.NewControlServerWithWebSocket(wsHub)

	// 创建WebServer实例
	webServer := &WebServer{
		router:     router,
		controlSrv: controlSrv,
		wsHub:      wsHub,
	}

	// 设置路由
	webServer.setupRoutes()

	// 启动WebSocket中心（后台运行）
	go wsHub.Start()

	return webServer, controlSrv
}

// setupRoutes 设置路由
func (w *WebServer) setupRoutes() {
	// 设置HTML模板
	router := w.router
	router.LoadHTMLGlob("control/web/templates/*")

	// 静态文件服务
	router.Static("/static", "./control/web/static")
	router.StaticFS("/assets", http.Dir("./assets"))

	// 首页
	router.GET("/", w.indexHandler)

	// 节点管理
	router.GET("/nodes", w.nodesHandler)
	router.GET("/nodes/:id", w.nodeDetailHandler)

	// 配置管理
	router.GET("/configs", w.configsHandler)
	router.GET("/configs/:id", w.configDetailHandler)

	// API接口
	router.GET("/api/nodes", w.apiNodesHandler)
	router.GET("/api/nodes/:id", w.apiNodeDetailHandler)
	router.GET("/api/health", w.apiHealthHandler)

	// WebSocket接口
	router.GET("/ws", w.wsHandler)
}

// indexHandler 首页
func (w *WebServer) indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"title":   "goForward 2.0 - 分布式控制台",
		"version": "v2.0.0",
	})
}

// nodesHandler 节点列表页面
func (w *WebServer) nodesHandler(c *gin.Context) {
	nodes := w.controlSrv.GetNodes()

	nodeList := make([]map[string]interface{}, 0, len(nodes))
	for nodeID, node := range nodes {
		nodeList = append(nodeList, map[string]interface{}{
			"id":       nodeID,
			"hostname": node.Info.Hostname,
			"ip":       node.Info.IpAddress,
			"status":   node.Status,
			"lastSeen": node.LastHeartbeat.Format("2006-01-02 15:04:05"),
		})
	}

	c.HTML(http.StatusOK, "nodes.tmpl", gin.H{
		"title": "节点管理 - goForward 2.0",
		"nodes": nodeList,
	})
}

// nodeDetailHandler 节点详情页面
func (w *WebServer) nodeDetailHandler(c *gin.Context) {
	nodeID := c.Param("id")

	node, exists := w.controlSrv.GetNodeStatus(nodeID)
	if !exists {
		c.HTML(http.StatusNotFound, "error.tmpl", gin.H{
			"title":   "错误",
			"message": "节点不存在",
		})
		return
	}

	c.HTML(http.StatusOK, "node_detail.tmpl", gin.H{
		"title":   "节点详情 - " + nodeID,
		"nodeID":  nodeID,
		"node":    node,
	})
}

// configsHandler 配置列表页面
func (w *WebServer) configsHandler(c *gin.Context) {
	configs := w.controlSrv.GetConfigs()

	configList := make([]map[string]interface{}, 0, len(configs))
	for configID, config := range configs {
		configList = append(configList, map[string]interface{}{
			"id":           configID,
			"name":         config.Name,
			"outboundType": config.OutboundType,
			"inboundPort":  config.InboundPort,
		})
	}

	c.HTML(http.StatusOK, "configs.tmpl", gin.H{
		"title":    "配置管理 - goForward 2.0",
		"configs":  configList,
	})
}

// configDetailHandler 配置详情页面
func (w *WebServer) configDetailHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "config_detail.tmpl", gin.H{
		"title": "配置详情 - goForward 2.0",
	})
}

// apiNodesHandler API - 获取节点列表
func (w *WebServer) apiNodesHandler(c *gin.Context) {
	nodes := w.controlSrv.GetNodes()

	nodeList := make([]map[string]interface{}, 0, len(nodes))
	for nodeID, node := range nodes {
		nodeList = append(nodeList, map[string]interface{}{
			"id":          nodeID,
			"hostname":    node.Info.Hostname,
			"ip":          node.Info.IpAddress,
			"status":      node.Status,
			"lastSeen":    node.LastHeartbeat.Unix(),
			"cpuPercent":  node.Health.CpuPercent,
			"memPercent":  node.Health.MemoryPercent,
			"diskPercent": node.Health.DiskPercent,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodeList,
	})
}

// apiNodeDetailHandler API - 获取节点详情
func (w *WebServer) apiNodeDetailHandler(c *gin.Context) {
	nodeID := c.Param("id")

	node, exists := w.controlSrv.GetNodeStatus(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "节点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":          nodeID,
			"info":        node.Info,
			"status":      node.Status,
			"lastSeen":    node.LastHeartbeat.Unix(),
			"health":      node.Health,
			"controlToken": node.ControlToken,
		},
	})
}

// apiHealthHandler API - 健康检查
func (w *WebServer) apiHealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "v2.0.0",
	})
}

// Start 启动Web服务器
func (w *WebServer) Start(address string) error {
	log.Printf("[Web] 启动Web管理界面，监听地址: %s", address)
	if err := w.router.Run(address); err != nil {
		return err
	}
	return nil
}

// Stop 停止Web服务器
func (w *WebServer) Stop() error {
	// Gin没有直接的Stop方法，需要使用自定义的HTTP服务器
	// 这里简化处理，实际生产中应该使用 http.Server
	log.Println("[Web] Web服务器已停止")
	return nil
}

// wsHandler WebSocket处理
func (w *WebServer) wsHandler(c *gin.Context) {
	w.wsHub.ServeWebSocket(c)
}