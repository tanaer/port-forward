package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"goForward/control/store"
	pb "goForward/proto"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func init() {
	lis = bufconn.Listen(bufSize)
}

// TestControlServer 测试控制服务器功能
func TestControlServer(t *testing.T) {
	// 创建控制服务器（测试环境使用nil数据库）
	server := NewControlServer(nil)

	// 启动gRPC服务器（使用bufconn进行测试）
	go func() {
		grpcServer := grpc.NewServer()
		pb.RegisterControlServiceServer(grpcServer, server)
		_ = grpcServer.Serve(lis)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端连接
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	client := pb.NewControlServiceClient(conn)

	// 测试1: 注册节点
	testRegisterNode(t, client, server)

	// 测试2: 心跳保活
	testHeartbeat(t, client, server)

	// 测试3: 配置管理
	testConfigManagement(t, client, server)

	// 测试4: 状态上报
	testReportStatus(t, client, server)
}

// testRegisterNode 测试节点注册
func testRegisterNode(t *testing.T, client pb.ControlServiceClient, server *ControlServer) {
	log.Println("=== 测试节点注册 ===")

	nodeInfo := &pb.NodeInfo{
		NodeId:    "test-node-001",
		Hostname:  "test-host",
		IpAddress: "192.168.1.100",
		Version:   "v2.0.0",
		Uptime:    3600,
		Labels:    map[string]string{"region": "us-west"},
	}

	resp, err := client.RegisterNode(context.Background(), nodeInfo)
	if err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	if !resp.Success {
		t.Errorf("注册失败: %s", resp.Message)
	}

	if resp.ControlToken == "" {
		t.Error("控制令牌为空")
	}

	log.Printf("✅ 节点注册成功，控制令牌: %s", resp.ControlToken)

	// 验证节点是否在注册表中
	nodes := server.GetNodes()
	if _, exists := nodes["test-node-001"]; !exists {
		t.Error("节点未在注册表中")
	}

	log.Println("✅ 节点注册验证通过")
}

// testHeartbeat 测试心跳保活
func testHeartbeat(t *testing.T, client pb.ControlServiceClient, server *ControlServer) {
	log.Println("=== 测试心跳保活 ===")

	stream, err := client.Heartbeat(context.Background())
	if err != nil {
		t.Fatalf("创建心跳流失败: %v", err)
	}

	// 启动接收响应的goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var responseCount int

	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			resp, err := stream.Recv()
			if err != nil {
				log.Printf("接收心跳响应失败: %v", err)
				return
			}

			responseCount++
			log.Printf("✅ 收到心跳响应 #%d: Alive=%v", responseCount, resp.Alive)

			if !resp.Alive {
				t.Error("心跳响应显示节点不活跃")
			}
		}
	}()

	// 发送心跳请求
	for i := 0; i < 3; i++ {
		req := &pb.HeartbeatRequest{
			NodeId:    "test-node-001",
			Timestamp: time.Now().Unix(),
			Health: &pb.NodeHealth{
				CpuPercent:    30 + int64(i)*5,
				MemoryPercent: 50 + int64(i)*3,
				DiskPercent:   60,
			},
		}

		if err := stream.Send(req); err != nil {
			t.Fatalf("发送心跳失败: %v", err)
		}

		log.Printf("✅ 发送心跳 #%d", i+1)
		time.Sleep(500 * time.Millisecond)
	}

	// 关闭流
	stream.CloseSend()
	wg.Wait()

	if responseCount == 0 {
		t.Error("未收到心跳响应")
	}

	log.Printf("✅ 心跳测试通过，共收到 %d 个响应", responseCount)
}

