package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "goForward/proto"
)

// ControlServer 实现ControlService接口
type ControlServer struct {
	pb.UnimplementedControlServiceServer

	// 节点注册表
	nodeRegistry *NodeRegistry

	// 配置管理器
	configManager *ConfigManager

	// Token管理器
	tokenManager *TokenManager

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
	ControlToken  string // 控制令牌
	Health        *pb.NodeHealth // 最新健康状态
}

// ConfigManager 配置管理器
type ConfigManager struct {
	configs map[int32]*pb.ProxyConfig // config_id -> ProxyConfig
	mu      sync.RWMutex
}

// TokenManager Token管理器
type TokenManager struct {
	tokens map[string]string // node_id -> token
	mu     sync.RWMutex
}

// NewControlServer 创建新的控制服务器
func NewControlServer() *ControlServer {
	server := &ControlServer{
		nodeRegistry: &NodeRegistry{
			nodes: make(map[string]*NodeInfo),
		},
		configManager: &ConfigManager{
			configs: make(map[int32]*pb.ProxyConfig),
		},
		tokenManager: &TokenManager{
			tokens: make(map[string]string),
		},
		healthCheckTicker: time.NewTicker(30 * time.Second),
	}

	// 启动健康检查goroutine
	go server.healthCheck()

	return server
}

// generateToken 生成Token
func (s *ControlServer) generateToken(nodeID string) string {
	// 生成32字节随机数
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)

	// 计算SHA256哈希
	hash := sha256.Sum256(bytes)
	token := hex.EncodeToString(hash[:])

	// 保存Token
	s.tokenManager.mu.Lock()
	s.tokenManager.tokens[nodeID] = token
	s.tokenManager.mu.Unlock()

	return token
}

// validateToken 验证Token
func (s *ControlServer) validateToken(nodeID, token string) bool {
	s.tokenManager.mu.RLock()
	defer s.tokenManager.mu.RUnlock()

	storedToken, exists := s.tokenManager.tokens[nodeID]
	if !exists {
		return false
	}

	return storedToken == token
}

// extractAuth 从context中提取认证信息
func (s *ControlServer) extractAuth(ctx context.Context) (string, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", status.Error(codes.Unauthenticated, "缺少认证元数据")
	}

	nodeID := ""
	token := ""

	if vals := md["node_id"]; len(vals) > 0 {
		nodeID = vals[0]
	}
	if vals := md["authorization"]; len(vals) > 0 {
		// 格式: "Bearer <token>"
		auth := vals[0]
		if len(auth) > 7 && auth[:7] == "Bearer " {
			token = auth[7:]
		}
	}

	if nodeID == "" || token == "" {
		return "", "", status.Error(codes.Unauthenticated, "认证信息不完整")
	}

	return nodeID, token, nil
}

