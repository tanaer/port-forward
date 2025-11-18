package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"goForward/control/store"
	pb "goForward/proto"
)

// ControlServer 实现ControlService接口
type ControlServer struct {
	pb.UnimplementedControlServiceServer

	// 节点注册表
	nodeRegistry *NodeRegistry

	// 配置管理器
	configManager *ConfigManager

	// 版本管理器（用于配置版本快照和历史管理）
	versionManager *VersionManager

	// Token管理器
	tokenManager *TokenManager

	// 数据存储层
	store *store.Store

	// WebSocket中心
	wsHub WebSocketHub

	// 事件总线
	eventBus *EventBus

	// 生命周期管理器
	lifecycleManager *LifecycleManager

	// 节点健康检查定时器
	healthCheckTicker *time.Ticker

	// 互斥锁
	mu sync.Mutex
}

// WebSocketHub WebSocket中心接口
type WebSocketHub interface {
	Broadcast(data interface{})
}

// NodeRegistry 节点注册表
// RollbackTask 回滚任务
type RollbackTask struct {
	ID            int32     `json:"id"`             // 任务ID
	ConfigID      int32     `json:"config_id"`      // 配置ID
	TargetVersion int32     `json:"target_version"` // 目标版本
	Reason        string    `json:"reason"`         // 回滚原因
	CreatedAt     time.Time `json:"created_at"`
}

type NodeRegistry struct {
	nodes         map[string]*NodeInfo       // node_id -> NodeInfo
	rollbackTasks map[string][]*RollbackTask // node_id -> 待执行的回滚任务列表
	mu            sync.RWMutex
}