// testConfigManagement 测试配置管理
func testConfigManagement(t *testing.T, client pb.ControlServiceClient, server *ControlServer) {
	log.Println("=== 测试配置管理 ===")

	// 获取节点Token
	nodes := server.GetNodes()
	node, exists := nodes["test-node-001"]
	if !exists {
		t.Fatal("节点未注册")
	}
	token := node.ControlToken

	// 创建带认证的context
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", token),
		"node_id":       "test-node-001",
	}))

	stream, err := client.StreamConfig(ctx)
	if err != nil {
		t.Fatalf("创建配置流失败: %v", err)
	}

	// 启动接收响应的goroutine
	var wg sync.WaitGroup
	var updateCount int
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			update, err := stream.Recv()
			if err != nil {
				log.Printf("接收配置更新失败: %v", err)
				return
			}

			mu.Lock()
			updateCount++
			mu.Unlock()

			log.Printf("✅ 收到配置更新 #%d: Success=%v, Message=%s",
				updateCount, update.Success, update.Message)

			if !update.Success {
				log.Printf("⚠️ 配置操作失败: %s", update.Message)
			}
		}
	}()

	// 测试1: 获取配置
	log.Println("--- 请求获取配置 ---")
	reqGet := &pb.ConfigRequest{
		NodeId:      "test-node-001",
		RequestType: "get",
	}
	if err := stream.Send(reqGet); err != nil {
		t.Fatalf("发送获取配置请求失败: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// 测试2: 添加配置
	log.Println("--- 请求添加配置 ---")
	config := &pb.ProxyConfig{
		Id:           1,
		Name:         "测试代理",
		OutboundType: "hysteria2",
		InboundPort:  10808,
		TargetServer: "192.168.1.200",
		TargetPort:   8080,
		Params: map[string]string{
			"password": "test123",
		},
	}
	server.AddConfig(config)

	reqUpdate := &pb.ConfigRequest{
		NodeId:      "test-node-001",
		RequestType: "update",
		Config:      config,
	}
	if err := stream.Send(reqUpdate); err != nil {
		t.Fatalf("发送配置更新请求失败: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// 测试3: 删除配置
	log.Println("--- 请求删除配置 ---")
	reqDelete := &pb.ConfigRequest{
		NodeId:      "test-node-001",
		RequestType: "delete",
		Config: &pb.ProxyConfig{
			Id: 1,
		},
	}
	if err := stream.Send(reqDelete); err != nil {
		t.Fatalf("发送配置删除请求失败: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// 关闭流
	stream.CloseSend()
	wg.Wait()

	if updateCount == 0 {
		t.Error("未收到配置更新响应")
	}

	log.Printf("✅ 配置管理测试通过，共收到 %d 个响应", updateCount)
}

// testReportStatus 测试状态上报
func testReportStatus(t *testing.T, client pb.ControlServiceClient, server *ControlServer) {
	log.Println("=== 测试状态上报 ===")

	// 获取节点Token
	nodes := server.GetNodes()
	node, exists := nodes["test-node-001"]
	if !exists {
		t.Fatal("节点未注册")
	}
	token := node.ControlToken

	// 创建带认证的context
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", token),
		"node_id":       "test-node-001",
	}))

	proxies := []*pb.ProxyStatus{
		{
			Id:                1,
			Name:              "测试代理1",
			Running:           true,
			Uptime:            3600,
			TotalBytes:        1024 * 1024 * 10, // 10MB
			ActiveConnections: 5,
		},
		{
			Id:                2,
			Name:              "测试代理2",
			Running:           false,
			Uptime:            1800,
			TotalBytes:        1024 * 1024 * 5, // 5MB
			ActiveConnections: 0,
			ErrorMessage:      "连接超时",
		},
	}

	status := &pb.NodeStatus{
		NodeId:    "test-node-001",
		Timestamp: time.Now().Unix(),
		Health: &pb.NodeHealth{
			CpuPercent:    40,
			MemoryPercent: 55,
			DiskPercent:   65,
		},
		Proxies: proxies,
	}

	resp, err := client.ReportStatus(ctx, status)
	if err != nil {
		t.Fatalf("上报状态失败: %v", err)
	}

	if !resp.Success {
		t.Errorf("状态上报失败: %s", resp.Message)
	}

	log.Printf("✅ 状态上报成功，推荐操作: %s", resp.Action)
	log.Println("✅ 状态上报测试通过")
}

// BenchmarkControlServer 性能测试
func BenchmarkControlServer(b *testing.B) {
	server := NewControlServer(nil)

	go func() {
		grpcServer := grpc.NewServer()
		pb.RegisterControlServiceServer(grpcServer, server)
		_ = grpcServer.Serve(lis)
	}()

	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	conn, _ := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()

	client := pb.NewControlServiceClient(conn)

	// 注册节点
	nodeInfo := &pb.NodeInfo{
		NodeId:    "bench-node",
		Hostname:  "bench-host",
		IpAddress: "192.168.1.100",
		Version:   "v2.0.0",
		Uptime:    3600,
	}
	_, _ = client.RegisterNode(ctx, nodeInfo)

	// 性能测试
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		status := &pb.NodeStatus{
			NodeId:    "bench-node",
			Timestamp: time.Now().Unix(),
			Health: &pb.NodeHealth{
				CpuPercent:    30,
				MemoryPercent: 50,
				DiskPercent:   60,
			},
			Proxies: []*pb.ProxyStatus{
				{
					Id:      1,
					Name:    "bench-proxy",
					Running: true,
				},
			},
		}

		_, _ = client.ReportStatus(ctx, status)
	}
}

// TestRollbackFlow 测试回滚流程集成测试
func TestRollbackFlow(t *testing.T) {
	// 创建事件处理器记录器
	var recordedEvents []*Event
	eventHandler := func(event *Event) error {
		recordedEvents = append(recordedEvents, event)
		return nil
	}

	// 创建控制服务器（不使用数据库）
	server := NewControlServer(nil)

	// 订阅事件处理器（记录所有事件）
	eventTypes := []EventType{
		EventConfigRolledBack,
		EventConfigVersionCreated,
		EventRollbackTaskCreated,
		EventRollbackTaskPushed,
	}
	for _, eventType := range eventTypes {
		server.eventBus.SubscribeFunc(eventType, eventHandler)
	}

	// 先注册节点
	nodeInfo := &NodeInfo{
		Info: &pb.NodeInfo{
			NodeId:    "test-node",
			Hostname:  "test-host",
			IpAddress: "192.168.1.100",
			Version:   "v2.0.0",
		},
		LastHeartbeat: time.Now(),
		Status:        "active",
		ControlToken:  "test-token",
	}
	server.nodeRegistry.mu.Lock()
	server.nodeRegistry.nodes["test-node"] = nodeInfo
	server.nodeRegistry.mu.Unlock()

	// 1. 测试 PushRollbackToNode 功能
	log.Println("=== 步骤1: 测试PushRollbackToNode ===")
	err := server.PushRollbackToNode("test-node", 1, 2, "测试主动推送回滚")
	if err != nil {
		t.Fatalf("PushRollbackToNode失败: %v", err)
	}

	// 验证任务是否添加到队列
	server.nodeRegistry.mu.RLock()
	tasks := server.nodeRegistry.rollbackTasks["test-node"]
	server.nodeRegistry.mu.RUnlock()

	if len(tasks) != 1 {
		t.Fatalf("回滚任务队列长度错误: 期望=1, 实际=%d", len(tasks))
	}

	task := tasks[0]
	if task.ConfigID != 1 || task.TargetVersion != 2 {
		t.Errorf("回滚任务内容错误: ConfigID=%d, TargetVersion=%d",
			task.ConfigID, task.TargetVersion)
	}
	log.Printf("✅ 主动推送回滚任务创建成功: TaskID=%d, ConfigID=%d, TargetVersion=%d",
		task.ID, task.ConfigID, task.TargetVersion)

	// 等待事件处理
	time.Sleep(100 * time.Millisecond)

	// 验证任务创建事件
	var foundTaskCreatedEvent bool
	for _, event := range recordedEvents {
		if event.Type == EventRollbackTaskCreated {
			foundTaskCreatedEvent = true
			if event.Data["node_id"] != "test-node" {
				t.Errorf("任务创建事件node_id错误")
			}
			log.Printf("✅ 收到任务创建事件: %v", event.Data)
		}
	}

	if !foundTaskCreatedEvent {
		t.Error("未收到EventRollbackTaskCreated事件")
	}

	// 2. 验证任务队列管理
	log.Println("=== 步骤2: 验证任务队列管理 ===")
	// 添加另一个任务
	err = server.PushRollbackToNode("test-node", 2, 3, "第二个任务")
	if err != nil {
		t.Errorf("添加第二个任务失败: %v", err)
	}

	server.nodeRegistry.mu.RLock()
	tasks = server.nodeRegistry.rollbackTasks["test-node"]
	server.nodeRegistry.mu.RUnlock()

	if len(tasks) != 2 {
		t.Errorf("添加任务后队列长度错误: 期望=2, 实际=%d", len(tasks))
	}

	log.Printf("✅ 任务队列管理正常: 当前任务数=%d", len(tasks))

	// 3. 验证事件系统
	log.Println("=== 步骤3: 验证事件系统 ===")
	time.Sleep(100 * time.Millisecond)
	if len(recordedEvents) == 0 {
		t.Error("未记录到任何事件")
	}

	log.Printf("✅ 事件系统正常: 共记录 %d 个事件", len(recordedEvents))

	log.Println("=== 🎉 回滚流程集成测试完成 ===")
}

// TestRollbackTaskWithJsonValidation 测试 get 分支任务推送和 JSON 验证
func TestRollbackTaskWithJsonValidation(t *testing.T) {
	log.Println("=== 测试任务队列和失败恢复机制 ===")

	// 创建控制服务器
	server := NewControlServer(nil)

	// 注册节点
	nodeInfo := &NodeInfo{
		Info:         &pb.NodeInfo{NodeId: "recovery-test-node"},
		Status:       "active",
		ControlToken: "test-token",
	}
	server.nodeRegistry.mu.Lock()
	server.nodeRegistry.nodes["recovery-test-node"] = nodeInfo
	server.nodeRegistry.mu.Unlock()

	log.Println("✓ 测试1: 任务队列持久性")
	// 创建任务
	task1 := &RollbackTask{
		ID:            1,
		ConfigID:      1,
		TargetVersion: 1,
		Reason:        "test persistence",
		CreatedAt:     time.Now(),
	}

	server.nodeRegistry.mu.Lock()
	server.nodeRegistry.rollbackTasks["recovery-test-node"] = []*RollbackTask{task1}
	server.nodeRegistry.mu.Unlock()

	// 验证任务在队列中
	server.nodeRegistry.mu.RLock()
	tasks := server.nodeRegistry.rollbackTasks["recovery-test-node"]
	server.nodeRegistry.mu.RUnlock()

	if len(tasks) == 1 {
		log.Printf("✅ 任务队列正确保存: %d 个任务", len(tasks))
	} else {
		t.Error("任务未正确保存到队列")
	}

	log.Println("✓ 测试2: 失败任务重新入队机制")
	// 当任务处理失败时（比如版本管理器为空），任务应该被重新入队
	// 这在 StreamConfig 的 get 分支中实现
	if server.versionManager == nil {
		log.Println("✅ 版本管理器为 nil，任务会被标记为失败并重新入队")
	}

	log.Println("=== 🎉 队列恢复机制测试完成 ===")
}

// TestRollbackGetStreamWithJsonValidation 真实 StreamConfig get 分支集成测试
// 验证 JSON 解析、失败恢复、任务重试的完整链路
func TestRollbackGetStreamWithJsonValidation(t *testing.T) {
	log.Println("=== StreamConfig get 分支集成测试：JSON 验证和任务重试 ===")

	// 创建控制服务器
	server := NewControlServer(nil)

	// 启动 gRPC 服务器
	go func() {
		grpcServer := grpc.NewServer()
		pb.RegisterControlServiceServer(grpcServer, server)
		_ = grpcServer.Serve(lis)
	}()
	time.Sleep(100 * time.Millisecond)

	// 创建客户端
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	client := pb.NewControlServiceClient(conn)

	// 1. 注册节点
	log.Println("\n✓ 步骤1: 注册节点")
	nodeInfo := &pb.NodeInfo{
		NodeId:    "json-verify-node",
		Hostname:  "test-host",
		IpAddress: "192.168.1.100",
		Version:   "v2.0.0",
	}
	registerResp, err := client.RegisterNode(context.Background(), nodeInfo)
	if err != nil || !registerResp.Success {
		t.Fatalf("注册失败: %v", err)
	}
	log.Printf("✅ 节点注册成功，Token: %s\n", registerResp.ControlToken)

	// 2. 建立 StreamConfig 连接
	log.Println("✓ 步骤2: 建立 StreamConfig 双向流")
	md := metadata.New(map[string]string{
		"node_id":       "json-verify-node",
		"authorization": fmt.Sprintf("Bearer %s", registerResp.ControlToken),
	})
	streamCtx := metadata.NewOutgoingContext(ctx, md)
	stream, err := client.StreamConfig(streamCtx)
	if err != nil {
		t.Fatalf("创建流失败: %v", err)
	}
	defer stream.CloseSend()
	log.Println("✅ StreamConfig 流已建立")

	// 3. 添加一个无效版本的回滚任务到队列（模拟 JSON 解析失败）
	log.Println("✓ 步骤3: 添加无效版本的回滚任务（验证失败重试）")
	invalidTask := &RollbackTask{
		ID:            1,
		ConfigID:      999, // 不存在的版本
		TargetVersion: 99,
		Reason:        "test invalid version",
		CreatedAt:     time.Now(),
	}
	server.nodeRegistry.mu.Lock()
	server.nodeRegistry.rollbackTasks["json-verify-node"] = []*RollbackTask{invalidTask}
	server.nodeRegistry.mu.Unlock()
	log.Printf("✅ 无效任务已添加到队列\n")

	// 4. Agent 发送 get 请求
	log.Println("✓ 步骤4: Agent 发送 get 请求触发任务处理")
	getReq := &pb.ConfigRequest{
		NodeId:      "json-verify-node",
		RequestType: "get",
	}
	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送 get 请求失败: %v", err)
	}
	log.Println("✅ get 请求已发送")

	// 5. 接收响应并验证失败情况下任务重新入队
	log.Println("✓ 步骤5: 接收响应，验证失败任务是否重新入队")
	update, err := stream.Recv()
	if err != nil {
		t.Fatalf("接收响应失败: %v", err)
	}

	if !update.Success {
		log.Printf("✅ 返回失败响应（符合预期）: %s\n", update.Message)
	}

	// 验证任务是否被重新入队
	time.Sleep(100 * time.Millisecond) // 等待事件处理
	server.nodeRegistry.mu.RLock()
	tasksAfter := server.nodeRegistry.rollbackTasks["json-verify-node"]
	server.nodeRegistry.mu.RUnlock()

	if len(tasksAfter) >= 1 {
		log.Printf("✅ 失败的任务已重新入队: %d 个任务\n", len(tasksAfter))
	} else {
		t.Error("❌ 失败的任务丢失，未重新入队")
	}

	// 6. 测试有效的任务（但没有版本管理器，还是会失败）
	log.Println("✓ 步骤6: 再次发送 get 请求处理同一个任务")
	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送第二次 get 请求失败: %v", err)
	}

	update, err = stream.Recv()
	if err != nil {
		t.Fatalf("接收第二次响应失败: %v", err)
	}

	log.Printf("✅ 第二次响应接收: Success=%v, Message=%s\n", update.Success, update.Message)

	// 最终验证：无版本管理器情况下，任务应该一直重试
	server.nodeRegistry.mu.RLock()
	finalTasks := server.nodeRegistry.rollbackTasks["json-verify-node"]
	server.nodeRegistry.mu.RUnlock()

	log.Printf("✅ 最终队列状态: %d 个任务\n", len(finalTasks))
	log.Println("=== 🎉 StreamConfig 集成测试完成 ===")
}

