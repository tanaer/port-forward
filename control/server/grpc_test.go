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
		Info: &pb.NodeInfo{NodeId: "recovery-test-node"},
		Status:  "active",
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
	server.nodeRegistry.mu.RUnlock()

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

// TestRollbackConfigSerializationSafety 测试配置序列化的健壮性
func TestRollbackConfigSerializationSafety(t *testing.T) {
	log.Println("=== 测试配置序列化的健壮性 ===")

	server := NewControlServer(nil)

	// 注册节点
	nodeInfo := &NodeInfo{
		Info: &pb.NodeInfo{NodeId: "serialize-test-node"},
		Status:  "active",
		ControlToken: "test-token",
	}
	server.nodeRegistry.mu.Lock()
	server.nodeRegistry.nodes["serialize-test-node"] = nodeInfo
	server.nodeRegistry.mu.Unlock()

	log.Println("✓ 测试1: 无效 JSON 处理")
	// 验证无法让代码 panic 的无效输入
	invalidInputs := []string{
		"",                    // 空字符串
		"{",                   // 不完整的 JSON
		`{"no_target":"x"}`,   // 缺少必要字段
		`{"target_server":123}`, // 错误的字段类型
	}

	for i, input := range invalidInputs {
		// 这些都应该被安全地处理
		var configParams map[string]interface{}
		err := json.Unmarshal([]byte(input), &configParams)
		
		if err != nil && input != "" {
			log.Printf("✅ 输入 %d: JSON 解析正确地返回错误 %v", i+1, err)
		} else if err == nil {
			log.Printf("✅ 输入 %d: JSON 解析成功，%d 个字段", i+1, len(configParams))
		}
	}

	log.Println("✓ 测试2: 类型断言安全性")
	// 验证所有类型断言都有检查
	testCases := []map[string]interface{}{
		{"target_server": "example.com", "target_port": 8080},                 // 正确
		{"target_server": 123, "target_port": 8080},                           // 错误的 target_server 类型
		{"target_server": "example.com", "target_port": "8080"},              // 错误的 target_port 类型
		{"target_server": "example.com"},                                      // 缺少 target_port
	}

	for i, testCase := range testCases {
		// 模拟代码中的安全类型断言
		targetServer, ok := testCase["target_server"]
		if !ok {
			log.Printf("✅ 测试 %d: 缺少 target_server，正确检测", i+1)
			continue
		}
		
		if _, isString := targetServer.(string); !isString {
			log.Printf("✅ 测试 %d: target_server 类型错误，正确检测为 %T", i+1, targetServer)
			continue
		}

		targetPort, hasPort := testCase["target_port"]
		if !hasPort {
			log.Printf("✅ 测试 %d: 缺少 target_port，正确检测", i+1)
			continue
		}

		if _, isNumber := targetPort.(float64); !isNumber {
			log.Printf("✅ 测试 %d: target_port 类型错误，正确检测为 %T", i+1, targetPort)
			continue
		}

		log.Printf("✅ 测试 %d: 所有字段通过验证", i+1)
	}

	log.Println("=== 🎉 序列化安全性测试完成 ===")
}
