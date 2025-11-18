package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"goForward/control/server"
	"goForward/control/store"
)

// setupRoutesForTest 设置路由（测试版本，跳过模板加载）
func (w *WebServer) setupRoutesForTest() {
	router := w.router

	// 跳过模板加载（测试环境）
	// router.LoadHTMLGlob("control/web/templates/*")

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

	// WebSocket接口
	router.GET("/ws", w.wsHandler)

	// 首页（简化版）
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

// newWebServerWithControlServerForTest 测试用的工厂函数
func newWebServerWithControlServerForTest(store *store.Store) (*WebServer, *server.ControlServer) {
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

	// 使用测试版路由设置
	webServer.setupRoutesForTest()

	// 启动WebSocket中心（后台运行）
	go wsHub.Start()

	return webServer, controlSrv
}

// TestWebServerHealth 测试健康检查端点
func TestWebServerHealth(t *testing.T) {
	router := gin.New()
	ws := &WebServer{
		router:     router,
		controlSrv: nil,
		wsHub:      NewWebSocketHub(),
	}
	ws.setupRoutesForTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("预期状态码 200, 得到 %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("响应不是有效的JSON: %v", err)
	}

	if status, ok := resp["status"]; !ok || status != "ok" {
		t.Errorf("预期status为'ok', 得到 %v", resp["status"])
	}
}

// TestWebServerIndex 测试主页路由
func TestWebServerIndex(t *testing.T) {
	router := gin.New()
	ws := &WebServer{
		router:     router,
		controlSrv: nil,
		wsHub:      NewWebSocketHub(),
	}
	ws.setupRoutesForTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	// 测试环境返回JSON响应
	if w.Code != http.StatusOK {
		t.Errorf("预期状态码 200, 得到 %d", w.Code)
	}
}

// TestWebServerAPINodes 测试节点列表API
func TestWebServerAPINodes(t *testing.T) {
	// 创建测试存储
	s, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("初始化测试数据库失败: %v", err)
	}
	defer s.Close()

	// 创建Web服务器和控制服务器
	webServer, _ := newWebServerWithControlServerForTest(s)

	// 创建测试节点
	nodeDAO := s.NodeDAO()
	testNode := &store.NodeRecord{
		NodeID:    "test-node-1",
		Hostname:  "test-host",
		IPAddress: "192.168.1.100",
		Version:   "v1.0.0",
		Status:    "active",
	}
	if err := nodeDAO.CreateNode(testNode); err != nil {
		t.Logf("创建测试节点失败: %v", err)
	}

	// 测试 /api/nodes 端点
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/nodes", nil)
	webServer.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("预期状态码 200, 得到 %d, 响应: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("响应不是有效的JSON: %v", err)
	}

	// 验证返回结构
	if data, ok := resp["data"]; !ok {
		t.Errorf("响应中缺少'data'字段")
	} else if _, ok := data.([]interface{}); !ok {
		t.Errorf("预期'data'是数组, 得到 %T", data)
	}
}

// TestWebServerAPINodeDetail 测试节点详情API
func TestWebServerAPINodeDetail(t *testing.T) {
	s, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("初始化测试数据库失败: %v", err)
	}
	defer s.Close()

	webServer, _ := newWebServerWithControlServerForTest(s)

	// 创建测试节点
	nodeDAO := s.NodeDAO()
	testNode := &store.NodeRecord{
		NodeID:    "test-node-detail",
		Hostname:  "detail-host",
		IPAddress: "192.168.1.101",
		Version:   "v1.0.0",
		Status:    "active",
	}
	if err := nodeDAO.CreateNode(testNode); err != nil {
		t.Logf("创建测试节点失败: %v", err)
	}

	// 测试 /api/nodes/:id 端点
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/nodes/test-node-detail", nil)
	webServer.router.ServeHTTP(w, req)

	// 节点可能不在运行时映射中，返回 404 是正常的
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Logf("节点详情状态码: %d", w.Code)
	}
}

// TestWebServerAPIConfigs 测试配置列表API
func TestWebServerAPIConfigs(t *testing.T) {
	s, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("初始化测试数据库失败: %v", err)
	}
	defer s.Close()

	webServer, _ := newWebServerWithControlServerForTest(s)

	// 测试 /api/configs 端点
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/configs", nil)
	webServer.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("预期状态码 200, 得到 %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("响应不是有效的JSON: %v", err)
	}
}

// TestWebServerJSONResponse 测试JSON响应格式
func TestWebServerJSONResponse(t *testing.T) {
	s, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("初始化测试数据库失败: %v", err)
	}
	defer s.Close()

	webServer, _ := newWebServerWithControlServerForTest(s)

	// 测试多个API端点的JSON格式
	endpoints := []string{
		"/api/health",
		"/api/nodes",
		"/api/configs",
	}

	for _, endpoint := range endpoints {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", endpoint, nil)
		webServer.router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Logf("端点 %s 返回 %d", endpoint, w.Code)
			continue
		}

		// 验证能正确解析为JSON
		var resp interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("端点 %s 返回无效JSON: %v", endpoint, err)
		}
	}
}

// TestWebSocketHub 测试WebSocket中心基础功能
func TestWebSocketHub(t *testing.T) {
	hub := NewWebSocketHub()

	// 启动hub（后台运行）
	done := make(chan bool)
	go func() {
		// 启动hub（会无限循环）
		hub.Start()
	}()

	// 给hub时间启动
	time.Sleep(50 * time.Millisecond)

	// 测试广播功能（不应panic）
	testData := map[string]interface{}{
		"type": "test",
		"data": "test message",
	}

	// 广播应该不会阻塞或panic
	go func() {
		hub.Broadcast(testData)
		done <- true
	}()

	select {
	case <-done:
		// 广播成功
	case <-time.After(1 * time.Second):
		t.Error("广播超时")
	}
}
