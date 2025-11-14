package server

import (
	"context"
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
	// 创建控制服务器
	server := NewControlServer()

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
			NodeId: "test-node-001",
			Timestamp: time.Now().Unix(),
			Health: &pb.NodeHealth{
				CpuPercent:     30 + int64(i)*5,
				MemoryPercent:  50 + int64(i)*3,
				DiskPercent:    60,
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
			CpuPercent:     40,
			MemoryPercent:  55,
			DiskPercent:    65,
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
	server := NewControlServer()

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
				CpuPercent:     30,
				MemoryPercent:  50,
				DiskPercent:    60,
			},
			Proxies: []*pb.ProxyStatus{
				{
					Id:    1,
					Name:  "bench-proxy",
					Running: true,
				},
			},
		}

		_, _ = client.ReportStatus(ctx, status)
	}
}