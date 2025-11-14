package client

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "goForward/proto"
)

// AgentClient Agent客户端
type AgentClient struct {
	conn          *grpc.ClientConn
	client        pb.ControlServiceClient
	nodeID        string
	controlAddr   string
	controlToken  string
	ctx           context.Context
	cancel        context.CancelFunc
	heartbeatCtx  context.Context
	heartbeatCancel context.CancelFunc
	configCtx     context.Context
	configCancel  context.CancelFunc
	heartbeatStream pb.ControlService_HeartbeatClient
	configStream  pb.ControlService_StreamConfigClient
}

// NewAgentClient 创建新的Agent客户端
func NewAgentClient(controlAddr, nodeID string) *AgentClient {
	return &AgentClient{
		controlAddr: controlAddr,
		nodeID:      nodeID,
		ctx:         context.Background(),
	}
}

// Connect 连接到控制端
func (a *AgentClient) Connect() error {
	conn, err := grpc.Dial(a.controlAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("连接控制端失败: %v", err)
	}

	a.conn = conn
	a.client = pb.NewControlServiceClient(conn)

	log.Printf("[Agent] 连接到控制端: %s", a.controlAddr)
	return nil
}

// RegisterNode 注册节点
func (a *AgentClient) RegisterNode() error {
	nodeInfo := &pb.NodeInfo{
		NodeId:    a.nodeID,
		Hostname:  getHostname(),
		IpAddress: getLocalIP(),
		Version:   "v2.0.0",
		Uptime:    time.Now().Unix(),
		Labels:    map[string]string{"environment": "production"},
	}

	resp, err := a.client.RegisterNode(a.ctx, nodeInfo)
	if err != nil {
		return fmt.Errorf("注册节点失败: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("注册失败: %s", resp.Message)
	}

	a.controlToken = resp.ControlToken
	log.Printf("[Agent] 节点注册成功，控制令牌: %s", a.controlToken)

	// 设置认证元数据
	md := metadata.New(map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", a.controlToken),
		"node_id":       a.nodeID,
	})
	a.ctx = metadata.NewOutgoingContext(a.ctx, md)

	return nil
}

// StartHeartbeat 启动心跳保活
func (a *AgentClient) StartHeartbeat() error {
	a.heartbeatCtx, a.heartbeatCancel = context.WithCancel(a.ctx)

	stream, err := a.client.Heartbeat(a.heartbeatCtx)
	if err != nil {
		return fmt.Errorf("启动心跳流失败: %v", err)
	}

	a.heartbeatStream = stream

	// 启动心跳发送goroutine
	go a.heartbeatSender()

	// 启动心跳接收goroutine
	go a.heartbeatReceiver()

	log.Printf("[Agent] 心跳保活已启动")
	return nil
}

// heartbeatSender 发送心跳
func (a *AgentClient) heartbeatSender() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			health := a.getNodeHealth()

			req := &pb.HeartbeatRequest{
				NodeId:    a.nodeID,
				Timestamp: time.Now().Unix(),
				Health:    health,
			}

			if err := a.heartbeatStream.Send(req); err != nil {
				log.Printf("[Agent] 发送心跳失败: %v", err)
				return
			}
		case <-a.heartbeatCtx.Done():
			return
		}
	}
}

// heartbeatReceiver 接收心跳响��
func (a *AgentClient) heartbeatReceiver() {
	for {
		resp, err := a.heartbeatStream.Recv()
		if err != nil {
			log.Printf("[Agent] 接收心跳响应失败: %v", err)
			return
		}

		log.Printf("[Agent] 收到心跳响应: Alive=%v, Next=%ds",
			resp.Alive, resp.NextHeartbeat)
	}
}

// StartConfigStream 启动配置流
func (a *AgentClient) StartConfigStream() error {
	a.configCtx, a.configCancel = context.WithCancel(a.ctx)

	stream, err := a.client.StreamConfig(a.configCtx)
	if err != nil {
		return fmt.Errorf("启动配置流失败: %v", err)
	}

	a.configStream = stream

	log.Printf("[Agent] 配置流已启动")
	return nil
}

// RequestConfig 请求配置
func (a *AgentClient) RequestConfig() ([]*pb.ProxyConfig, error) {
	req := &pb.ConfigRequest{
		NodeId:      a.nodeID,
		RequestType: "get",
	}

	if err := a.configStream.Send(req); err != nil {
		return nil, fmt.Errorf("发送配置请求失败: %v", err)
	}

	update, err := a.configStream.Recv()
	if err != nil {
		return nil, fmt.Errorf("接收配置响应失败: %v", err)
	}

	if !update.Success {
		return nil, fmt.Errorf("获取配置失败: %s", update.Message)
	}

	log.Printf("[Agent] 收到配置: %d 个代理", len(update.Configs))
	return update.Configs, nil
}

