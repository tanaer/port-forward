package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	pb "goForward/proto"
)

// ControlServer 实现ControlService接口
type ControlServer struct {
	pb.UnimplementedControlServiceServer

	// 节点注册表
	nodeRegistry *NodeRegistry

	// 配置管理器
	configManager *ConfigManager

	// 节点健康检查定时器
	healthCheckTicker *time.Ticker

	// 互斥锁
	mu sync.Mutex
}

// NodeRegistry 节点注册表
type NodeRegistry struct {
	nodes map[string]*NodeInfo // node_id -> NodeInfo
	mu    sync.RWMutex
}

// NodeInfo 节点信息
type NodeInfo struct {
	Info          *pb.NodeInfo
	LastHeartbeat time.Time
	Status        string // "active", "inactive", "unknown"
}

// ConfigManager 配置管理器
type ConfigManager struct {
	configs map[int32]*pb.ProxyConfig // config_id -> ProxyConfig
	mu      sync.RWMutex
}

// NewControlServer 创建新的控制服务器
func NewControlServer() *ControlServer {
	return &ControlServer{
		nodeRegistry: &NodeRegistry{
			nodes: make(map[string]*NodeInfo),
		},
		configManager: &ConfigManager{
			configs: make(map[int32]*pb.ProxyConfig),
		},
		healthCheckTicker: time.NewTicker(30 * time.Second),
	}
}

// RegisterNode 实现节点注册
func (s *ControlServer) RegisterNode(ctx context.Context, nodeInfo *pb.NodeInfo) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[控制端] 收到节点注册请求: NodeID=%s, Hostname=%s, IP=%s",
		nodeInfo.NodeId, nodeInfo.Hostname, nodeInfo.IpAddress)

	// 生成控制令牌（实际中应该使用更安全的生成方式）
	controlToken := fmt.Sprintf("ctrl_%s_%d", nodeInfo.NodeId, time.Now().Unix())

	// 注册节点
	s.nodeRegistry.mu.Lock()
	s.nodeRegistry.nodes[nodeInfo.NodeId] = &NodeInfo{
		Info:          nodeInfo,
		LastHeartbeat: time.Now(),
		Status:        "active",
	}
	s.nodeRegistry.mu.Unlock()

	log.Printf("[控制端] 节点注册成功: %s", nodeInfo.NodeId)

	return &pb.RegisterResponse{
		Success:      true,
		Message:      "节点注册成功",
		ControlToken: controlToken,
	}, nil
}

// Heartbeat 实现心跳保活（双向流）
func (s *ControlServer) Heartbeat(stream pb.ControlService_HeartbeatServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			log.Printf("[控制端] 心跳流接收错误: %v", err)
			return err
		}

		nodeID := req.NodeId
		log.Printf("[控制端] 收到心跳: NodeID=%s, 健康状态: CPU=%d%%, Mem=%d%%",
			nodeID, req.Health.CpuPercent, req.Health.MemoryPercent)

		// 更新节点心跳时间
		s.nodeRegistry.mu.Lock()
		if node, exists := s.nodeRegistry.nodes[nodeID]; exists {
			node.LastHeartbeat = time.Now()
			node.Status = "active"
		}
		s.nodeRegistry.mu.Unlock()

		// 发送心跳响应
		resp := &pb.HeartbeatResponse{
			NodeId:       nodeID,
			Timestamp:    time.Now().Unix(),
			Alive:        true,
			NextHeartbeat: 30, // 下次30秒后
		}

		if err := stream.Send(resp); err != nil {
			log.Printf("[控制端] 心跳流发送错误: %v", err)
			return err
		}
	}
}

