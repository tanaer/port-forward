package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"goForward/control/server"
	"goForward/control/store"
	pb "goForward/proto"
)

const bufSize = 1024 * 1024

// TestAgentRollbackExecution 测试 Agent 端回滚执行流程
func TestAgentRollbackExecution(t *testing.T) {
	log.Println("=== 集成测试：控制端推送 → Agent 执行回滚 ===")

	// 创建内存数据库
	memStore, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存数据库失败: %v", err)
	}
	defer memStore.Close()

	// 创建控制服务器
	controlServer := server.NewControlServerWithWebSocket(memStore, nil)

	// 创建 bufconn 监听器
	lis := bufconn.Listen(bufSize)

	// 启动 gRPC 服务器
	grpcServer := grpc.NewServer()
	pb.RegisterControlServiceServer(grpcServer, controlServer)
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// 创建 Agent 客户端
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("创建 gRPC 连接失败: %v", err)
	}
	defer conn.Close()

	client := pb.NewControlServiceClient(conn)

	// 步骤1: Agent 注册节点
	log.Println("\n✓ 步骤1: Agent 注册节点")
	nodeInfo := &pb.NodeInfo{
		NodeId:    "agent-test-node",
		Hostname:  "test-agent",
		IpAddress: "192.168.1.50",
		Version:   "v2.0.0",
	}

	registerResp, err := client.RegisterNode(context.Background(), nodeInfo)
	if err != nil || !registerResp.Success {
		t.Fatalf("节点注册失败: %v", err)
	}
	log.Printf("✅ 节点注册成功，Token: %s", registerResp.ControlToken)

	// 步骤2: 创建配置版本快照（模拟历史配置）
	log.Println("\n✓ 步骤2: 创建配置版本快照")

	// 先创建配置记录（满足外键约束）
	configV1 := store.ProxyConfigRecord{
		ID:           1, // 手动设置ID
		NodeID:       "agent-test-node",
		Name:         "测试代理",
		OutboundType: "hysteria2",
		InboundPort:  10808,
		ConfigJSON:   `{"target_server":"old.example.com","target_port":8080,"password":"old123"}`,
	}
	err = memStore.ProxyConfigDAO().CreateConfig(&configV1)
	if err != nil {
		t.Fatalf("创建配置失败: %v", err)
	}
	log.Printf("✅ 配置已创建: ID=%d", configV1.ID)

	versionManager := controlServer.GetVersionManager()
	configV1JSON, _ := json.Marshal(configV1)
	version1, err := versionManager.CaptureVersion(configV1.ID, string(configV1JSON), "create", "初始版本", "system")
	if err != nil {
		t.Fatalf("创建版本1失败: %v", err)
	}
	log.Printf("✅ 版本1已创建: Version=%d", version1.Version)

	// 创建版本2（当前版本，有问题需要回滚）
	configV2 := configV1
	configV2.ConfigJSON = `{"target_server":"new.example.com","target_port":9090,"password":"new456"}`
	configV2JSON, _ := json.Marshal(configV2)
	version2, err := versionManager.CaptureVersion(configV1.ID, string(configV2JSON), "update", "有问题的更新", "admin")
	if err != nil {
		t.Fatalf("创建版本2失败: %v", err)
	}
	log.Printf("✅ 版本2已创建: Version=%d", version2.Version)

	// 步骤3: 控制端推送回滚任务到数据库
	log.Println("\n✓ 步骤3: 控制端推送回滚任务")
	err = controlServer.PushRollbackToNode("agent-test-node", configV1.ID, version1.Version, "版本2有bug，回滚到版本1")
	if err != nil {
		t.Fatalf("推送回滚任务失败: %v", err)
	}
	log.Println("✅ 回滚任务已推送到数据库")

	// 验证任务已在数据库中
	tasks, err := memStore.RollbackTaskDAO().GetPendingTasksByNode("agent-test-node")
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("任务队列长度错误: 期望=1, 实际=%d", len(tasks))
	}
	log.Printf("✅ 数据库中有 %d 个待执行任务", len(tasks))

	// 步骤4: Agent 建立 StreamConfig 连接
	log.Println("\n✓ 步骤4: Agent 建立 StreamConfig 连接")
	md := map[string]string{
		"node_id":       "agent-test-node",
		"authorization": fmt.Sprintf("Bearer %s", registerResp.ControlToken),
	}

	streamCtx := metadata.NewOutgoingContext(context.Background(), metadata.New(md))

	stream, err := client.StreamConfig(streamCtx)
	if err != nil {
		t.Fatalf("创建流失败: %v", err)
	}
	defer stream.CloseSend()
	log.Println("✅ StreamConfig 流已建立")

	// 步骤5: Agent 发送 get 请求（触发回滚任务推送）
	log.Println("\n✓ 步骤5: Agent 发送 get 请求")
	getReq := &pb.ConfigRequest{
		NodeId:      "agent-test-node",
		RequestType: "get",
	}

	err = stream.Send(getReq)
	if err != nil {
		t.Fatalf("发送 get 请求失败: %v", err)
	}
	log.Println("✅ get 请求已发送")

	// 步骤6: Agent 接收配置更新（包含 RollbackInfo）
	log.Println("\n✓ 步骤6: Agent 接收配置更新")
	update, err := stream.Recv()
	if err != nil {
		t.Fatalf("接收响应失败: %v", err)
	}

	if !update.Success {
		t.Fatalf("配置获取失败: %s", update.Message)
	}

	// 验证 RollbackInfo
	if update.RollbackInfo == nil {
		t.Fatal("❌ 未收到 RollbackInfo")
	}

	log.Printf("✅ 收到 RollbackInfo: ConfigID=%d, TargetVersion=%d, Reason=%s",
		update.RollbackInfo.ConfigId,
		update.RollbackInfo.TargetVersion,
		update.RollbackInfo.RollbackReason)

	// 验证回滚配置数据
	if len(update.Configs) == 0 {
		t.Fatal("❌ 回滚任务没有附带配置数据")
	}

	rollbackConfig := update.Configs[0]
	log.Printf("✅ 收到回滚配置: ID=%d, Name=%s, TargetServer=%s, TargetPort=%d",
		rollbackConfig.Id, rollbackConfig.Name,
		rollbackConfig.TargetServer, rollbackConfig.TargetPort)

	// 验证配置是否是版本1的内容
	if rollbackConfig.TargetServer != "old.example.com" {
		t.Errorf("❌ 回滚配置错误: 期望 target_server=old.example.com, 实际=%s",
			rollbackConfig.TargetServer)
	}

	if rollbackConfig.TargetPort != 8080 {
		t.Errorf("❌ 回滚配置错误: 期望 target_port=8080, 实际=%d",
			rollbackConfig.TargetPort)
	}

	// 步骤7: 验证数据库中任务状态已更新为 processing
	log.Println("\n✓ 步骤7: 验证任务状态")
	tasksAfter, err := memStore.RollbackTaskDAO().GetTasksByNode("agent-test-node")
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}

	if len(tasksAfter) != 1 {
		t.Fatalf("任务数量错误: 期望=1, 实际=%d", len(tasksAfter))
	}

	if tasksAfter[0].Status != "processing" {
		t.Errorf("任务状态错误: 期望=processing, 实际=%s", tasksAfter[0].Status)
	}

	log.Printf("✅ 任务状态已更新为: %s", tasksAfter[0].Status)

	log.Println("\n=== 🎉 Agent 回滚执行集成测试完成 ===")
	log.Println("验证通过：")
	log.Println("  1. ✅ 控制端成功推送回滚任务到数据库")
	log.Println("  2. ✅ Agent 发送 get 请求触发任务处理")
	log.Println("  3. ✅ Agent 收到包含 RollbackInfo 的配置更新")
	log.Println("  4. ✅ 回滚配置数据正确（版本1的内容）")
	log.Println("  5. ✅ 数据库中任务状态更新为 processing")
}