// UpdateConfig 更新配置
func (a *AgentClient) UpdateConfig(config *pb.ProxyConfig) error {
	req := &pb.ConfigRequest{
		NodeId:      a.nodeID,
		RequestType: "update",
		Config:      config,
	}

	if err := a.configStream.Send(req); err != nil {
		return fmt.Errorf("发送配置更新失败: %v", err)
	}

	update, err := a.configStream.Recv()
	if err != nil {
		return fmt.Errorf("接收配置更新响应失败: %v", err)
	}

	if !update.Success {
		return fmt.Errorf("配置更新失败: %s", update.Message)
	}

	log.Printf("[Agent] 配置更新成功: %s", update.Message)
	return nil
}

// DeleteConfig 删除配置
func (a *AgentClient) DeleteConfig(configID int32) error {
	req := &pb.ConfigRequest{
		NodeId:      a.nodeID,
		RequestType: "delete",
		Config: &pb.ProxyConfig{
			Id: configID,
		},
	}

	if err := a.configStream.Send(req); err != nil {
		return fmt.Errorf("发送配置删除失败: %v", err)
	}

	update, err := a.configStream.Recv()
	if err != nil {
		return fmt.Errorf("接收配置删除响应失败: %v", err)
	}

	if !update.Success {
		return fmt.Errorf("配置删除失败: %s", update.Message)
	}

	log.Printf("[Agent] 配置删除成功: %s", update.Message)
	return nil
}

// ReportStatus 上报状态
func (a *AgentClient) ReportStatus(proxies []*pb.ProxyStatus) error {
	status := &pb.NodeStatus{
		NodeId:    a.nodeID,
		Timestamp: time.Now().Unix(),
		Health:    a.getNodeHealth(),
		Proxies:   proxies,
	}

	resp, err := a.client.ReportStatus(a.ctx, status)
	if err != nil {
		return fmt.Errorf("上报状态失败: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("状态上报失败: %s", resp.Message)
	}

	log.Printf("[Agent] 状态上报成功，推荐操作: %s", resp.Action)
	return nil
}

// getNodeHealth 获取节点健康状态
func (a *AgentClient) getNodeHealth() *pb.NodeHealth {
	// 实现真实的健康检查逻辑
	return &pb.NodeHealth{
		CpuPercent:        getCPUPercent(),
		MemoryPercent:     getMemoryPercent(),
		DiskPercent:       getDiskPercent(),
		ActiveConnections: getActiveConnections(),
		NetworkRx:         getNetworkRx(),
		NetworkTx:         getNetworkTx(),
	}
}

// Disconnect 断开连接
func (a *AgentClient) Disconnect() error {
	if a.heartbeatCancel != nil {
		a.heartbeatCancel()
	}
	if a.configCancel != nil {
		a.configCancel()
	}
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

// Stop 停止客户端
func (a *AgentClient) Stop() error {
	log.Printf("[Agent] 正在停止客户端...")

	if a.heartbeatCancel != nil {
		a.heartbeatCancel()
	}
	if a.configCancel != nil {
		a.configCancel()
	}

	if err := a.Disconnect(); err != nil {
		return fmt.Errorf("断开连接失败: %v", err)
	}

	log.Printf("[Agent] 客户端已停止")
	return nil
}

// 辅助函数

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "unknown"
}

func getCPUPercent() int64 {
	// 使用runtime.GOMAXPROCS获取CPU核心数，然后模拟CPU使用率
	// 实际生产中应该使用系统调用获取真实CPU使用率
	numCPU := runtime.NumCPU()
	return int64(10 + (numCPU * 5)) // 模拟值
}

func getMemoryPercent() int64 {
	// 实际生产中应该使用gopsutil库
	// 这里返回模拟值
	return int64(45)
}

func getDiskPercent() int64 {
	// 实际生产中应该使用gopsutil库获取磁盘使用率
	return int64(60)
}

func getActiveConnections() int32 {
	// 实际生产中应该从/proc/net/tcp等文件读取
	return int32(10)
}

func getNetworkRx() float64 {
	// 实际生产中应该使用gopsutil库
	return 1024.5
}

func getNetworkTx() float64 {
	// 实际生产中应该使用gopsutil库
	return 2048.3
}