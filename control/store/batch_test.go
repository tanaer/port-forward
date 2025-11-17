package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeDAO_BatchGetNodesByIDs(t *testing.T) {
	store, err := NewMemoryStore()
	require.NoError(t, err)
	defer store.Close()

	// 创建测试节点
	nodeIDs := []string{"node-1", "node-2", "node-3"}
	for _, nodeID := range nodeIDs {
		nodeRecord := &NodeRecord{
			NodeID:       nodeID,
			Hostname:     "hostname-" + nodeID,
			IPAddress:    "192.168.1.100",
			Version:      "2.0.0",
			Status:       "active",
			ControlToken: "test-token",
			CreatedAt:    time.Now().Unix(),
			UpdatedAt:    time.Now().Unix(),
		}
		err = store.NodeDAO().CreateNode(nodeRecord)
		require.NoError(t, err)
	}

	// 测试批量获取
	nodes, err := store.NodeDAO().BatchGetNodesByIDs(nodeIDs)
	require.NoError(t, err)
	assert.Equal(t, 3, len(nodes))

	// 验证节点数据
	nodeMap := make(map[string]*NodeRecord)
	for _, node := range nodes {
		nodeMap[node.NodeID] = node
	}

	for _, nodeID := range nodeIDs {
		assert.Contains(t, nodeMap, nodeID)
		assert.Equal(t, "hostname-"+nodeID, nodeMap[nodeID].Hostname)
		assert.Equal(t, "active", nodeMap[nodeID].Status)
	}
}

func TestNodeDAO_BatchUpdateNodesStatus(t *testing.T) {
	store, err := NewMemoryStore()
	require.NoError(t, err)
	defer store.Close()

	// 创建测试节点
	nodeIDs := []string{"node-1", "node-2", "node-3"}
	for _, nodeID := range nodeIDs {
		nodeRecord := &NodeRecord{
			NodeID:       nodeID,
			Hostname:     "hostname-" + nodeID,
			IPAddress:    "192.168.1.100",
			Version:      "2.0.0",
			Status:       "active",
			ControlToken: "test-token",
			CreatedAt:    time.Now().Unix(),
			UpdatedAt:    time.Now().Unix(),
		}
		err = store.NodeDAO().CreateNode(nodeRecord)
		require.NoError(t, err)
	}

	// 批量更新状态
	affected, err := store.NodeDAO().BatchUpdateNodesStatus(nodeIDs, "maintenance")
	require.NoError(t, err)
	assert.Equal(t, int64(3), affected)

	// 验证状态更新
	nodes, err := store.NodeDAO().BatchGetNodesByIDs(nodeIDs)
	require.NoError(t, err)
	for _, node := range nodes {
		assert.Equal(t, "maintenance", node.Status)
	}
}

func TestProxyConfigDAO_BatchCreateConfigs(t *testing.T) {
	store, err := NewMemoryStore()
	require.NoError(t, err)
	defer store.Close()

	// 创建测试节点
	nodeRecord := &NodeRecord{
		NodeID:       "node-1",
		Hostname:     "hostname-node-1",
		IPAddress:    "192.168.1.100",
		Version:      "2.0.0",
		Status:       "active",
		ControlToken: "test-token",
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
	err = store.NodeDAO().CreateNode(nodeRecord)
	require.NoError(t, err)

	// 创建测试配置
	configs := []*ProxyConfigRecord{
		{
			NodeID:       "node-1",
			Name:         "配置-1",
			OutboundType: "tcp",
			ConfigJSON:   `{"listen_port": 8080}`,
			Version:      1,
		},
		{
			NodeID:       "node-1",
			Name:         "配置-2",
			OutboundType: "udp",
			ConfigJSON:   `{"port": 9090}`,
			Version:      1,
		},
	}

	// 批量创建
	affected, err := store.ProxyConfigDAO().BatchCreateConfigs(configs)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	// 验证配置数据
	storedConfigs, err := store.ProxyConfigDAO().GetConfigsByNodeID("node-1")
	require.NoError(t, err)
	assert.Equal(t, 2, len(storedConfigs))

	// 验证 InboundPort 解析
	for _, config := range storedConfigs {
		if config.Name == "配置-1" {
			assert.Equal(t, int32(8080), config.InboundPort)
		} else if config.Name == "配置-2" {
			assert.Equal(t, int32(9090), config.InboundPort)
		}
	}
}

func TestProxyConfigDAO_BatchDeleteConfigs(t *testing.T) {
	store, err := NewMemoryStore()
	require.NoError(t, err)
	defer store.Close()

	// 创建测试节点
	nodeRecord := &NodeRecord{
		NodeID:       "node-1",
		Hostname:     "hostname-node-1",
		IPAddress:    "192.168.1.100",
		Version:      "2.0.0",
		Status:       "active",
		ControlToken: "test-token",
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
	err = store.NodeDAO().CreateNode(nodeRecord)
	require.NoError(t, err)

	// 创建测试配置
	configs := []*ProxyConfigRecord{
		{
			NodeID:       "node-1",
			Name:         "配置-1",
			OutboundType: "tcp",
			ConfigJSON:   `{"listen_port": 8080}`,
			Version:      1,
		},
		{
			NodeID:       "node-1",
			Name:         "配置-2",
			OutboundType: "udp",
			ConfigJSON:   `{"port": 9090}`,
			Version:      1,
		},
	}

	_, err = store.ProxyConfigDAO().BatchCreateConfigs(configs)
	require.NoError(t, err)

	// 批量删除
	configIDs := []int32{1, 2}
	affected, err := store.ProxyConfigDAO().BatchDeleteConfigs(configIDs)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	// 验证删除
	storedConfigs, err := store.ProxyConfigDAO().GetConfigsByNodeID("node-1")
	require.NoError(t, err)
	assert.Equal(t, 0, len(storedConfigs))
}

func TestParseInboundPort(t *testing.T) {
	tests := []struct {
		name     string
		configJSON string
		expected int32
		hasError bool
	}{
		{
			name:     "listen_port 字段",
			configJSON: `{"listen_port": 8080}`,
			expected: 8080,
			hasError: false,
		},
		{
			name:     "port 字段",
			configJSON: `{"port": 9090}`,
			expected: 9090,
			hasError: false,
		},
		{
			name:     "空配置",
			configJSON: "",
			expected: 0,
			hasError: true,
		},
		{
			name:     "无端口字段",
			configJSON: `{"host": "localhost"}`,
			expected: 0,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := parseInboundPort(tt.configJSON)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, port)
			}
		})
	}
}