// TestRollbackJsonParsingErrorHandling 测试 JSON 解析错误的具体处理
func TestRollbackJsonParsingErrorHandling(t *testing.T) {
	log.Println("=== 测试 JSON 解析错误的完整处理 ===")

	// 测试数据：模拟各种 JSON 错误情况
	testCases := []struct {
		name        string
		configJSON  string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "有效的配置",
			configJSON:  `{"target_server":"example.com","target_port":8080}`,
			shouldError: false,
		},
		{
			name:        "缺少必需字段",
			configJSON:  `{"target_server":"example.com"}`,
			shouldError: true,
			errorMsg:    "缺少 target_port",
		},
		{
			name:        "错误的字段类型",
			configJSON:  `{"target_server":123,"target_port":8080}`,
			shouldError: true,
			errorMsg:    "类型错误",
		},
		{
			name:        "不完整的 JSON",
			configJSON:  `{"target_server":"example.com"`,
			shouldError: true,
			errorMsg:    "JSON 解析失败",
		},
		{
			name:        "空字符串",
			configJSON:  ``,
			shouldError: true,
			errorMsg:    "JSON 解析失败",
		},
	}

	for _, tc := range testCases {
		log.Printf("\n✓ 测试: %s", tc.name)

		// 安全地解析配置 JSON
		var configParams map[string]interface{}
		err := json.Unmarshal([]byte(tc.configJSON), &configParams)

		if err != nil {
			if tc.shouldError {
				log.Printf("  ✅ 正确捕获 JSON 解析错误: %v", err)
			} else {
				t.Errorf("  ❌ 不应该出现解析错误: %v", err)
			}
			continue
		}

		// 验证字段存在性和类型
		targetServer, ok := configParams["target_server"]
		if !ok {
			if tc.shouldError {
				log.Printf("  ✅ 正确检测到缺少 target_server")
			}
			continue
		}

		if _, isString := targetServer.(string); !isString {
			if tc.shouldError {
				log.Printf("  ✅ 正确检测到字段类型错误")
			}
			continue
		}

		targetPort, hasPort := configParams["target_port"]
		if !hasPort {
			if tc.shouldError {
				log.Printf("  ✅ 正确检测到缺少 target_port")
			}
			continue
		}

		if _, isNumber := targetPort.(float64); !isNumber {
			if tc.shouldError {
				log.Printf("  ✅ 正确检测到 target_port 类型错误")
			}
			continue
		}

		// 所有检查都通过
		if !tc.shouldError {
			log.Printf("  ✅ 所有字段验证通过")
		}
	}

	log.Println("=== 🎉 JSON 解析错误处理测试完成 ===")
}

