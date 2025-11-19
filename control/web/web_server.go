package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"goForward/control/server"
	"goForward/control/store"
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
func NewWebServerWithControlServer(store *store.Store) (*WebServer, *server.ControlServer) {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 创建WebSocket中心
	wsHub := NewWebSocketHub()

	// 创建带WebSocket的控制服务器
	controlSrv := server.NewControlServerWithWebSocket(store, wsHub)

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
	router.POST("/api/nodes/:id/isolate", w.apiIsolateNodeHandler)
	router.POST("/api/nodes/:id/recover", w.apiRecoverNodeHandler)
	router.GET("/api/nodes/:id/health", w.apiNodeHealthHandler)
	router.GET("/api/nodes/:id/events", w.apiNodeEventsHandler)
	router.POST("/api/nodes/batch/restart", w.apiBatchRestartNodesHandler)
	router.POST("/api/nodes/batch/status", w.apiBatchUpdateNodesStatusHandler)
	router.GET("/api/health", w.apiHealthHandler)

	// 配置API接口
	router.GET("/api/configs", w.apiConfigsHandler)
	router.POST("/api/configs/batch", w.apiBatchConfigsHandler)
	router.DELETE("/api/configs/batch", w.apiBatchDeleteConfigsHandler)
	router.GET("/api/configs/:id/versions", w.apiConfigVersionsHandler)
	router.POST("/api/configs/:id/rollback/:version", w.apiConfigRollbackHandler)

	// 死信队列API接口
	router.GET("/api/dlq/tasks", w.apiDLQListHandler)
	router.GET("/api/dlq/tasks/:id", w.apiDLQDetailHandler)
	router.POST("/api/dlq/tasks/:id/replay", w.apiDLQReplayHandler)
	router.DELETE("/api/dlq/tasks/:id", w.apiDLQDeleteHandler)
	router.POST("/api/dlq/cleanup", w.apiDLQCleanupHandler)

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
		"title":  "节点详情 - " + nodeID,
		"nodeID": nodeID,
		"node":   node,
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
		"title":   "配置管理 - goForward 2.0",
		"configs": configList,
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
			"id":           nodeID,
			"info":         node.Info,
			"status":       node.Status,
			"lastSeen":     node.LastHeartbeat.Unix(),
			"health":       node.Health,
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

// apiConfigsHandler API - 获取配置列表
func (w *WebServer) apiConfigsHandler(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    configList,
	})
}

// apiBatchRestartNodesHandler API - 批量重启节点
func (w *WebServer) apiBatchRestartNodesHandler(c *gin.Context) {
	var req struct {
		NodeIDs []string `json:"node_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证节点ID列表
	if len(req.NodeIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID列表不能为空",
		})
		return
	}

	// 调用控制服务器的批量重启方法
	results := w.controlSrv.BatchRestartNodes(req.NodeIDs)

	// 统计成功和失败的数量
	successCount := 0
	failedCount := 0
	for _, success := range results {
		if success {
			successCount++
		} else {
			failedCount++
		}
	}

	// 如果所有节点都返回false，说明功能未实现
	if failedCount == len(results) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"success":      false,
			"error":        "批量重启功能暂未实现，需 Phase 2 支持",
			"successCount": successCount,
			"failedCount":  failedCount,
			"results":      results,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      fmt.Sprintf("批量重启完成: 成功 %d, 失败 %d", successCount, failedCount),
		"successCount": successCount,
		"failedCount":  failedCount,
		"results":      results,
	})
}

// apiBatchUpdateNodesStatusHandler API - 批量更新节点状态
func (w *WebServer) apiBatchUpdateNodesStatusHandler(c *gin.Context) {
	var req struct {
		NodeIDs []string `json:"node_ids"`
		Status  string   `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证节点ID列表
	if len(req.NodeIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID列表不能为空",
		})
		return
	}

	// 验证状态字段
	if req.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "状态不能为空",
		})
		return
	}

	// 验证状态值
	validStatuses := map[string]bool{
		"active":      true,
		"inactive":    true,
		"maintenance": true,
		"unknown":     true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("无效的状态值 '%s'，必须是 active/inactive/maintenance/unknown", req.Status),
		})
		return
	}

	// 调用控制服务器的批量更新方法
	affected, err := w.controlSrv.BatchUpdateNodesStatus(req.NodeIDs, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "批量更新节点状态失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   fmt.Sprintf("批量更新节点状态完成，影响 %d 个节点", affected),
		"affected":  affected,
		"requested": len(req.NodeIDs),
	})
}