// RegisterNode 实现节点注册
func (s *ControlServer) RegisterNode(ctx context.Context, nodeInfo *pb.NodeInfo) (*pb.RegisterResponse, error) {
	log.Printf("[控制端] 收到节点注册请求: NodeID=%s, Hostname=%s, IP=%s",
		nodeInfo.NodeId, nodeInfo.Hostname, nodeInfo.IpAddress)

	// 生成控制令牌
	controlToken := s.generateToken(nodeInfo.NodeId)

	// 注册节点
	s.nodeRegistry.mu.Lock()
	s.nodeRegistry.nodes[nodeInfo.NodeId] = &NodeInfo{
		Info:          nodeInfo,
		LastHeartbeat: time.Now(),
		Status:        "active",
		ControlToken:  controlToken,
	}
	s.nodeRegistry.mu.Unlock()

	log.Printf("[控制端] 节点注册成功: %s, Token=%s", nodeInfo.NodeId, controlToken)

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
		log.Printf("[控制端] 收到心跳: NodeID=%s, 健康状态: CPU=%d%%, Mem=%d%%, Disk=%d%%, Conn=%d",
			nodeID, req.Health.CpuPercent, req.Health.MemoryPercent,
			req.Health.DiskPercent, req.Health.ActiveConnections)

		// 检查CPU使用率
		if req.Health.CpuPercent > 90 {
			log.Printf("[控制端] 警告: 节点 %s CPU使用率过高: %d%%", nodeID, req.Health.CpuPercent)
		}

		// 检查内存使用率
		if req.Health.MemoryPercent > 90 {
			log.Printf("[控制端] 警告: 节点 %s 内存使用率过高: %d%%", nodeID, req.Health.MemoryPercent)
		}

		// 检查磁盘使用率
		if req.Health.DiskPercent > 90 {
			log.Printf("[控制端] 警告: 节点 %s 磁盘使用率过高: %d%%", nodeID, req.Health.DiskPercent)
		}

		// 更新节点心跳时间
		s.nodeRegistry.mu.Lock()
		if node, exists := s.nodeRegistry.nodes[nodeID]; exists {
			node.LastHeartbeat = time.Now()
			node.Status = "active"

			// 更新健康信息
			node.Health = req.Health
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

		// 验证认证
		nodeID, token, err := s.extractAuth(stream.Context())
		if err != nil {
			log.Printf("[控制端] 配置流认证失败: %v", err)
			update := &pb.ConfigUpdate{
				NodeId:  req.NodeId,
				Success: false,
				Message: "认证失败",
			}
			_ = stream.Send(update)
			continue
		}

		// 验证Token
		if !s.validateToken(nodeID, token) {
			log.Printf("[控制端] 配置流Token验证失败: NodeID=%s", nodeID)
			update := &pb.ConfigUpdate{
				NodeId:  req.NodeId,
				Success: false,
				Message: "Token验证失败",
			}
			_ = stream.Send(update)
			continue
		}

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
				Message: fmt.Sprintf("获取配置成功，共%d个", len(configs)),
				Configs: configs,
			}
			log.Printf("[控制端] 向节点 %s 下发 %d 个配置", nodeID, len(configs))

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
	// 验证认证
	nodeID, token, err := s.extractAuth(ctx)
	if err != nil {
		log.Printf("[控制端] 状态上报认证失败: %v", err)
		return &pb.StatusResponse{
			Success: false,
			Message: "认证失败",
		}, nil
	}

	// 验证Token
	if !s.validateToken(nodeID, token) {
		log.Printf("[控制端] 状态上报Token验证失败: NodeID=%s", nodeID)
		return &pb.StatusResponse{
			Success: false,
			Message: "Token验证失败",
		}, nil
	}

	nodeID = status.NodeId
	log.Printf("[控制端] 收到状态上报: NodeID=%s, 代理数量=%d, CPU=%d%%, Mem=%d%%",
		nodeID, len(status.Proxies), status.Health.CpuPercent, status.Health.MemoryPercent)

	// 分析状态，决定推荐操作
	action := "none"
	hasErrors := false
	var errorMsgs []string

	for _, proxy := range status.Proxies {
		if !proxy.Running {
			hasErrors = true
			msg := fmt.Sprintf("代理 ID=%d (%s) 异常: %s", proxy.Id, proxy.Name, proxy.ErrorMessage)
			errorMsgs = append(errorMsgs, msg)
			log.Printf("[控制端] %s", msg)
		}
	}

	if hasErrors {
		action = "restart"
		log.Printf("[控制端] 节点 %s 存在异常代理，推荐重启", nodeID)
	}

	return &pb.StatusResponse{
		Success: true,
		Message: fmt.Sprintf("状态上报成功，检测到 %d 个异常", len(errorMsgs)),
		Action:  action,
	}, nil
}

// Start 启动gRPC服务器
func (s *ControlServer) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("监听地址失败: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(1024*1024), // 1MB
		grpc.MaxSendMsgSize(1024*1024), // 1MB
	)
	pb.RegisterControlServiceServer(grpcServer, s)

	log.Printf("[控制端] gRPC服务器启动，监听地址: %s", address)

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
				log.Printf("[控制端] 节点失联（心跳超时90秒）: %s", nodeID)

				// 发出告警
				log.Printf("[控制端] 🚨 告警: 节点 %s 失联，请检查节点状态", nodeID)
			}
		} else {
			// 检查健康指标
			if node.Health != nil {
				if node.Health.CpuPercent > 90 {
					log.Printf("[控制端] ⚠️ 告警: 节点 %s CPU使用率过高: %d%%", nodeID, node.Health.CpuPercent)
				}
				if node.Health.MemoryPercent > 90 {
					log.Printf("[控制端] ⚠️ 告警: 节点 %s 内存使用率过高: %d%%", nodeID, node.Health.MemoryPercent)
				}
				if node.Health.DiskPercent > 90 {
					log.Printf("[控制端] ⚠️ 告警: 节点 %s 磁盘使用率过高: %d%%", nodeID, node.Health.DiskPercent)
				}
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

// GetNodeStatus 获取单个节点状态
func (s *ControlServer) GetNodeStatus(nodeID string) (*NodeInfo, bool) {
	s.nodeRegistry.mu.RLock()
	defer s.nodeRegistry.mu.RUnlock()

	node, exists := s.nodeRegistry.nodes[nodeID]
	return node, exists
}

// AddConfig 添加配置
func (s *ControlServer) AddConfig(config *pb.ProxyConfig) {
	s.configManager.mu.Lock()
	defer s.configManager.mu.Unlock()
	s.configManager.configs[config.Id] = config
	log.Printf("[控制端] 配置已添加: ID=%d, Name=%s, OutboundType=%s",
		config.Id, config.Name, config.OutboundType)
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

// RemoveConfig 删除配置
func (s *ControlServer) RemoveConfig(configID int32) {
	s.configManager.mu.Lock()
	defer s.configManager.mu.Unlock()
	delete(s.configManager.configs, configID)
	log.Printf("[控制端] 配置已删除: ID=%d", configID)
}