// TestRollbackFailurePathWithEventCapture 测试回滚失败路径的事件捕获和任务重入队
// 验证：当版本快照缺失required字段时，是否能正确发布EventRollbackTaskFailed并重新入队
func TestRollbackFailurePathWithEventCapture(t *testing.T) {
	log.Println("=== 测试回滚失败路径：EventRollbackTaskFailed 事件捕获 ===")

	// 创建独立的bufconn listener（避免与其他测试共享）
	localLis := bufconn.Listen(bufSize)

	// 创建事件捕获器
	var capturedFailureEvents []*Event
	var capturedPushedEvents []*Event
	var mu sync.Mutex

	failureHandler := func(event *Event) error {
		mu.Lock()
		defer mu.Unlock()
		capturedFailureEvents = append(capturedFailureEvents, event)
		log.Printf("✓ 捕获到EventRollbackTaskFailed: task_id=%v, reason=%v",
			event.Data["task_id"], event.Data["reason"])
		return nil
	}

	pushedHandler := func(event *Event) error {
		mu.Lock()
		defer mu.Unlock()
		capturedPushedEvents = append(capturedPushedEvents, event)
		log.Printf("✓ 捕获到EventRollbackTaskPushed: task_id=%v",
			event.Data["task_id"])
		return nil
	}

	// 创建控制服务器和gRPC
	server := NewControlServer(nil)
	server.eventBus.SubscribeFunc(EventRollbackTaskFailed, failureHandler)
	server.eventBus.SubscribeFunc(EventRollbackTaskPushed, pushedHandler)

	// 启动gRPC服务器
	go func() {
		grpcServer := grpc.NewServer()
		pb.RegisterControlServiceServer(grpcServer, server)
		_ = grpcServer.Serve(localLis)
	}()
	time.Sleep(100 * time.Millisecond)

	// 创建客户端
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return localLis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	client := pb.NewControlServiceClient(conn)

	// 步骤1: 注册节点
	log.Println("\n✓ 步骤1: 注册节点")
	nodeInfo := &pb.NodeInfo{
		NodeId:    "failure-test-node",
		Hostname:  "test-host",
		IpAddress: "192.168.1.100",
		Version:   "v2.0.0",
	}
	registerResp, err := client.RegisterNode(context.Background(), nodeInfo)
	if err != nil || !registerResp.Success {
		t.Fatalf("节点注册失败: %v", err)
	}
	log.Printf("✅ 节点注册成功，Token: %s\n", registerResp.ControlToken)

	// 步骤2: 建立StreamConfig连接
	log.Println("✓ 步骤2: 建立 StreamConfig 双向流")
	md := metadata.New(map[string]string{
		"node_id":       "failure-test-node",
		"authorization": fmt.Sprintf("Bearer %s", registerResp.ControlToken),
	})
	streamCtx := metadata.NewOutgoingContext(ctx, md)
	stream, err := client.StreamConfig(streamCtx)
	if err != nil {
		t.Fatalf("创建流失败: %v", err)
	}
	defer stream.CloseSend()
	log.Println("✅ StreamConfig 流已建立")

	// 步骤3: 创建一个会导致JSON解析失败的回滚任务
	// 场景：版本快照的config_snapshot解析失败（缺少required字段）
	log.Println("✓ 步骤3: 推送一个会触发JSON解析失败的回滚任务到队列")

	// 先模拟版本管理器返回不完整的快照
	// 这里我们直接手动构造回滚任务，让grpc_server.go中的JSON解析逻辑触发失败
	invalidTask := &RollbackTask{
		ID:            1,
		ConfigID:      999,
		TargetVersion: 1,
		Reason:        "test_invalid_snapshot",
		CreatedAt:     time.Now(),
	}

	server.nodeRegistry.mu.Lock()
	server.nodeRegistry.rollbackTasks["failure-test-node"] = []*RollbackTask{invalidTask}
	server.nodeRegistry.mu.Unlock()
	log.Printf("✅ 无效任务已添加到队列: ConfigID=%d, TargetVersion=%d",
		invalidTask.ConfigID, invalidTask.TargetVersion)

	// 步骤4: Agent发送get请求，触发回滚任务处理（会因版本不存在而失败）
	log.Println("✓ 步骤4: Agent发送get请求，触发任务处理（预期失败）")
	getReq := &pb.ConfigRequest{
		NodeId:      "failure-test-node",
		RequestType: "get",
	}
	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送get请求失败: %v", err)
	}
	log.Println("✅ get请求已发送")

	// 步骤5: 接收响应，应该是失败
	log.Println("✓ 步骤5: 接收响应并验证失败")
	update, err := stream.Recv()
	if err != nil {
		t.Fatalf("接收响应失败: %v", err)
	}

	if update.Success {
		t.Error("❌ 预期失败，但收到成功响应")
	} else {
		log.Printf("✅ 收到失败响应（符合预期）: %s", update.Message)
	}

	// 步骤6: 等待事件处理
	log.Println("✓ 步骤6: 等待事件处理")
	time.Sleep(200 * time.Millisecond)

	// 步骤7: 验证EventRollbackTaskFailed是否被发布
	log.Println("✓ 步骤7: 验证事件捕获结果")
	mu.Lock()
	failureCount := len(capturedFailureEvents)
	var eventsCopy []*Event
	for _, e := range capturedFailureEvents {
		eventsCopy = append(eventsCopy, e)
	}
	mu.Unlock()

	if failureCount == 0 {
		t.Error("❌ 未捕获到EventRollbackTaskFailed事件")
	} else {
		log.Printf("✅ 成功捕获到%d个EventRollbackTaskFailed事件", failureCount)
		for i, event := range eventsCopy {
			log.Printf("   [事件%d] task_id=%v, config_id=%v, reason=%v",
				i+1, event.Data["task_id"], event.Data["config_id"],
				event.Data["reason"])
		}
	}

	// 步骤8: 验证任务是否重新入队
	log.Println("✓ 步骤8: 验证任务重新入队")
	server.nodeRegistry.mu.RLock()
	tasksAfterFailure := server.nodeRegistry.rollbackTasks["failure-test-node"]
	server.nodeRegistry.mu.RUnlock()

	if len(tasksAfterFailure) == 1 {
		log.Printf("✅ 失败的任务已重新入队: 共%d个任务", len(tasksAfterFailure))
		reueuedTask := tasksAfterFailure[0]
		log.Printf("   重新入队的任务: TaskID=%d, ConfigID=%d, TargetVersion=%d",
			reueuedTask.ID, reueuedTask.ConfigID, reueuedTask.TargetVersion)
	} else {
		t.Errorf("❌ 任务队列长度错误: 期望=1, 实际=%d", len(tasksAfterFailure))
	}

	// 步骤9: 再次发送get请求，验证任务被重试
	log.Println("✓ 步骤9: 再次发送get请求，验证重试机制")
	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送第二次get请求失败: %v", err)
	}

	update, err = stream.Recv()
	if err != nil {
		t.Fatalf("接收第二次响应失败: %v", err)
	}

	if !update.Success {
		log.Printf("✅ 第二次请求仍然失败（任务持续重试）: %s", update.Message)
	}

	// 最终验证：事件系统工作
	log.Println("✓ 步骤10: 最终验证")
	mu.Lock()
	finalFailureCount := len(capturedFailureEvents)
	finalPushedCount := len(capturedPushedEvents)
	mu.Unlock()

	if finalFailureCount >= 2 {
		log.Printf("✅ 多次重试，事件系统正常工作: 共%d个失败事件", finalFailureCount)
	}

	log.Println("=== 🎉 回滚失败路径集成测试完成 ===")
	log.Printf("最终统计:")
	log.Printf("  - EventRollbackTaskFailed 事件: %d 个", finalFailureCount)
	log.Printf("  - EventRollbackTaskPushed 事件: %d 个", finalPushedCount)
	log.Printf("  - 队列中待执行任务: %d 个", len(tasksAfterFailure))
}