// apiBatchConfigsHandler API - 批量创建/更新配置
func (w *WebServer) apiBatchConfigsHandler(c *gin.Context) {
	var req struct {
		Configs []*store.ProxyConfigRecord `json:"configs"`
		Action  string                     `json:"action"` // "create" or "update"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证配置列表
	if len(req.Configs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "配��列表不能为空",
		})
		return
	}

	// 验证操作类型
	if req.Action != "create" && req.Action != "update" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "操作类型必须是 'create' 或 'update'",
		})
		return
	}

	// 验证每个配置的必需字段
	for i, config := range req.Configs {
		if config.NodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   fmt.Sprintf("配置 %d 缺少 node_id", i),
			})
			return
		}
		if config.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   fmt.Sprintf("配置 %d 缺少 name", i),
			})
			return
		}
		if config.OutboundType == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   fmt.Sprintf("配置 %d 缺少 outbound_type", i),
			})
			return
		}
		if config.ConfigJSON == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   fmt.Sprintf("配置 %d 缺少 config_json", i),
			})
			return
		}
	}

	var affected int64
	var err error

	if req.Action == "create" {
		affected, err = w.controlSrv.BatchCreateConfigs(req.Configs)
	} else {
		affected, err = w.controlSrv.BatchUpdateConfigs(req.Configs)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("批量%s配置失败: %v", req.Action, err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   fmt.Sprintf("批量%s配置完成，影响 %d 个配置", req.Action, affected),
		"affected":  affected,
		"requested": len(req.Configs),
	})
}