// StreamConfig 实现配置分发（双向流）
func (s *ControlServer) StreamConfig(stream pb.ControlService_StreamConfigServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			log.Printf("[控制端] 配置流接收错误: %v", err)
			return err
		}

		nodeID := req.NodeId
		requestType := req.RequestType
		log.Printf("[控制端] 收到配置请求: NodeID=%s, Type=%s", nodeID, requestType)

		var update *pb.ConfigUpdate

		switch requestType {
		case "get":
			// 返回当前所有配置
			s.configManager.mu.RLock()
			var configs []*pb.ProxyConfig
			for _, config := range s.configManager.configs {
				configs = append(configs, config)
			}
			s.configManager.mu.RUnlock()

			update = &pb.ConfigUpdate{
				NodeId:  nodeID,
				Success: true,
				Message: "获取配置成功",
				Configs: configs,
			}

		case "update":
			// 更新配置
			if req.Config != nil {
				s.configManager.mu.Lock()
				s.configManager.configs[req.Config.Id] = req.Config
				s.configManager.mu.Unlock()

				update = &pb.ConfigUpdate{
					NodeId:  nodeID,
					Success: true,
					Message: fmt.Sprintf("配置 %d 更新成功", req.Config.Id),
				}
				log.Printf("[控制端] 配置更新: ID=%d, Name=%s", req.Config.Id, req.Config.Name)
			} else {
				update = &pb.ConfigUpdate{
					NodeId:  nodeID,
					Success: false,
					Message: "无效的配置",
				}
			}

		case "delete":
			// 删除配置
			if req.Config != nil {
				s.configManager.mu.Lock()
				delete(s.configManager.configs, req.Config.Id)
				s.configManager.mu.Unlock()

				update = &pb.ConfigUpdate{
					NodeId:  nodeID,
					Success: true,
					Message: fmt.Sprintf("配置 %d 删除成功", req.Config.Id),
				}
				log.Printf("[控制端] 配置删除: ID=%d", req.Config.Id)
			} else {
				update = &pb.ConfigUpdate{
					NodeId:  nodeID,
					Success: false,
					Message: "未指定要删除的配置",
				}
			}

		default:
			update = &pb.ConfigUpdate{
				NodeId:  nodeID,
				Success: false,
				Message: fmt.Sprintf("未知的请求类型: %s", requestType),
			}
		}

		// 发送配置更新响应
		if err := stream.Send(update); err != nil {
			log.Printf("[控制端] 配置流发送错误: %v", err)
			return err
		}
	}
}

// ReportStatus 实现状态上报
func (s *ControlServer) ReportStatus(ctx context.Context, status *pb.NodeStatus) (*pb.StatusResponse, error) {
	nodeID := status.NodeId
	log.Printf("[控制端] 收到状态上报: NodeID=%s, 代理数量=%d",
		nodeID, len(status.Proxies))

	// 分析状态，决定推荐操作
	action := "none"
	hasErrors := false

	for _, proxy := range status.Proxies {
		if !proxy.Running {
			hasErrors = true
			log.Printf("[控制端] 代理异常: ID=%d, Name=%s, 错误=%s",
				proxy.Id, proxy.Name, proxy.ErrorMessage)
		}
	}

	if hasErrors {
		action = "restart"
	}

	return &pb.StatusResponse{
		Success: true,
		Message: "状态上报成功",
		Action:  action,
	}, nil
}

// Start 启动gRPC服务器
func (s *ControlServer) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("监听地址失败: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterControlServiceServer(grpcServer, s)

	log.Printf("[控制端] gRPC服务器启动，监听地址: %s", address)

	// 启动健康检查goroutine
	go s.healthCheck()

	// 启动服务器
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("服务器运行错误: %v", err)
	}

	return nil
}

// healthCheck 定期检查节点健康状态
func (s *ControlServer) healthCheck() {
	for range s.healthCheckTicker.C {
		s.checkNodeHealth()
	}
}

// checkNodeHealth 检查节点健康状态
func (s *ControlServer) checkNodeHealth() {
	now := time.Now()
	timeout := 90 * time.Second // 90秒超时

	s.nodeRegistry.mu.Lock()
	defer s.nodeRegistry.mu.Unlock()

	for nodeID, node := range s.nodeRegistry.nodes {
		if now.Sub(node.LastHeartbeat) > timeout {
			if node.Status != "inactive" {
				node.Status = "inactive"
				log.Printf("[控制端] 节点失联: %s", nodeID)
			}
		}
	}
}

// GetNodes 获取所有节点
func (s *ControlServer) GetNodes() map[string]*NodeInfo {
	s.nodeRegistry.mu.RLock()
	defer s.nodeRegistry.mu.RUnlock()

	result := make(map[string]*NodeInfo)
	for k, v := range s.nodeRegistry.nodes {
		result[k] = v
	}
	return result
}

// AddConfig 添加配置
func (s *ControlServer) AddConfig(config *pb.ProxyConfig) {
	s.configManager.mu.Lock()
	defer s.configManager.mu.Unlock()
	s.configManager.configs[config.Id] = config
	log.Printf("[控制端] 配置已添加: ID=%d, Name=%s", config.Id, config.Name)
}

// GetConfigs 获取所有配置
func (s *ControlServer) GetConfigs() map[int32]*pb.ProxyConfig {
	s.configManager.mu.RLock()
	defer s.configManager.mu.RUnlock()

	result := make(map[int32]*pb.ProxyConfig)
	for k, v := range s.configManager.configs {
		result[k] = v
	}
	return result
}