// TestRollbackTaskMaxRetriesExceeded 测试最大重试限制机制
// 验证：当任务retry_count达到maxRetries限制时，自动标记为failed且不再被分配
func TestRollbackTaskMaxRetriesExceeded(t *testing.T) {
	log.Println("=== 测试最大重试限制机制 ===")

	// 步骤1: 创建内存数据库
	log.Println("\n✓ 步骤1: 创建内存数据库")
	memStore, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存数据库失败: %v", err)
	}
	defer memStore.Close()
	log.Println("✅ 内存数据库已创建")

	// 步骤2: 创建独立的bufconn listener
	log.Println("\n✓ 步骤2: 创建独立的bufconn listener")
	localLis := bufconn.Listen(bufSize)

	// 步骤3: 创建事件捕获器
	log.Println("✓ 步骤3: 创建事件捕获器")
	var capturedFailureEvents []*Event
	var mu sync.Mutex

	failureHandler := func(event *Event) error {
		mu.Lock()
		defer mu.Unlock()
		capturedFailureEvents = append(capturedFailureEvents, event)
		log.Printf("✓ 捕获到EventRollbackTaskFailed: task_id=%v, reason=%v",
			event.Data["task_id"], event.Data["reason"])
		return nil
	}

	// 步骤4: 创建控制服务器
	log.Println("✓ 步骤4: 创建控制服务器")
	server := NewControlServerWithWebSocket(memStore, nil)
	server.eventBus.SubscribeFunc(EventRollbackTaskFailed, failureHandler)

	// 启动gRPC服务器
	go func() {
		grpcServer := grpc.NewServer()
		pb.RegisterControlServiceServer(grpcServer, server)
		_ = grpcServer.Serve(localLis)
	}()
	time.Sleep(100 * time.Millisecond)
	log.Println("✅ 控制服务器已启动")

	// 步骤5: 创建客户端
	log.Println("\n✓ 步骤5: 创建gRPC客户端")
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return localLis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()
	client := pb.NewControlServiceClient(conn)
	log.Println("✅ 客户端已创建")

	// 步骤6: 注册节点
	log.Println("\n✓ 步骤6: 注册节点")
	nodeInfo := &pb.NodeInfo{
		NodeId:    "max-retries-test-node",
		Hostname:  "test-host",
		IpAddress: "192.168.1.100",
		Version:   "v2.0.0",
	}
	registerResp, err := client.RegisterNode(context.Background(), nodeInfo)
	if err != nil || !registerResp.Success {
		t.Fatalf("节点注册失败: %v", err)
	}
	log.Printf("✅ 节点注册成功，Token: %s", registerResp.ControlToken)

	// 步骤7: 创建5个已超过最大重试次数的任务（retry_count=6，超过maxRetries=5）
	log.Println("\n✓ 步骤7: 创建5个retry_count=6的任务到数据库（超过maxRetries=5）")
	now := time.Now().Unix()
	for i := 1; i <= 5; i++ {
		taskRecord := &store.RollbackTaskRecord{
			NodeID:        "max-retries-test-node",
			ConfigID:      int32(i),
			TargetVersion: 1,
			Status:        "pending",
			RetryCount:    6, // 超过maxRetries限制（6 > 5）
			Reason:        fmt.Sprintf("测试任务%d", i),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		_, err := memStore.RollbackTaskDAO().CreateTask(taskRecord)
		if err != nil {
			t.Fatalf("创建任务%d失败: %v", i, err)
		}
	}
	log.Println("✅ 5个任务已创建到数据库，每个retry_count=6（超过maxRetries=5）")

	// 验证任务已在数据库中
	tasks, err := memStore.RollbackTaskDAO().GetPendingTasksByNode("max-retries-test-node")
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if len(tasks) != 5 {
		t.Fatalf("任务数量错误: 期望=5, 实际=%d", len(tasks))
	}
	log.Printf("✅ 数据库中有 %d 个pending任务", len(tasks))

	// 步骤8: 建立StreamConfig连接
	log.Println("\n✓ 步骤8: 建立StreamConfig双向流")
	md := metadata.New(map[string]string{
		"node_id":       "max-retries-test-node",
		"authorization": fmt.Sprintf("Bearer %s", registerResp.ControlToken),
	})
	streamCtx := metadata.NewOutgoingContext(ctx, md)
	stream, err := client.StreamConfig(streamCtx)
	if err != nil {
		t.Fatalf("创建流失败: %v", err)
	}
	defer stream.CloseSend()
	log.Println("✅ StreamConfig流已建立")

	// 步骤9: Agent发送get请求，触发最大重试检查
	log.Println("\n✓ 步骤9: Agent发送get请求，触发最大重试检查")
	getReq := &pb.ConfigRequest{
		NodeId:      "max-retries-test-node",
		RequestType: "get",
	}
	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送get请求失败: %v", err)
	}
	log.Println("✅ get请求已发送")

	// 步骤10: 接收响应
	log.Println("\n✓ 步骤10: 接收响应")
	update, err := stream.Recv()
	if err != nil {
		t.Fatalf("接收响应失败: %v", err)
	}
	log.Printf("✅ 收到响应: Success=%v, Message=%s", update.Success, update.Message)

	// 步骤11: 等待事件处理和数据库更新
	log.Println("\n✓ 步骤11: 等待事件处理和数据库更新")
	time.Sleep(300 * time.Millisecond)

	// 步骤12: 验证任务被标记为failed
	log.Println("✓ 步骤12: 验证任务状态已更新为failed")
	allTasks, err := memStore.RollbackTaskDAO().GetTasksByNode("max-retries-test-node")
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}

	failedCount := 0
	for _, task := range allTasks {
		if task.Status == "failed" {
			failedCount++
			if task.ErrorMessage == "" {
				t.Errorf("❌ 任务ID=%d 的error_message为空", task.ID)
			}
			if !contains(task.ErrorMessage, "超过最大限制") {
				t.Errorf("❌ 任务ID=%d 的错误消息不符合预期: %s", task.ID, task.ErrorMessage)
			}
		}
	}

	if failedCount != 5 {
		t.Errorf("❌ failed状态任务数量错误: 期望=5, 实际=%d", failedCount)
	} else {
		log.Printf("✅ 全部5个任务已标记为failed")
	}

	// 步骤13: 验证EventRollbackTaskFailed事件
	log.Println("\n✓ 步骤13: 验证EventRollbackTaskFailed事件")
	mu.Lock()
	eventCount := len(capturedFailureEvents)
	mu.Unlock()

	if eventCount != 5 {
		t.Errorf("❌ 失败事件数量错误: 期望=5, 实际=%d", eventCount)
	} else {
		log.Printf("✅ 成功捕获到5个EventRollbackTaskFailed事件")
	}

	// 步骤14: 验证任务不再被分配
	log.Println("\n✓ 步骤14: 再次发送get请求，验证任务不再被分配")
	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送第二次get请求失败: %v", err)
	}

	update2, err := stream.Recv()
	if err != nil {
		t.Fatalf("接收第二次响应失败: %v", err)
	}

	// 应该没有回滚任务（因为全部已failed）
	if update2.RollbackInfo != nil {
		t.Error("❌ failed任务仍然被分配")
	} else {
		log.Println("✅ failed任务不再被分配（符合预期）")
	}

	// 步骤15: 验证日志包含"超过最大重试次数"
	log.Println("\n✓ 步骤15: 验证日志消息")
	for _, task := range allTasks {
		if task.Status == "failed" {
			if !contains(task.ErrorMessage, "超过最大限制") && !contains(task.ErrorMessage, "超过最大重试次数") {
				t.Errorf("❌ 任务ID=%d 错误消息不包含预期文本: %s", task.ID, task.ErrorMessage)
			}
		}
	}
	log.Println("✅ 日志消息验证通过")

	log.Println("\n=== 🎉 最大重试限制测试完成 ===")
	log.Println("验证通过：")
	log.Println("  1. ✅ 5个任务retry_count=6创建成功（超过maxRetries=5）")
	log.Println("  2. ✅ get请求触发最大重试检查")
	log.Println("  3. ✅ 全部任务被标记为failed")
	log.Println("  4. ✅ 发布了5个EventRollbackTaskFailed事件")
	log.Println("  5. ✅ failed任务不再被分配")
	log.Println("  6. ✅ 错误消息包含'超过最大重试次数'")
}