// NodeInfo 节点信息
type NodeInfo struct {
	Info          *pb.NodeInfo
	LastHeartbeat time.Time
	Status        string         // "active", "inactive", "unknown"
	ControlToken  string         // 控制令牌
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

// NewControlServer 创建新的控制服务器（无WebSocket）
func NewControlServer(store *store.Store) *ControlServer {
	return NewControlServerWithWebSocket(store, nil)
}

// NewControlServerWithWebSocket 创建带WebSocket的控制服务器
func NewControlServerWithWebSocket(store *store.Store, wsHub WebSocketHub) *ControlServer {
	// 初始化基础组件
	nodeRegistry := &NodeRegistry{
		nodes:         make(map[string]*NodeInfo),
		rollbackTasks: make(map[string][]*RollbackTask),
	}

	// 创建事件总线（4个工作协程）
	eventBus := NewEventBus(4)

	// 创建生命周期管理器
	lifecycleManager := NewLifecycleManager(nodeRegistry, eventBus)

	// 创建版本管理器（支持 nil store 用于测试）
	var versionManager *VersionManager
	if store != nil {
		versionDAO := store.ConfigVersionDAO()
		versionManager = NewVersionManager(versionDAO, eventBus)
	}

	server := &ControlServer{
		nodeRegistry: nodeRegistry,
		configManager: &ConfigManager{
			configs: make(map[int32]*pb.ProxyConfig),
		},
		versionManager: versionManager,
		tokenManager: &TokenManager{
			tokens: make(map[string]string),
		},
		store:             store,
		wsHub:             wsHub,
		eventBus:          eventBus,
		lifecycleManager:  lifecycleManager,
		healthCheckTicker: time.NewTicker(30 * time.Second),
	}

	// 订阅事件处理器
	server.subscribeEventHandlers()

	// 启动健康检查goroutine
	go server.healthCheck()

	// 加载数据库中的节点数据
	server.loadNodesFromDatabase()

	return server
}

// NewControlServerFull 创建完整的控制服务器（带数据库和WebSocket）
func NewControlServerFull(store *store.Store, wsHub WebSocketHub) *ControlServer {
	return NewControlServerWithWebSocket(store, wsHub)
}

// subscribeEventHandlers 订阅事件处理器
func (s *ControlServer) subscribeEventHandlers() {
	if s.eventBus == nil {
		return
	}

	// 订阅日志处理器（所有事件）
	logHandler := NewLogEventHandler()
	s.eventBus.Subscribe(EventNodeRegistered, logHandler)
	s.eventBus.Subscribe(EventNodeHealthChanged, logHandler)
	s.eventBus.Subscribe(EventNodeIsolated, logHandler)
	s.eventBus.Subscribe(EventNodeRecovered, logHandler)
	s.eventBus.Subscribe(EventNodeOffline, logHandler)
	s.eventBus.Subscribe(EventNodeOnline, logHandler)
	s.eventBus.Subscribe(EventConfigCreated, logHandler)
	s.eventBus.Subscribe(EventConfigUpdated, logHandler)
	s.eventBus.Subscribe(EventConfigDeleted, logHandler)
	s.eventBus.Subscribe(EventConfigRolledBack, logHandler)
	s.eventBus.Subscribe(EventConfigVersionCreated, logHandler)
	s.eventBus.Subscribe(EventConfigApplied, logHandler)
	s.eventBus.Subscribe(EventConfigFailed, logHandler)
	s.eventBus.Subscribe(EventRollbackTaskCreated, logHandler)
	s.eventBus.Subscribe(EventRollbackTaskPushed, logHandler)
	s.eventBus.Subscribe(EventRollbackTaskFailed, logHandler)

	// 订阅 WebSocket 处理器（如果存在）
	if s.wsHub != nil {
		wsHandler := NewWebSocketEventHandler(s.wsHub)
		s.eventBus.Subscribe(EventNodeRegistered, wsHandler)
		s.eventBus.Subscribe(EventNodeHealthChanged, wsHandler)
		s.eventBus.Subscribe(EventNodeIsolated, wsHandler)
		s.eventBus.Subscribe(EventNodeRecovered, wsHandler)
		s.eventBus.Subscribe(EventNodeOffline, wsHandler)
		s.eventBus.Subscribe(EventNodeOnline, wsHandler)
		s.eventBus.Subscribe(EventConfigCreated, wsHandler)
		s.eventBus.Subscribe(EventConfigUpdated, wsHandler)
		s.eventBus.Subscribe(EventConfigDeleted, wsHandler)
		s.eventBus.Subscribe(EventConfigRolledBack, wsHandler)
		s.eventBus.Subscribe(EventConfigVersionCreated, wsHandler)
		s.eventBus.Subscribe(EventConfigApplied, wsHandler)
		s.eventBus.Subscribe(EventConfigFailed, wsHandler)
		s.eventBus.Subscribe(EventRollbackTaskCreated, wsHandler)
		s.eventBus.Subscribe(EventRollbackTaskPushed, wsHandler)
		s.eventBus.Subscribe(EventRollbackTaskFailed, wsHandler)
	}

	// 订阅数据库处理器（节点事件）
	if s.store != nil {
		dbHandler := NewDatabaseEventHandler(s.store)
		s.eventBus.Subscribe(EventNodeRegistered, dbHandler)
		s.eventBus.Subscribe(EventNodeHealthChanged, dbHandler)
		s.eventBus.Subscribe(EventNodeIsolated, dbHandler)
		s.eventBus.Subscribe(EventNodeRecovered, dbHandler)
		s.eventBus.Subscribe(EventNodeOffline, dbHandler)
		s.eventBus.Subscribe(EventNodeOnline, dbHandler)
	}

	log.Println("[控制端] 事件处理器订阅完成")
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

	// 检查是否为新节点注册
	isNewNode := false
	if s.store != nil {
		if existingNode, _ := s.store.NodeDAO().GetNodeByID(nodeInfo.NodeId); existingNode == nil {
			isNewNode = true
			log.Printf("[控制端] 检测到新节点注册: %s", nodeInfo.NodeId)
		} else {
			log.Printf("[控制端] 检测到节点重新注册: %s", nodeInfo.NodeId)
		}
	}

	// 生成控制令牌
	controlToken := s.generateToken(nodeInfo.NodeId)

	// 保存到数据库
	if s.store != nil {
		nodeRecord := &store.NodeRecord{
			NodeID:       nodeInfo.NodeId,
			Hostname:     nodeInfo.Hostname,
			IPAddress:    nodeInfo.IpAddress,
			Version:      nodeInfo.Version,
			Status:       "active",
			ControlToken: controlToken,
		}

		// 如果节点已存在，先删除
		if existingNode, _ := s.store.NodeDAO().GetNodeByID(nodeInfo.NodeId); existingNode != nil {
			if err := s.store.NodeDAO().DeleteNode(nodeInfo.NodeId); err != nil {
				log.Printf("[控制端] 删除旧节点记录失败: %v", err)
			}
		}

		// 创建新节点记录
		if err := s.store.NodeDAO().CreateNode(nodeRecord); err != nil {
			log.Printf("[控制端] 保存节点到数据库失败: %v", err)
		}

		// 记录节点注册日志
		if isNewNode {
			eventMessage := fmt.Sprintf("新节点注册: %s (%s)", nodeInfo.Hostname, nodeInfo.IpAddress)
			if err := s.store.NodeLogDAO().CreateLog(&store.NodeLogRecord{
				NodeID:    nodeInfo.NodeId,
				LogType:   "node_registered",
				Message:   eventMessage,
				Data:      fmt.Sprintf(`{"hostname":"%s","ip":"%s","version":"%s"}`, nodeInfo.Hostname, nodeInfo.IpAddress, nodeInfo.Version),
				CreatedAt: time.Now().Unix(),
			}); err != nil {
				log.Printf("[控制端] 记录节点注册日志失败: %v", err)
			}
		} else {
			eventMessage := fmt.Sprintf("节点重新注册: %s (%s)", nodeInfo.Hostname, nodeInfo.IpAddress)
			if err := s.store.NodeLogDAO().CreateLog(&store.NodeLogRecord{
				NodeID:    nodeInfo.NodeId,
				LogType:   "node_reregistered",
				Message:   eventMessage,
				Data:      fmt.Sprintf(`{"hostname":"%s","ip":"%s","version":"%s"}`, nodeInfo.Hostname, nodeInfo.IpAddress, nodeInfo.Version),
				CreatedAt: time.Now().Unix(),
			}); err != nil {
				log.Printf("[控制端] 记录节点重新注册日志失败: %v", err)
			}
		}
	}

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

	// 发布节点注册事件
	if s.eventBus != nil {
		s.eventBus.Publish(&Event{
			Type:   EventNodeRegistered,
			NodeID: nodeInfo.NodeId,
			Data: map[string]interface{}{
				"is_new_node": isNewNode,
				"hostname":    nodeInfo.Hostname,
				"ip_address":  nodeInfo.IpAddress,
				"version":     nodeInfo.Version,
			},
			Timestamp: time.Now().Unix(),
		})
	}

	// 触发生命周期事件
	if s.lifecycleManager != nil {
		s.lifecycleManager.OnNodeRegistered(nodeInfo.NodeId, nodeInfo)
	}

	// 通过WebSocket广播新节点
	if s.wsHub != nil {
		s.broadcastNodeUpdate("node_registered", nodeInfo.NodeId)
	}

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

		// 使用生命周期管理器更新健康状态（包含自动恢复检测）
		if s.lifecycleManager != nil {
			s.lifecycleManager.UpdateNodeHealth(nodeID, req.Health)
		}

		// 更新节点心跳时间
		s.nodeRegistry.mu.Lock()
		if node, exists := s.nodeRegistry.nodes[nodeID]; exists {
			node.LastHeartbeat = time.Now()
			node.Status = "active"
		}
		s.nodeRegistry.mu.Unlock()

		// 更新数据库
		if s.store != nil {
			// 更新节点状态
			s.store.NodeDAO().UpdateNodeStatus(nodeID, "active")

			// 记录心跳日志
			s.store.NodeLogDAO().CreateLog(&store.NodeLogRecord{
				NodeID:  nodeID,
				LogType: "heartbeat",
				Message: "心跳保活",
				Data: fmt.Sprintf(`{"cpu_percent": %d, "memory_percent": %d, "disk_percent": %d, "active_connections": %d}`,
					req.Health.CpuPercent, req.Health.MemoryPercent, req.Health.DiskPercent, req.Health.ActiveConnections),
				CreatedAt: time.Now().Unix(),
			})
		}

		// 通过WebSocket广播节点状态更新
		if s.wsHub != nil {
			s.broadcastNodeUpdate("heartbeat", nodeID)
		}

		// 发送心跳响应
		resp := &pb.HeartbeatResponse{
			NodeId:        nodeID,
			Timestamp:     time.Now().Unix(),
			Alive:         true,
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

			// 检查是否有待执行的回滚任务
			var task *RollbackTask
			var dbTaskID int64

			// 优先从数据库读取任务
			if s.store != nil {
				dbTasks, err := s.store.RollbackTaskDAO().GetPendingTasksByNode(nodeID)
				if err != nil {
					log.Printf("[控制端] 从数据库读取任务失败: %v", err)
				} else if len(dbTasks) > 0 {
					// 使用数据库中的第一个待执行任务
					dbTask := dbTasks[0]
					dbTaskID = dbTask.ID
					task = &RollbackTask{
						ID:            int32(dbTask.ID),
						ConfigID:      dbTask.ConfigID,
						TargetVersion: dbTask.TargetVersion,
						Reason:        dbTask.Reason,
						CreatedAt:     time.Unix(dbTask.CreatedAt, 0),
					}
					log.Printf("[控制端] 从数据库加载任务: TaskID=%d, ConfigID=%d, TargetVersion=%d",
						task.ID, task.ConfigID, task.TargetVersion)
				}
			}

			// 如果数据库没有任务，检查内存队列（降级方案）
			if task == nil {
				s.nodeRegistry.mu.Lock()
				memTasks := s.nodeRegistry.rollbackTasks[nodeID]
				if len(memTasks) > 0 {
					task = memTasks[0]
					log.Printf("[控制端] 从内存队列加载任务: TaskID=%d, ConfigID=%d, TargetVersion=%d",
						task.ID, task.ConfigID, task.TargetVersion)
				}
				s.nodeRegistry.mu.Unlock()
			}

			if task != nil {

				log.Printf("[控制端] 向节点 %s 发送回滚任务: TaskID=%d, ConfigID=%d, TargetVersion=%d",
					nodeID, task.ID, task.ConfigID, task.TargetVersion)

				// 获取目标版本配置
				var rollbackConfig *pb.ProxyConfig
				var taskFailed bool
				var failureReason string

				if s.versionManager == nil {
					taskFailed = true
					failureReason = "版本管理器未初始化"
					log.Printf("[控制端] %s", failureReason)
				} else {
					record, err := s.versionManager.GetVersion(task.ConfigID, task.TargetVersion)
					if err != nil {
						taskFailed = true
						failureReason = fmt.Sprintf("获取回滚版本失败: %v", err)
						log.Printf("[控制端] %s", failureReason)
					} else {
						var configRecord store.ProxyConfigRecord
						if err := json.Unmarshal([]byte(record.ConfigSnapshot), &configRecord); err != nil {
							taskFailed = true
							failureReason = fmt.Sprintf("解析回滚配置快照失败: %v", err)
							log.Printf("[控制端] %s", failureReason)
						} else {
							// 安全地解析配置JSON获取参数
							var configParams map[string]interface{}
							if err := json.Unmarshal([]byte(configRecord.ConfigJSON), &configParams); err != nil {
								taskFailed = true
								failureReason = fmt.Sprintf("解析回滚配置参数失败: %v", err)
								log.Printf("[控制端] %s", failureReason)
							} else {
								// 安全地提取并验证必需字段
								targetServer, ok := configParams["target_server"]
								if !ok {
									taskFailed = true
									failureReason = "回滚配置缺少 target_server 字段"
									log.Printf("[控制端] %s", failureReason)
								} else if targetServerStr, isString := targetServer.(string); !isString {
									taskFailed = true
									failureReason = fmt.Sprintf("target_server 字段类型错误: %T", targetServer)
									log.Printf("[控制端] %s", failureReason)
								} else {
									targetPortVal, hasPort := configParams["target_port"]
									if !hasPort {
										taskFailed = true
										failureReason = "回滚配置缺少 target_port 字段"
										log.Printf("[控制端] %s", failureReason)
									} else {
										targetPort, isNumber := targetPortVal.(float64)
										if !isNumber {
											taskFailed = true
											failureReason = fmt.Sprintf("target_port 字段类型错误: %T", targetPortVal)
											log.Printf("[控制端] %s", failureReason)
										} else {
											// 所有字段验证通过，构造回滚配置
											rollbackConfig = &pb.ProxyConfig{
												Id:           configRecord.ID,
												Name:         configRecord.Name,
												OutboundType: configRecord.OutboundType,
												InboundPort:  configRecord.InboundPort,
												TargetServer: targetServerStr,
												TargetPort:   int32(targetPort),
												Params:       make(map[string]string),
											}

											// 转换其他参数
											for k, v := range configParams {
												if k != "target_server" && k != "target_port" {
													rollbackConfig.Params[k] = fmt.Sprintf("%v", v)
												}
											}
										}
									}
								}
							}
						}
					}
				}

				// 处理任务结果
				if taskFailed {
					// 任务失败：发布失败事件，更新状态（保持pending，增加重试计数）
					update = &pb.ConfigUpdate{
						NodeId:  nodeID,
						Success: false,
						Message: failureReason,
					}

					// 发布任务失败事件
					if s.eventBus != nil {
						s.eventBus.Publish(&Event{
							Type:     EventRollbackTaskFailed,
							ConfigID: task.ConfigID,
							Data: map[string]interface{}{
								"task_id":        task.ID,
								"node_id":        nodeID,
								"reason":         failureReason,
								"target_version": task.TargetVersion,
							},
							Timestamp: time.Now().Unix(),
						})
					}

					// 更新数据库：增加重试计数（如果使用数据库）
					if dbTaskID > 0 && s.store != nil {
						if err := s.store.RollbackTaskDAO().IncrementRetryCount(dbTaskID); err != nil {
							log.Printf("[控制端] 更新任务重试计数失败: %v", err)
						}
						if err := s.store.RollbackTaskDAO().UpdateTaskStatus(dbTaskID, "pending", failureReason); err != nil {
							log.Printf("[控制端] 更新任务状态失败: %v", err)
						}
						log.Printf("[控制端] 回滚任务失败，已记录到数据库: TaskID=%d, Reason=%s", task.ID, failureReason)
					} else {
						// 内存队列：重新入队，供后续重试
						s.nodeRegistry.mu.Lock()
						memTasks := s.nodeRegistry.rollbackTasks[nodeID]
						if len(memTasks) > 0 {
							s.nodeRegistry.rollbackTasks[nodeID] = append([]*RollbackTask{task}, memTasks[1:]...)
						}
						s.nodeRegistry.mu.Unlock()
						log.Printf("[控制端] 回滚任务失败，已重新入队: TaskID=%d, Reason=%s", task.ID, failureReason)
					}
				} else {
					// 任务成功准备：发送回滚配置给Agent，并标记任务为已推送

					// 从数据库或内存删除/更新任务
					if dbTaskID > 0 && s.store != nil {
						// 数据库：标记为processing（等待Agent执行反馈）
						if err := s.store.RollbackTaskDAO().UpdateTaskStatus(dbTaskID, "processing", ""); err != nil {
							log.Printf("[控制端] 更新任务状态失败: %v", err)
						}
						log.Printf("[控制端] 回滚任务已标记为processing: TaskID=%d", task.ID)
					} else {
						// 内存队列：移除任务
						s.nodeRegistry.mu.Lock()
						memTasks := s.nodeRegistry.rollbackTasks[nodeID]
						if len(memTasks) > 0 {
							s.nodeRegistry.rollbackTasks[nodeID] = memTasks[1:]
						}
						s.nodeRegistry.mu.Unlock()
						log.Printf("[控制端] 回滚任务已从内存队列移除: TaskID=%d", task.ID)
					}

					update = &pb.ConfigUpdate{
						NodeId:  nodeID,
						Success: true,
						Message: fmt.Sprintf("回滚任务执行: TaskID=%d, ConfigID=%d -> Version=%d (%s)",
							task.ID, task.ConfigID, task.TargetVersion, task.Reason),
						Configs: []*pb.ProxyConfig{rollbackConfig},
						RollbackInfo: &pb.RollbackInfo{
							ConfigId:       task.ConfigID,
							TargetVersion:  task.TargetVersion,
							RollbackReason: task.Reason,
						},
					}

					// 发布任务推送成功事件
					if s.eventBus != nil {
						s.eventBus.Publish(&Event{
							Type:     EventRollbackTaskPushed,
							ConfigID: task.ConfigID,
							Data: map[string]interface{}{
								"task_id":        task.ID,
								"node_id":        nodeID,
								"target_version": task.TargetVersion,
								"reason":         task.Reason,
							},
							Timestamp: time.Now().Unix(),
						})
					}

					log.Printf("[控制端] 回滚任务已推送给Agent: TaskID=%d, ConfigID=%d",
						task.ID, task.ConfigID)
				}
			} else {
				// 无待执行任务，返回正常配置列表
				update = &pb.ConfigUpdate{
					NodeId:  nodeID,
					Success: true,
					Message: fmt.Sprintf("获取配置成功，共%d个", len(configs)),
					Configs: configs,
				}
				log.Printf("[控制端] 向节点 %s 下发 %d 个配置", nodeID, len(configs))
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

		case "rollback":
			// 处理回滚请求 - Agent请求控制端执行回滚
			if req.RollbackInfo != nil {
				configID := req.RollbackInfo.ConfigId
				targetVersion := req.RollbackInfo.TargetVersion
				reason := req.RollbackInfo.RollbackReason

				log.Printf("[控制端] 收到节点 %s 回滚请求: ConfigID=%d, TargetVersion=%d, Reason=%s",
					nodeID, configID, targetVersion, reason)

				// 获取版本管理器
				if s.versionManager == nil {
					update = &pb.ConfigUpdate{
						NodeId:  nodeID,
						Success: false,
						Message: "版本管理器未初始化",
					}
				} else {
					// 获取目标版本的配置快照
					record, err := s.versionManager.GetVersion(configID, targetVersion)
					if err != nil {
						update = &pb.ConfigUpdate{
							NodeId:  nodeID,
							Success: false,
							Message: fmt.Sprintf("获取版本失败: %v", err),
						}
					} else {
						// 将快照转换为 ProxyConfigRecord
						var configRecord store.ProxyConfigRecord
						if err := json.Unmarshal([]byte(record.ConfigSnapshot), &configRecord); err != nil {
							update = &pb.ConfigUpdate{
								NodeId:  nodeID,
								Success: false,
								Message: fmt.Sprintf("解析配置快照失败: %v", err),
							}
						} else {
							// 更新当前配置（回滚到目标版本）
							configs := []*store.ProxyConfigRecord{&configRecord}
							affected, err := s.BatchUpdateConfigs(configs)
							if err != nil {
								update = &pb.ConfigUpdate{
									NodeId:  nodeID,
									Success: false,
									Message: fmt.Sprintf("回滚配置失败: %v", err),
								}
							} else {
								update = &pb.ConfigUpdate{
									NodeId:  nodeID,
									Success: true,
									Message: fmt.Sprintf("配置 %d 回滚到版本 %d 成功，影响 %d 个配置",
										configID, targetVersion, affected),
								}

								// 生成回滚版本的真实版本号
								var actualVersion int32
								if s.versionManager != nil {
									// 将配置JSON序列化以创建版本快照
									configJSON, _ := json.Marshal(configRecord)
									versionRecord, err := s.versionManager.CaptureVersion(
										configID,
										string(configJSON),
										"restore",
										fmt.Sprintf("回滚到版本 %d", targetVersion),
										fmt.Sprintf("agent_%s", nodeID),
									)
									if err != nil {
										log.Printf("[控制端] 生成回滚版本号失败: %v", err)
										// 使用计算值作为后备
										actualVersion = record.Version + 1
									} else {
										actualVersion = versionRecord.Version
									}
								}

								// 发布回滚事件
								if s.eventBus != nil {
									s.eventBus.Publish(&Event{
										Type:     EventConfigRolledBack,
										ConfigID: configID,
										Data: map[string]interface{}{
											"target_version":  targetVersion,
											"current_version": actualVersion,
											"rollback_by":     "agent_" + nodeID,
											"initiated_by":    req.RollbackInfo.InitiatedBy,
										},
										Timestamp: time.Now().Unix(),
									})
								}

								log.Printf("[控制端] 配置回滚完成: ConfigID=%d, TargetVersion=%d",
									configID, targetVersion)
							}
						}
					}
				}
			} else {
				update = &pb.ConfigUpdate{
					NodeId:  nodeID,
					Success: false,
					Message: "回滚信息无效",
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
		oldStatus := node.Status

		// 检查心跳超时
		if now.Sub(node.LastHeartbeat) > timeout {
			if node.Status != "inactive" {
				node.Status = "inactive"
				log.Printf("[控制端] 节点失联（心跳超时90秒）: %s", nodeID)

				// 触发节点离线事件
				if s.lifecycleManager != nil {
					s.lifecycleManager.OnNodeOffline(nodeID)
				}

				// 广播节点失联
				if s.wsHub != nil && oldStatus != "inactive" {
					go s.broadcastNodeUpdate("node_inactive", nodeID)
				}
			}
		} else {
			// 节点在线，检查健康状态
			if node.Status == "inactive" {
				node.Status = "active"
				log.Printf("[控制端] 节点恢复在线: %s", nodeID)

				// 触发节点上线事件
				if s.lifecycleManager != nil {
					s.lifecycleManager.OnNodeOnline(nodeID)
				}

				if s.wsHub != nil {
					go s.broadcastNodeUpdate("node_active", nodeID)
				}
			}

			// 使用生命周期管理器检查健康状态
			if s.lifecycleManager != nil && node.Health != nil {
				oldHealthStatus, _ := s.lifecycleManager.GetNodeHealthStatus(nodeID)
				newHealthStatus := s.lifecycleManager.CheckNodeHealth(nodeID, node.Health)

				// 状态变化时触发事件
				if oldHealthStatus != newHealthStatus {
					s.lifecycleManager.OnHealthChanged(nodeID, oldHealthStatus, newHealthStatus, node.Health)
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

// loadNodesFromDatabase 从数据库加载节点数据
func (s *ControlServer) loadNodesFromDatabase() {
	if s.store == nil {
		log.Println("[控制端] 未配置数据存储，跳过节点数据加载")
		return
	}

	nodes, err := s.store.NodeDAO().GetAllNodes()
	if err != nil {
		log.Printf("[控制端] 从数据库加载节点数据失败: %v", err)
		return
	}

	log.Printf("[控制端] 从数据库加载 %d 个节点", len(nodes))

	for _, node := range nodes {
		s.nodeRegistry.mu.Lock()
		s.nodeRegistry.nodes[node.NodeID] = &NodeInfo{
			Info: &pb.NodeInfo{
				NodeId:    node.NodeID,
				Hostname:  node.Hostname,
				IpAddress: node.IPAddress,
				Version:   node.Version,
			},
			LastHeartbeat: time.Unix(node.UpdatedAt, 0),
			Status:        node.Status,
			ControlToken:  node.ControlToken,
			Health: &pb.NodeHealth{
				CpuPercent:        0,
				MemoryPercent:     0,
				DiskPercent:       0,
				ActiveConnections: 0,
			},
		}
		s.nodeRegistry.mu.Unlock()
	}

	log.Printf("[控制端] 节点数据加载完成，共 %d 个节点", len(nodes))
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

// broadcastNodeUpdate 广播节点更新
func (s *ControlServer) broadcastNodeUpdate(eventType, nodeID string) {
	if s.wsHub == nil {
		return
	}

	// 获取最新节点状态
	s.nodeRegistry.mu.RLock()
	node, exists := s.nodeRegistry.nodes[nodeID]
	s.nodeRegistry.mu.RUnlock()

	if !exists {
		return
	}

	// 构建广播数据
	data := map[string]interface{}{
		"event":      eventType,
		"node_id":    nodeID,
		"status":     node.Status,
		"hostname":   node.Info.Hostname,
		"ip_address": node.Info.IpAddress,
		"last_seen":  node.LastHeartbeat.Unix(),
	}

	if node.Health != nil {
		data["health"] = map[string]interface{}{
			"cpu_percent":    node.Health.CpuPercent,
			"memory_percent": node.Health.MemoryPercent,
			"disk_percent":   node.Health.DiskPercent,
		}
	}

	// 广播消息
	s.wsHub.Broadcast(data)
}

// BatchGetNodes 批量获取节点状态
func (s *ControlServer) BatchGetNodes(nodeIDs []string) map[string]*NodeInfo {
	result := make(map[string]*NodeInfo)

	for _, nodeID := range nodeIDs {
		if node, exists := s.GetNodeStatus(nodeID); exists {
			result[nodeID] = node
		}
	}

	log.Printf("[控制端] 批量获取节点状态完成，共 %d 个节点", len(result))
	return result
}

// BatchUpdateConfig 批量更新配置
func (s *ControlServer) BatchUpdateConfig(configs map[string]*pb.ProxyConfig) map[string]bool {
	results := make(map[string]bool)

	for nodeID, config := range configs {
		// 验证节点是否存在
		if _, exists := s.GetNodeStatus(nodeID); !exists {
			results[nodeID] = false
			log.Printf("[控制端] 批量更新配置失败: 节点 %s 不存在", nodeID)
			continue
		}

		// 添加到配置管理器
		s.configManager.mu.Lock()
		s.configManager.configs[config.Id] = config
		s.configManager.mu.Unlock()

		results[nodeID] = true
		log.Printf("[控制端] 批量更新配置成功: 节点 %s, 配置 ID=%d", nodeID, config.Id)
	}

	log.Printf("[控制端] 批量更新配置完成，成功 %d 个", len(results))
	return results
}

// BatchRestartNodes 批量重启节点
func (s *ControlServer) BatchRestartNodes(nodeIDs []string) map[string]bool {
	results := make(map[string]bool)

	for _, nodeID := range nodeIDs {
		// 验证节点是否存在
		if _, exists := s.GetNodeStatus(nodeID); !exists {
			results[nodeID] = false
			log.Printf("[控制端] 批量重启失败: 节点 %s 不存在", nodeID)
			continue
		}

		// 记录功能未实现
		// TODO: 实现节点重启逻辑 - 需要向 Agent 发送 gRPC 指令
		// 目前只是记录操作，不实际执行重启
		results[nodeID] = false // 修改为 false，标记功能未实现
		log.Printf("[控制端] 批量重启节点 %s: 功能暂未实现（需 Phase 2 支持）", nodeID)
	}

	log.Printf("[控制端] 批量重启节点完成，共 %d 个节点，结果: %v", len(results), results)
	return results
}

// BatchDeleteConfig 批量删除配置
func (s *ControlServer) BatchDeleteConfig(configIDs []int32) map[int32]bool {
	results := make(map[int32]bool)

	for _, configID := range configIDs {
		// 检查配置是否存在
		s.configManager.mu.RLock()
		_, exists := s.configManager.configs[configID]
		s.configManager.mu.RUnlock()

		if !exists {
			results[configID] = false
			log.Printf("[控制端] 批量删除配置失败: 配置 %d 不存在", configID)
			continue
		}

		// 删除配置
		s.configManager.mu.Lock()
		delete(s.configManager.configs, configID)
		s.configManager.mu.Unlock()

		results[configID] = true
		log.Printf("[控制端] 批量删除配置成功: ID=%d", configID)
	}

	log.Printf("[控制端] 批量删除配置完成，成功 %d 个", len(results))
	return results
}

// PushRollbackToNode 向指定节点推送回滚配置
func (s *ControlServer) PushRollbackToNode(nodeID string, configID int32, targetVersion int32, reason string) error {
	log.Printf("[控制端] 准备向节点 %s 推送回滚配置: ConfigID=%d, TargetVersion=%d",
		nodeID, configID, targetVersion)

	// 检查节点是否存在
	s.nodeRegistry.mu.RLock()
	_, exists := s.nodeRegistry.nodes[nodeID]
	s.nodeRegistry.mu.RUnlock()

	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	var taskID int64

	// 使用数据库持久化任务（如果可用）
	if s.store != nil {
		taskRecord := &store.RollbackTaskRecord{
			NodeID:        nodeID,
			ConfigID:      configID,
			TargetVersion: targetVersion,
			Status:        "pending",
			RetryCount:    0,
			Reason:        reason,
			ErrorMessage:  "",
		}

		id, err := s.store.RollbackTaskDAO().CreateTask(taskRecord)
		if err != nil {
			log.Printf("[控制端] 数据库持久化任务失败，回退到内存队列: %v", err)
			// 降级到内存队列
			taskID = s.addTaskToMemoryQueue(nodeID, configID, targetVersion, reason)
		} else {
			taskID = id
			log.Printf("[控制端] 回滚任务已持久化到数据库: TaskID=%d, NodeID=%s, ConfigID=%d, TargetVersion=%d",
				taskID, nodeID, configID, targetVersion)
		}
	} else {
		// 无数据库，使用内存队列
		taskID = s.addTaskToMemoryQueue(nodeID, configID, targetVersion, reason)
	}

	// 发布任务创建事件
	if s.eventBus != nil {
		s.eventBus.Publish(&Event{
			Type:     EventRollbackTaskCreated,
			ConfigID: configID,
			Data: map[string]interface{}{
				"task_id":        taskID,
				"node_id":        nodeID,
				"target_version": targetVersion,
				"reason":         reason,
			},
			Timestamp: time.Now().Unix(),
		})
	}

	return nil
}

// addTaskToMemoryQueue 添加任务到内存队列（降级方案）
func (s *ControlServer) addTaskToMemoryQueue(nodeID string, configID int32, targetVersion int32, reason string) int64 {
	task := &RollbackTask{
		ConfigID:      configID,
		TargetVersion: targetVersion,
		Reason:        reason,
		CreatedAt:     time.Now(),
	}

	s.nodeRegistry.mu.Lock()
	task.ID = int32(len(s.nodeRegistry.rollbackTasks[nodeID]) + 1)
	s.nodeRegistry.rollbackTasks[nodeID] = append(s.nodeRegistry.rollbackTasks[nodeID], task)
	s.nodeRegistry.mu.Unlock()

	log.Printf("[控制端] 回滚任务已添加到内存队列: NodeID=%s, TaskID=%d, ConfigID=%d, TargetVersion=%d",
		nodeID, task.ID, configID, targetVersion)

	return int64(task.ID)
}

// GetNodesByGroup 批量获取分组节点
func (s *ControlServer) GetNodesByGroup(nodeGroup string) map[string]*NodeInfo {
	result := make(map[string]*NodeInfo)

	// 从store中获取分组节点
	if s.store != nil {
		nodes, err := s.store.NodeDAO().GetNodesByGroup(nodeGroup)
		if err != nil {
			log.Printf("[控制端] 获取分组节点失败: %v", err)
			return result
		}

		for _, node := range nodes {
			// 从内存中获取最新状态
			if nodeInfo, exists := s.GetNodeStatus(node.NodeID); exists {
				result[node.NodeID] = nodeInfo
			}
		}
	} else {
		// 如果没有store，从内存中筛选
		s.nodeRegistry.mu.RLock()
		for nodeID, node := range s.nodeRegistry.nodes {
			// 这里假设在nodeInfo结构体中有分组信息
			// 但当前实现中还没有，需要从store中获取
			// 所以这里是fallback逻辑
			_ = nodeID
			_ = node
		}
		s.nodeRegistry.mu.RUnlock()
	}

	log.Printf("[控制端] 获取分组节点 %s 完成，共 %d 个节点", nodeGroup, len(result))
	return result
}

// BatchUpdateNodesStatus 批量更新节点状态
func (s *ControlServer) BatchUpdateNodesStatus(nodeIDs []string, status string) (int64, error) {
	if s.store == nil {
		log.Println("[控制端] 未配置数据存储，无法批量更新节点状态")
		return 0, fmt.Errorf("数据存储未初始化")
	}

	affected, err := s.store.NodeDAO().BatchUpdateNodesStatus(nodeIDs, status)
	if err != nil {
		return 0, fmt.Errorf("批量更新节点状态失败: %v", err)
	}

	// 更新内存中的节点状态
	s.nodeRegistry.mu.Lock()
	for _, nodeID := range nodeIDs {
		if node, exists := s.nodeRegistry.nodes[nodeID]; exists {
			node.Status = status
		}
	}
	s.nodeRegistry.mu.Unlock()

	log.Printf("[控制端] 批量更新节点状态完成，影响 %d 个节点", affected)
	return affected, nil
}

// BatchCreateConfigs 批量创建配置
func (s *ControlServer) BatchCreateConfigs(configs []*store.ProxyConfigRecord) (int64, error) {
	if s.store == nil {
		log.Println("[控制端] 未配置数据存储，无法批量创建配置")
		return 0, fmt.Errorf("数据存储未初始化")
	}

	affected, err := s.store.ProxyConfigDAO().BatchCreateConfigs(configs)
	if err != nil {
		return 0, fmt.Errorf("批量创建配置失败: %v", err)
	}

	// 版本捕获钩子：为每个新创建的配置创建初始版本快照
	if s.versionManager != nil {
		for _, config := range configs {
			// 将配置序列化为JSON
			configJSON, err := json.Marshal(config)
			if err != nil {
				log.Printf("[控制端] 配置 %d 序列化失败，跳过版本捕获: %v", config.ID, err)
				continue
			}

			// 创建初始版本快照
			_, err = s.versionManager.CaptureVersion(
				config.ID,
				string(configJSON),
				"create",
				fmt.Sprintf("创建配置: %s", config.Name),
				"system",
			)
			if err != nil {
				log.Printf("[控制端] 配置 %d 版本捕获失败: %v", config.ID, err)
				// 版本捕获失败不影响配置创建流程，只记录日志
			}
		}
	} else {
		log.Println("[控制端] 版本管理器未初始化，跳过版本捕获")
	}

	// 更新内存中的配置
	for _, config := range configs {
		pbConfig := &pb.ProxyConfig{
			Id:           config.ID,
			Name:         config.Name,
			OutboundType: config.OutboundType,
			InboundPort:  config.InboundPort,
		}
		s.configManager.mu.Lock()
		s.configManager.configs[config.ID] = pbConfig
		s.configManager.mu.Unlock()
	}

	log.Printf("[控制端] 批量创建配置完成，影响 %d 个配置", affected)
	return affected, nil
}

// BatchUpdateConfigs 批量更新配置（使用ProxyConfigRecord）
func (s *ControlServer) BatchUpdateConfigs(configs []*store.ProxyConfigRecord) (int64, error) {
	if s.store == nil {
		log.Println("[控制端] 未配置数据存储，无法批量更新配置")
		return 0, fmt.Errorf("数据存储未初始化")
	}

	// 将 ProxyConfigRecord 转换为 map[int32]*ProxyConfigRecord
	configMap := make(map[int32]*store.ProxyConfigRecord)
	for _, config := range configs {
		configMap[config.ID] = config
	}

	affected, err := s.store.ProxyConfigDAO().BatchUpdateConfigs(configMap)
	if err != nil {
		return 0, fmt.Errorf("批量更新配置失败: %v", err)
	}

	// 版本捕获钩子：为每个更新的配置创建版本快照
	if s.versionManager != nil {
		for _, config := range configs {
			// 将配置序列化为JSON
			configJSON, err := json.Marshal(config)
			if err != nil {
				log.Printf("[控制端] 配置 %d 序列化失败，跳过版本捕获: %v", config.ID, err)
				continue
			}

			// 创建版本快照
			_, err = s.versionManager.CaptureVersion(
				config.ID,
				string(configJSON),
				"update",
				fmt.Sprintf("批量更新配置: %s", config.Name),
				"system",
			)
			if err != nil {
				log.Printf("[控制端] 配置 %d 版本捕获失败: %v", config.ID, err)
				// 版本捕获失败不影响配置更新流程，只记录日志
			}
		}
	} else {
		log.Println("[控制端] 版本管理器未初始化，跳过版本捕获")
	}

	// 更新内存中的配置
	for _, config := range configs {
		pbConfig := &pb.ProxyConfig{
			Id:           config.ID,
			Name:         config.Name,
			OutboundType: config.OutboundType,
			InboundPort:  config.InboundPort,
		}
		s.configManager.mu.Lock()
		s.configManager.configs[config.ID] = pbConfig
		s.configManager.mu.Unlock()
	}

	log.Printf("[控制端] 批量更新配置完成，影响 %d 个配置", affected)
	return affected, nil
}

// GetLifecycleManager 获取生命周期管理器
func (s *ControlServer) GetLifecycleManager() *LifecycleManager {
	return s.lifecycleManager
}

// GetVersionManager 获取版本管理器
func (s *ControlServer) GetVersionManager() *VersionManager {
	return s.versionManager
}

// GetEventBus 获取事件总线
func (s *ControlServer) GetEventBus() *EventBus {
	return s.eventBus
}

// GetStore 获取数据存储
func (s *ControlServer) GetStore() *store.Store {
	return s.store
}