// apiBatchDeleteConfigsHandler API - 批量删除配置
func (w *WebServer) apiBatchDeleteConfigsHandler(c *gin.Context) {
	var req struct {
		ConfigIDs []int32 `json:"config_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数错误: " + err.Error(),
		})
		return
	}

	if len(req.ConfigIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "配置ID列表不能为空",
		})
		return
	}

	// 调用控制服务器的批量删除方法
	results := w.controlSrv.BatchDeleteConfig(req.ConfigIDs)

	// 统计成功删除的数量
	affected := 0
	for _, success := range results {
		if success {
			affected++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   fmt.Sprintf("批量删除配置完成，影响 %d 个配置", affected),
		"affected":  affected,
		"requested": len(req.ConfigIDs),
		"results":   results,
	})
}

// apiIsolateNodeHandler 隔离节点API
func (w *WebServer) apiIsolateNodeHandler(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID不能为空",
		})
		return
	}

	// 解析请求体（可选的隔离原因）
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有提供原因，使用默认值
		req.Reason = "手动隔离"
	}

	// 检查节点是否存在
	if _, exists := w.controlSrv.GetNodeStatus(nodeID); !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("节点 %s 不存在", nodeID),
		})
		return
	}

	// 获取生命周期管理器并执行隔离
	lifecycleManager := w.controlSrv.GetLifecycleManager()
	if lifecycleManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "生命周期管理器未初始化",
		})
		return
	}

	// 执行隔离
	if err := lifecycleManager.IsolateNode(nodeID, req.Reason, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[Web API] 节点已隔离: %s, 原因: %s", nodeID, req.Reason)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("节点 %s 已成功隔离", nodeID),
		"node_id": nodeID,
		"reason":  req.Reason,
	})
}

// apiRecoverNodeHandler 恢复节点API
func (w *WebServer) apiRecoverNodeHandler(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID不能为空",
		})
		return
	}

	// 检查节点是否存在
	if _, exists := w.controlSrv.GetNodeStatus(nodeID); !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("节点 %s 不存在", nodeID),
		})
		return
	}

	// 获取生命周期管理器并执行恢复
	lifecycleManager := w.controlSrv.GetLifecycleManager()
	if lifecycleManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "生命周期管理器未初始化",
		})
		return
	}

	// 执行恢复
	if err := lifecycleManager.RecoverNode(nodeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[Web API] 节点已恢复: %s", nodeID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("节点 %s 已成功恢复", nodeID),
		"node_id": nodeID,
	})
}

// apiNodeHealthHandler 获取节点健康状态API
func (w *WebServer) apiNodeHealthHandler(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID不能为空",
		})
		return
	}

	// 检查节点是否存在
	node, exists := w.controlSrv.GetNodeStatus(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("节点 %s 不存在", nodeID),
		})
		return
	}

	// 获取生命周期管理器
	lifecycleManager := w.controlSrv.GetLifecycleManager()
	if lifecycleManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "生命周期管理器未初始化",
		})
		return
	}

	// 获取健康状态
	healthStatus, err := lifecycleManager.GetNodeHealthStatus(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 获取失败计数和离线时间
	failureCount := lifecycleManager.GetFailureCount(nodeID)
	offlineTime, isOffline := lifecycleManager.GetOfflineTime(nodeID)
	isolated := lifecycleManager.IsNodeIsolated(nodeID)

	response := gin.H{
		"success":       true,
		"node_id":       nodeID,
		"health_status": string(healthStatus),
		"status":        node.Status,
		"failure_count": failureCount,
		"isolated":      isolated,
	}

	if isOffline {
		response["offline_since"] = offlineTime.Unix()
		response["offline_duration_seconds"] = int64(node.LastHeartbeat.Sub(offlineTime).Seconds())
	}

	if node.Health != nil {
		response["health_metrics"] = gin.H{
			"cpu_percent":        node.Health.CpuPercent,
			"memory_percent":     node.Health.MemoryPercent,
			"disk_percent":       node.Health.DiskPercent,
			"active_connections": node.Health.ActiveConnections,
		}
	}

	c.JSON(http.StatusOK, response)
}

// apiNodeEventsHandler 获取节点事件历史API
func (w *WebServer) apiNodeEventsHandler(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID不能为空",
		})
		return
	}

	// 获取limit参数（默认50条）
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && parsedLimit == 1 {
			if limit < 1 {
				limit = 10
			} else if limit > 500 {
				limit = 500
			}
		}
	}

	// 检查节点是否存在
	if _, exists := w.controlSrv.GetNodeStatus(nodeID); !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("节点 %s 不存在", nodeID),
		})
		return
	}

	// 从数据库获取事件
	store := w.controlSrv.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "数据存储未初始化",
		})
		return
	}

	events, err := store.NodeEventDAO().GetEventsByNodeID(nodeID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("获取节点事件失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"node_id": nodeID,
		"events":  events,
		"count":   len(events),
	})
}

// apiConfigVersionsHandler 获取配置版本历史
func (w *WebServer) apiConfigVersionsHandler(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "配置ID不能为空",
		})
		return
	}

	// 解析配置ID
	var id int32
	if _, err := fmt.Sscanf(configID, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的配置ID格式",
		})
		return
	}

	// 获取版本管理器
	versionManager := w.controlSrv.GetVersionManager()
	if versionManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "版本管理器未初始化",
		})
		return
	}

	// 获取版本历史
	limit := 20 // 默认返回20条记录
	offset := 0 // 从最新版本开始

	versions, err := versionManager.GetVersionHistory(id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("获取版本历史失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"config_id": id,
		"versions":  versions,
		"count":     len(versions),
	})
}

// apiConfigRollbackHandler 配置回滚
func (w *WebServer) apiConfigRollbackHandler(c *gin.Context) {
	configID := c.Param("id")
	versionStr := c.Param("version")

	if configID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "配置ID不能为空",
		})
		return
	}

	if versionStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "目标版本号不能为空",
		})
		return
	}

	// 解析配置ID和目标版本
	var configIDInt int32
	var targetVersion int32

	if _, err := fmt.Sscanf(configID, "%d", &configIDInt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的配置ID格式",
		})
		return
	}

	if _, err := fmt.Sscanf(versionStr, "%d", &targetVersion); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的版本号格式",
		})
		return
	}

	// 获取版本管理器
	versionManager := w.controlSrv.GetVersionManager()
	if versionManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "版本管理器未初始化",
		})
		return
	}

	// 获取目标版本的配置快照
	record, err := versionManager.GetVersion(configIDInt, targetVersion)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("目标版本不存在: %v", err),
		})
		return
	}

	// 将快照转换为 ProxyConfigRecord
	var configRecord store.ProxyConfigRecord
	if err := json.Unmarshal([]byte(record.ConfigSnapshot), &configRecord); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("解析配置快照失败: %v", err),
		})
		return
	}

	// 更新当前配置（回滚到目标版本）
	configs := []*store.ProxyConfigRecord{&configRecord}
	affected, err := w.controlSrv.BatchUpdateConfigs(configs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("回滚配置失败: %v", err),
		})
		return
	}

	// 发布回滚事件
	if w.controlSrv.GetEventBus() != nil {
		w.controlSrv.GetEventBus().Publish(&server.Event{
			Type:     server.EventConfigRolledBack,
			ConfigID: configIDInt,
			Data: map[string]interface{}{
				"target_version":  targetVersion,
				"current_version": record.Version + 1, // 回滚后的新版本号
				"rollback_by":     "web_ui",
			},
			Timestamp: time.Now().Unix(),
		})
	}

	log.Printf("[Web API] 配置 %d 回滚到版本 %d 完成，影响 %d 个配置",
		configIDInt, targetVersion, affected)

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          "配置回滚成功",
		"config_id":        configIDInt,
		"target_version":   targetVersion,
		"affected_configs": affected,
	})
}

// apiDLQListHandler API - 获取死信队列任务列表
func (w *WebServer) apiDLQListHandler(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit := 100
	if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 1000 {
		limit = val
	}

	store := w.controlSrv.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "存储层未初始化",
		})
		return
	}

	tasks, err := store.DLQDAO().ListDLQTasks(limit)
	if err != nil {
		log.Printf("[Web API] 获取DLQ任务列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("获取DLQ任务列表失败: %v", err),
		})
		return
	}

	taskList := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		metadata := ""
		if task.Metadata.Valid {
			metadata = task.Metadata.String
		}
		taskList = append(taskList, map[string]interface{}{
			"id":                task.ID,
			"original_task_id":  task.OriginalTaskID,
			"node_id":           task.NodeID,
			"config_id":         task.ConfigID,
			"target_version":    task.TargetVersion,
			"status":            task.Status,
			"failure_reason":    task.FailureReason,
			"retry_count":       task.RetryCount,
			"moved_to_dlq_at":   time.Unix(task.MovedToDLQAt, 0).Format("2006-01-02 15:04:05"),
			"dlq_expiry_at":     time.Unix(task.DLQExpiryAt, 0).Format("2006-01-02 15:04:05"),
			"metadata":          metadata,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    taskList,
		"count":   len(taskList),
	})
}

// apiDLQDetailHandler API - 获取死信队列任务详情
func (w *WebServer) apiDLQDetailHandler(c *gin.Context) {
	dlqIDStr := c.Param("id")
	dlqID, err := strconv.ParseInt(dlqIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的DLQ任务ID",
		})
		return
	}

	store := w.controlSrv.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "存储层未初始化",
		})
		return
	}

	task, err := store.DLQDAO().GetDLQTaskByID(dlqID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("DLQ任务不存在: %v", err),
		})
		return
	}

	metadata := ""
	if task.Metadata.Valid {
		metadata = task.Metadata.String
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"id":                task.ID,
			"original_task_id":  task.OriginalTaskID,
			"node_id":           task.NodeID,
			"config_id":         task.ConfigID,
			"target_version":    task.TargetVersion,
			"status":            task.Status,
			"failure_reason":    task.FailureReason,
			"retry_count":       task.RetryCount,
			"moved_to_dlq_at":   time.Unix(task.MovedToDLQAt, 0).Format("2006-01-02 15:04:05"),
			"dlq_expiry_at":     time.Unix(task.DLQExpiryAt, 0).Format("2006-01-02 15:04:05"),
			"metadata":          metadata,
		},
	})
}

// apiDLQReplayHandler API - 从死信队列重放任务
func (w *WebServer) apiDLQReplayHandler(c *gin.Context) {
	dlqIDStr := c.Param("id")
	dlqID, err := strconv.ParseInt(dlqIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的DLQ任务ID",
		})
		return
	}

	store := w.controlSrv.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "存储层未初始化",
		})
		return
	}

	if err := store.DLQDAO().ReplayFromDLQ(dlqID); err != nil {
		log.Printf("[Web API] 从DLQ重放任务 %d 失败: %v", dlqID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("DLQ任务重放失败: %v", err),
		})
		return
	}

	log.Printf("[Web API] DLQ任务 %d 已重放到队列", dlqID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DLQ任务已重放到队列",
		"dlq_id":  dlqID,
	})
}

// apiDLQDeleteHandler API - 从死信队列删除任务
func (w *WebServer) apiDLQDeleteHandler(c *gin.Context) {
	dlqIDStr := c.Param("id")
	dlqID, err := strconv.ParseInt(dlqIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的DLQ任务ID",
		})
		return
	}

	store := w.controlSrv.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "存储层未初始化",
		})
		return
	}

	if err := store.DLQDAO().DeleteFromDLQ(dlqID); err != nil {
		log.Printf("[Web API] 删除DLQ任务 %d 失败: %v", dlqID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("DLQ任务删除失败: %v", err),
		})
		return
	}

	log.Printf("[Web API] DLQ任务 %d 已永久删除", dlqID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DLQ任务已永久删除",
		"dlq_id":  dlqID,
	})
}

// apiDLQCleanupHandler API - 清理过期的DLQ任务
func (w *WebServer) apiDLQCleanupHandler(c *gin.Context) {
	store := w.controlSrv.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "存储层未初始化",
		})
		return
	}

	affected, err := store.DLQDAO().CleanupExpiredDLQTasks()
	if err != nil {
		log.Printf("[Web API] 清理过期DLQ任务失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("清理过期DLQ任务失败: %v", err),
		})
		return
	}

	log.Printf("[Web API] 清理过期DLQ任务: 删除 %d 条", affected)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "过期DLQ任务已清理",
		"deleted": affected,
	})
}

// Run 启动Web服务器
func (w *WebServer) Run(port string) error {
	log.Printf("[Web服务器] 启动Web管理界面: http://localhost:%s", port)
	return w.router.Run(":" + port)
}

// GetRouter 获取Gin路由器
func (w *WebServer) GetRouter() *gin.Engine {
	return w.router
}

// GetControlServer 获取控制服务器
func (w *WebServer) GetControlServer() *server.ControlServer {
	return w.controlSrv
}