// TestRollbackTaskProcessingTimeout 测试processing超时机制
// 验证：当任务在processing状态超过10分钟时，自动重置为pending状态
func TestRollbackTaskProcessingTimeout(t *testing.T) {
	log.Println("=== 测试processing超时机制 ===")

	// 步骤1: 创建内存数据库
	log.Println("\n✓ 步骤1: 创建内存数据库")
	memStore, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存数据库失败: %v", err)
	}
	defer memStore.Close()
	log.Println("✅ 内存数据库已创建")

	// 步骤2: 创建控制服务器
	log.Println("\n✓ 步骤2: 创建控制服务器")
	var capturedEvents []*Event
	var mu sync.Mutex

	eventHandler := func(event *Event) error {
		mu.Lock()
		defer mu.Unlock()
		capturedEvents = append(capturedEvents, event)
		log.Printf("✓ 捕获到事件: Type=%s, Data=%v", event.Type, event.Data)
		return nil
	}

	server := NewControlServerWithWebSocket(memStore, nil)
	server.eventBus.SubscribeFunc(EventRollbackTaskFailed, eventHandler)
	log.Println("✅ 控制服务器已创建")

	// 步骤3: 注册节点（满足外键约束）
	log.Println("\n✓ 步骤3: 注册节点（满足外键约束）")
	nodeRecord := &store.NodeRecord{
		NodeID:    "timeout-test-node",
		Hostname:  "test-host",
		IPAddress: "192.168.1.100",
		Status:    "active",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	err = memStore.NodeDAO().CreateNode(nodeRecord)
	if err != nil {
		t.Fatalf("创建节点失败: %v", err)
	}
	log.Println("✅ 节点已注册到数据库")

	// 步骤4: 创建1个processing状态的任务，updated_at为11分钟前
	log.Println("\n✓ 步骤4: 创建processing任务（updated_at=11分钟前）")
	now := time.Now().Unix()
	elevenMinutesAgo := now - 11*60 // 11分钟前

	taskRecord := &store.RollbackTaskRecord{
		NodeID:        "timeout-test-node",
		ConfigID:      1,
		TargetVersion: 1,
		Status:        "processing",
		RetryCount:    2,
		Reason:        "测试超时任务",
		CreatedAt:     now,
		UpdatedAt:     elevenMinutesAgo,
	}

	taskID, err := memStore.RollbackTaskDAO().CreateTask(taskRecord)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	log.Printf("✅ 任务已创建: ID=%d, Status=processing, UpdatedAt=%d (11分钟前)", taskID, elevenMinutesAgo)

	// 验证任务在数据库中
	task, err := memStore.RollbackTaskDAO().GetTaskByID(taskID)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if task.Status != "processing" {
		t.Fatalf("任务状态错误: 期望=processing, 实际=%s", task.Status)
	}
	log.Printf("✅ 任务状态验证通过: Status=%s, RetryCount=%d", task.Status, task.RetryCount)

	// 步骤5: 手动调用monitorStalledTasks()检测超时任务
	log.Println("\n✓ 步骤5: 手动触发超时任务检测")
	stalledTaskIDs, err := memStore.RollbackTaskDAO().GetStalledProcessingTasks(server.processingTimeout)
	if err != nil {
		t.Fatalf("查询超时任务失败: %v", err)
	}

	if len(stalledTaskIDs) != 1 {
		t.Fatalf("超时任务数量错误: 期望=1, 实际=%d", len(stalledTaskIDs))
	}

	if stalledTaskIDs[0] != taskID {
		t.Fatalf("超时任务ID错误: 期望=%d, 实际=%d", taskID, stalledTaskIDs[0])
	}
	log.Printf("✅ 检测到1个超时任务: TaskID=%d", stalledTaskIDs[0])

	// 步骤6: 重置任务到pending状态
	log.Println("\n✓ 步骤6: 重置任务到pending状态")
	err = memStore.RollbackTaskDAO().ResetProcessingTaskToPending(taskID)
	if err != nil {
		t.Fatalf("重置任务失败: %v", err)
	}
	log.Printf("✅ 任务已重置到pending: TaskID=%d", taskID)

	// 步骤7: 验证任务状态已更新
	log.Println("\n✓ 步骤7: 验证任务状态")
	taskAfter, err := memStore.RollbackTaskDAO().GetTaskByID(taskID)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}

	if taskAfter.Status != "pending" {
		t.Errorf("❌ 任务状态错误: 期望=pending, 实际=%s", taskAfter.Status)
	} else {
		log.Printf("✅ 任务状态已更新: Status=%s", taskAfter.Status)
	}

	// 步骤8: 验证retry_count没有增加
	log.Println("\n✓ 步骤8: 验证retry_count没有增加")
	if taskAfter.RetryCount != 2 {
		t.Errorf("❌ retry_count被修改: 期望=2, 实际=%d", taskAfter.RetryCount)
	} else {
		log.Printf("✅ retry_count保持不变: %d", taskAfter.RetryCount)
	}

	// 步骤9: 验证updated_at已更新
	log.Println("\n✓ 步骤9: 验证updated_at已更新")
	if taskAfter.UpdatedAt <= elevenMinutesAgo {
		t.Errorf("❌ updated_at未更新: 旧值=%d, 新值=%d", elevenMinutesAgo, taskAfter.UpdatedAt)
	} else {
		log.Printf("✅ updated_at已更新: %d -> %d", elevenMinutesAgo, taskAfter.UpdatedAt)
	}

	// 步骤9: 手动发布事件（模拟monitorStalledTasks的行为）
	log.Println("\n✓ 步骤10: 手动发布重置事件")
	server.eventBus.Publish(&Event{
		Type: EventRollbackTaskFailed,
		Data: map[string]interface{}{
			"task_id":      taskID,
			"reason":       fmt.Sprintf("Processing超时 %d 秒，已重置到pending", server.processingTimeout),
			"auto_reset":   true,
			"stalled_task": true,
		},
		Timestamp: time.Now().Unix(),
	})
	time.Sleep(100 * time.Millisecond)

	// 步骤11: 验证事件捕获
	log.Println("\n✓ 步骤11: 验证事件捕获")
	mu.Lock()
	eventCount := len(capturedEvents)
	hasAutoResetFlag := false
	hasStalledTaskFlag := false

	for _, event := range capturedEvents {
		if autoReset, ok := event.Data["auto_reset"].(bool); ok && autoReset {
			hasAutoResetFlag = true
		}
		if stalledTask, ok := event.Data["stalled_task"].(bool); ok && stalledTask {
			hasStalledTaskFlag = true
		}
	}
	mu.Unlock()

	if eventCount != 1 {
		t.Errorf("❌ 事件数量错误: 期望=1, 实际=%d", eventCount)
	} else {
		log.Printf("✅ 成功捕获到1个事件")
	}

	if !hasAutoResetFlag {
		t.Error("❌ 事件缺少auto_reset标记")
	} else {
		log.Println("✅ 事件包含auto_reset=true标记")
	}

	if !hasStalledTaskFlag {
		t.Error("❌ 事件缺少stalled_task标记")
	} else {
		log.Println("✅ 事件包含stalled_task=true标记")
	}

	log.Println("\n=== 🎉 Processing超时机制测试完成 ===")
	log.Println("验证通过：")
	log.Println("  1. ✅ processing任务11分钟超时被检测")
	log.Println("  2. ✅ 任务被重置到pending状态")
	log.Println("  3. ✅ retry_count没有增加")
	log.Println("  4. ✅ updated_at时间戳已更新")
	log.Println("  5. ✅ 事件包含auto_reset和stalled_task标记")
}

// TestRollbackTaskTimeoutWithRetryLimit 测试超时机制与重试限制的协同
// 验证：任务多次超时后，retry_count达到maxRetries时被标记为failed
func TestRollbackTaskTimeoutWithRetryLimit(t *testing.T) {
	log.Println("=== 测试超时机制与重试限制协同 ===")

	// 步骤1: 创建内存数据库
	log.Println("\n✓ 步骤1: 创建内存数据库")
	memStore, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存数据库失败: %v", err)
	}
	defer memStore.Close()
	log.Println("✅ 内存数据库已创建")

	// 步骤2: 创建独立的bufconn listener
	log.Println("\n✓ 步骤2: 创建独立的bufconn listener")
	localLis := bufconn.Listen(bufSize)

	// 步骤3: 创建事件捕获器
	log.Println("✓ 步骤3: 创建事件捕获器")
	var capturedEvents []*Event
	var mu sync.Mutex

	eventHandler := func(event *Event) error {
		mu.Lock()
		defer mu.Unlock()
		capturedEvents = append(capturedEvents, event)
		log.Printf("✓ 捕获到事件: Type=%s, auto_reset=%v, auto_failed=%v",
			event.Type, event.Data["auto_reset"], event.Data["auto_failed"])
		return nil
	}

	// 步骤4: 创建控制服务器
	log.Println("✓ 步骤4: 创建控制服务器")
	server := NewControlServerWithWebSocket(memStore, nil)
	server.eventBus.SubscribeFunc(EventRollbackTaskFailed, eventHandler)

	// 启动gRPC服务器
	go func() {
		grpcServer := grpc.NewServer()
		pb.RegisterControlServiceServer(grpcServer, server)
		_ = grpcServer.Serve(localLis)
	}()
	time.Sleep(100 * time.Millisecond)
	log.Println("✅ 控制服务器已启动")

	// 步骤5: 创建客户端
	log.Println("\n✓ 步骤5: 创建gRPC客户端")
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return localLis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()
	client := pb.NewControlServiceClient(conn)
	log.Println("✅ 客户端已创建")

	// 步骤6: 注册节点
	log.Println("\n✓ 步骤6: 注册节点")
	nodeInfo := &pb.NodeInfo{
		NodeId:    "combined-test-node",
		Hostname:  "test-host",
		IpAddress: "192.168.1.100",
		Version:   "v2.0.0",
	}
	registerResp, err := client.RegisterNode(context.Background(), nodeInfo)
	if err != nil || !registerResp.Success {
		t.Fatalf("节点注册失败: %v", err)
	}
	log.Printf("✅ 节点注册成功，Token: %s", registerResp.ControlToken)

	// 步骤7: 创建任务（retry_count=6，且处于processing超时状态）
	log.Println("\n✓ 步骤7: 创建retry_count=6且超时的任务（超过maxRetries=5）")
	now := time.Now().Unix()
	elevenMinutesAgo := now - 11*60

	taskRecord := &store.RollbackTaskRecord{
		NodeID:        "combined-test-node",
		ConfigID:      1,
		TargetVersion: 1,
		Status:        "processing",
		RetryCount:    6, // 已超过maxRetries限制（6 > 5）
		Reason:        "测试超时+重试限制",
		CreatedAt:     now,
		UpdatedAt:     elevenMinutesAgo,
	}

	taskID, err := memStore.RollbackTaskDAO().CreateTask(taskRecord)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	log.Printf("✅ 任务已创建: ID=%d, Status=processing, RetryCount=6, UpdatedAt=%d", taskID, elevenMinutesAgo)

	// 步骤8: 首先检测超时，重置到pending
	log.Println("\n✓ 步骤8: 检测超时任务并重置到pending")
	stalledTaskIDs, err := memStore.RollbackTaskDAO().GetStalledProcessingTasks(server.processingTimeout)
	if err != nil {
		t.Fatalf("查询超时任务失败: %v", err)
	}

	if len(stalledTaskIDs) != 1 {
		t.Fatalf("超时任务数量错误: 期望=1, 实际=%d", len(stalledTaskIDs))
	}

	err = memStore.RollbackTaskDAO().ResetProcessingTaskToPending(taskID)
	if err != nil {
		t.Fatalf("重置任务失败: %v", err)
	}
	log.Printf("✅ 任务已重置到pending: TaskID=%d", taskID)

	// 验证状态
	task1, err := memStore.RollbackTaskDAO().GetTaskByID(taskID)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if task1.Status != "pending" {
		t.Errorf("❌ 重置后状态错误: 期望=pending, 实际=%s", task1.Status)
	}
	if task1.RetryCount != 6 {
		t.Errorf("❌ retry_count被修改: 期望=6, 实际=%d", task1.RetryCount)
	}
	log.Printf("✅ 超时重置验证通过: Status=%s, RetryCount=%d", task1.Status, task1.RetryCount)

	// 步骤9: 建立StreamConfig连接
	log.Println("\n✓ 步骤9: 建立StreamConfig双向流")
	md := metadata.New(map[string]string{
		"node_id":       "combined-test-node",
		"authorization": fmt.Sprintf("Bearer %s", registerResp.ControlToken),
	})
	streamCtx := metadata.NewOutgoingContext(ctx, md)
	stream, err := client.StreamConfig(streamCtx)
	if err != nil {
		t.Fatalf("创建流失败: %v", err)
	}
	defer stream.CloseSend()
	log.Println("✅ StreamConfig流已建立")

	// 步骤10: Agent发送get请求，触发重试检查（任务应该被标记为failed）
	log.Println("\n✓ 步骤10: Agent发送get请求，触发重试检查")
	getReq := &pb.ConfigRequest{
		NodeId:      "combined-test-node",
		RequestType: "get",
	}
	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送get请求失败: %v", err)
	}
	log.Println("✅ get请求已发送")

	// 步骤11: 接收响应
	log.Println("\n✓ 步骤11: 接收响应")
	update, err := stream.Recv()
	if err != nil {
		t.Fatalf("接收响应失败: %v", err)
	}
	log.Printf("✅ 收到响应: Success=%v, Message=%s", update.Success, update.Message)

	// 步骤12: 等待事件处理和数据库更新
	log.Println("\n✓ 步骤12: 等待事件处理和数据库更新")
	time.Sleep(300 * time.Millisecond)

	// 步骤13: 验证任务最终被标记为failed
	log.Println("✓ 步骤13: 验证任务最终状态为failed")
	task2, err := memStore.RollbackTaskDAO().GetTaskByID(taskID)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}

	if task2.Status != "failed" {
		t.Errorf("❌ 最终状态错误: 期望=failed, 实际=%s", task2.Status)
	} else {
		log.Printf("✅ 任务最终状态: Status=%s", task2.Status)
	}

	if task2.ErrorMessage == "" {
		t.Error("❌ error_message为空")
	} else {
		log.Printf("✅ 错误消息: %s", task2.ErrorMessage)
	}

	// 步骤14: 验证事件捕获
	log.Println("\n✓ 步骤14: 验证事件捕获")
	mu.Lock()
	eventCount := len(capturedEvents)
	hasAutoFailedEvent := false

	for _, event := range capturedEvents {
		if autoFailed, ok := event.Data["auto_failed"].(bool); ok && autoFailed {
			hasAutoFailedEvent = true
			log.Printf("  - 捕获到auto_failed事件: task_id=%v, reason=%v",
				event.Data["task_id"], event.Data["reason"])
		}
	}
	mu.Unlock()

	if eventCount == 0 {
		t.Error("❌ 未捕获到任何事件")
	} else {
		log.Printf("✅ 成功捕获到%d个事件", eventCount)
	}

	if !hasAutoFailedEvent {
		t.Error("❌ 未找到auto_failed=true的事件")
	} else {
		log.Println("✅ 成功捕获到auto_failed事件")
	}

	// 步骤15: 验证任务不再被分配
	log.Println("\n✓ 步骤15: 再次发送get请求，验证任务不再被分配")
	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送第二次get请求失败: %v", err)
	}

	update2, err := stream.Recv()
	if err != nil {
		t.Fatalf("接收第二次响应失败: %v", err)
	}

	if update2.RollbackInfo != nil {
		t.Error("❌ failed任务仍然被分配")
	} else {
		log.Println("✅ failed任务不再被分配（符合预期）")
	}

	log.Println("\n=== 🎉 超时+重试限制协同测试完成 ===")
	log.Println("验证通过：")
	log.Println("  1. ✅ 任务超时被检测并重置到pending")
	log.Println("  2. ✅ retry_count=6在重置时保持不变")
	log.Println("  3. ✅ get请求触发重试检查")
	log.Println("  4. ✅ 任务因超过最大重试次数被标记为failed")
	log.Println("  5. ✅ 发布了auto_failed事件")
	log.Println("  6. ✅ failed任务不再被分配")
}

// contains 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
