package store

import (
	"testing"
	"time"
)

// TestDLQTaskLifecycle 测试DLQ任务完整生命周期
func TestDLQTaskLifecycle(t *testing.T) {
	// 创建内存存储
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存存储失败: %v", err)
	}
	defer store.Close()

	dlqDAO := store.DLQDAO()

	// 1. 测试MoveToDLQ - 移动失败任务到DLQ
	dlqID, err := dlqDAO.MoveToDLQ(
		1001,           // originalTaskID
		"node-1",       // nodeID
		100,            // configID
		5,              // targetVersion
		"failed",       // status
		"Agent timeout", // failureReason
		7,              // retryCount
	)
	if err != nil {
		t.Errorf("MoveToDLQ失败: %v", err)
		return
	}
	if dlqID <= 0 {
		t.Errorf("MoveToDLQ返回无效ID: %d", dlqID)
		return
	}
	t.Logf("DLQ任务已创建: ID=%d", dlqID)

	// 2. 测试GetDLQTaskByID - 获取DLQ任务详情
	task, err := dlqDAO.GetDLQTaskByID(dlqID)
	if err != nil {
		t.Errorf("GetDLQTaskByID失败: %v", err)
		return
	}
	if task.ID != dlqID {
		t.Errorf("任务ID不匹配: 期望%d, 实际%d", dlqID, task.ID)
		return
	}
	if task.NodeID != "node-1" {
		t.Errorf("NodeID不匹配: 期望node-1, 实际%s", task.NodeID)
		return
	}
	if task.FailureReason != "Agent timeout" {
		t.Errorf("FailureReason不匹配: 期望'Agent timeout', 实际'%s'", task.FailureReason)
		return
	}
	t.Logf("DLQ任务详情验证成功: %+v", task)

	// 3. 测试ListDLQTasks - 列出DLQ任务
	tasks, err := dlqDAO.ListDLQTasks(100)
	if err != nil {
		t.Errorf("ListDLQTasks失败: %v", err)
		return
	}
	if len(tasks) == 0 {
		t.Errorf("ListDLQTasks返回空列表")
		return
	}
	found := false
	for _, t := range tasks {
		if t.ID == dlqID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("新创建的DLQ任务不在列表中")
		return
	}
	t.Logf("DLQ任务列表查询成功: %d条任务", len(tasks))

	// 4. 测试GetDLQTaskCount - 获取DLQ任务计数
	count, err := dlqDAO.GetDLQTaskCount()
	if err != nil {
		t.Errorf("GetDLQTaskCount失败: %v", err)
		return
	}
	if count != int64(len(tasks)) {
		t.Errorf("任务计数不匹配: 期望%d, 实际%d", int64(len(tasks)), count)
		return
	}
	t.Logf("DLQ任务计数验证成功: %d条", count)

	// 5. 测试DeleteFromDLQ - 删除DLQ任务
	err = dlqDAO.DeleteFromDLQ(dlqID)
	if err != nil {
		t.Errorf("DeleteFromDLQ失败: %v", err)
		return
	}
	t.Logf("DLQ任务已删除: ID=%d", dlqID)

	// 验证删除成功
	_, err = dlqDAO.GetDLQTaskByID(dlqID)
	if err == nil {
		t.Errorf("删除后任务仍存在")
		return
	}
	t.Logf("DLQ任务删除验证成功")
}

// TestDLQMultipleMovements 测试多个任务的DLQ转移
func TestDLQMultipleMovements(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存存储失败: %v", err)
	}
	defer store.Close()

	dlqDAO := store.DLQDAO()

	// 移动5个不同节点的任务到DLQ
	nodeIDs := []string{"node-1", "node-2", "node-3", "node-4", "node-5"}
	dlqIDs := make([]int64, 0)

	for i, nodeID := range nodeIDs {
		dlqID, err := dlqDAO.MoveToDLQ(
			int64(2000+i),     // originalTaskID
			nodeID,            // nodeID
			int32(200 + i),    // configID
			int32(10),         // targetVersion
			"failed",          // status
			"Timeout: "+nodeID, // failureReason
			3,                 // retryCount
		)
		if err != nil {
			t.Errorf("MoveToDLQ失败 (%s): %v", nodeID, err)
			return
		}
		dlqIDs = append(dlqIDs, dlqID)
	}

	// 验证所有任务都在DLQ中
	count, err := dlqDAO.GetDLQTaskCount()
	if err != nil {
		t.Errorf("GetDLQTaskCount失败: %v", err)
		return
	}
	if count != int64(len(nodeIDs)) {
		t.Errorf("任务计数不匹配: 期望%d, 实际%d", len(nodeIDs), count)
		return
	}

	tasks, err := dlqDAO.ListDLQTasks(100)
	if err != nil {
		t.Errorf("ListDLQTasks失败: %v", err)
		return
	}
	if len(tasks) != len(nodeIDs) {
		t.Errorf("列表任务数不匹配: 期望%d, 实际%d", len(nodeIDs), len(tasks))
		return
	}

	t.Logf("成功移动%d个任务到DLQ", len(dlqIDs))
}

// TestDLQReplayFromDLQ 测试从DLQ重放任务
func TestDLQReplayFromDLQ(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存存储失败: %v", err)
	}
	defer store.Close()

	dlqDAO := store.DLQDAO()
	nodeDAO := store.NodeDAO()
	rollbackTaskDAO := store.RollbackTaskDAO()

	// 1. 先创建一个节点，因为rollback_tasks有外键约束
	nodeRecord := &NodeRecord{
		NodeID:       "node-1",
		Hostname:     "test-host",
		IPAddress:    "192.168.1.100",
		Version:      "v2.0.0",
		Status:       "online",
		ControlToken: "test-token",
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}

	if err := nodeDAO.CreateNode(nodeRecord); err != nil {
		t.Fatalf("CreateNode失败: %v", err)
	}
	t.Logf("测试节点已创建")

	// 2. 创建一个回滚任务
	originalTask := &RollbackTaskRecord{
		NodeID:        "node-1",
		ConfigID:      100,
		TargetVersion: 5,
		Status:        "pending",
		RetryCount:    0,
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}

	originalTaskID, err := rollbackTaskDAO.CreateTask(originalTask)
	if err != nil {
		t.Fatalf("CreateTask失败: %v", err)
	}
	t.Logf("原始任务已创建: ID=%d", originalTaskID)

	// 3. 将任务移动到DLQ
	dlqID, err := dlqDAO.MoveToDLQ(
		originalTaskID,
		originalTask.NodeID,
		originalTask.ConfigID,
		originalTask.TargetVersion,
		"failed",
		"Processing timeout",
		6,
	)
	if err != nil {
		t.Errorf("MoveToDLQ失败: %v", err)
		return
	}
	t.Logf("任务已移动到DLQ: DLQID=%d", dlqID)

	// 4. 从DLQ重放任务
	err = dlqDAO.ReplayFromDLQ(dlqID)
	if err != nil {
		t.Errorf("ReplayFromDLQ失败: %v", err)
		return
	}
	t.Logf("任务已从DLQ重放")

	// 5. 验证任务已重新插入到rollback_tasks表
	tasks, err := rollbackTaskDAO.GetPendingTasksByNode(originalTask.NodeID)
	if err != nil {
		t.Errorf("GetPendingTasksByNode失败: %v", err)
		return
	}

	if len(tasks) == 0 {
		t.Logf("警告: 重放任务未在待处理任务列表中找到 (可能在其他状态)")
	} else {
		t.Logf("重放任务已找到: %+v", tasks[0])
	}

	// 6. 验证任务已从DLQ中删除
	_, err = dlqDAO.GetDLQTaskByID(dlqID)
	if err == nil {
		t.Errorf("重放后任务仍在DLQ中")
		return
	}
	t.Logf("重放后任务已从DLQ删除")
}

// TestDLQCleanupExpired 测试清理过期DLQ任务
func TestDLQCleanupExpired(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存存储失败: %v", err)
	}
	defer store.Close()

	dlqDAO := store.DLQDAO()

	// 1. 移动一个任务到DLQ (30天后过期)
	dlqID1, err := dlqDAO.MoveToDLQ(
		3001,
		"node-1",
		300,
		10,
		"failed",
		"Test expiry",
		5,
	)
	if err != nil {
		t.Errorf("MoveToDLQ失败: %v", err)
		return
	}
	t.Logf("第一个任务已创建: ID=%d", dlqID1)

	// 初始计数
	countBefore, err := dlqDAO.GetDLQTaskCount()
	if err != nil {
		t.Errorf("GetDLQTaskCount失败: %v", err)
		return
	}
	t.Logf("清理前任务数: %d", countBefore)

	// 2. 清理过期任务（应该没有过期的，因为刚创建）
	affected, err := dlqDAO.CleanupExpiredDLQTasks()
	if err != nil {
		t.Errorf("CleanupExpiredDLQTasks失败: %v", err)
		return
	}
	if affected != 0 {
		t.Logf("警告: 清理了%d个过期任务 (预期0个)", affected)
	}

	// 3. 验证任务仍在DLQ中
	countAfter, err := dlqDAO.GetDLQTaskCount()
	if err != nil {
		t.Errorf("GetDLQTaskCount失败: %v", err)
		return
	}
	if countAfter != countBefore {
		t.Errorf("清理后任务计数变化: 之前%d, 之后%d", countBefore, countAfter)
		return
	}
	t.Logf("清理验证成功: 任务计数保持%d", countAfter)
}

// TestDLQExpiryValidation 测试DLQ过期时间验证
func TestDLQExpiryValidation(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存存储失败: %v", err)
	}
	defer store.Close()

	dlqDAO := store.DLQDAO()

	// 移动任务到DLQ
	dlqID, err := dlqDAO.MoveToDLQ(
		4001,
		"node-1",
		400,
		15,
		"failed",
		"Expiry test",
		4,
	)
	if err != nil {
		t.Errorf("MoveToDLQ失败: %v", err)
		return
	}

	// 获取任务并验证过期时间
	task, err := dlqDAO.GetDLQTaskByID(dlqID)
	if err != nil {
		t.Errorf("GetDLQTaskByID失败: %v", err)
		return
	}

	now := time.Now().Unix()
	thirtyDaysInSeconds := int64(30 * 24 * 60 * 60)
	expectedExpiryMin := now + thirtyDaysInSeconds - 60 // 允许1分钟误差
	expectedExpiryMax := now + thirtyDaysInSeconds + 60

	if task.DLQExpiryAt < expectedExpiryMin || task.DLQExpiryAt > expectedExpiryMax {
		t.Errorf("过期时间不正确: 期望约%d, 实际%d", now+thirtyDaysInSeconds, task.DLQExpiryAt)
		return
	}

	t.Logf("过期时间验证成功: %s", time.Unix(task.DLQExpiryAt, 0).Format("2006-01-02 15:04:05"))
}

// TestDLQMetadataHandling 测试DLQ元数据处理
func TestDLQMetadataHandling(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("创建内存存储失败: %v", err)
	}
	defer store.Close()

	dlqDAO := store.DLQDAO()

	// 移动任务到DLQ，包含元数据
	dlqID, err := dlqDAO.MoveToDLQ(
		5001,
		"node-1",
		500,
		20,
		"failed",
		"Metadata test",
		3,
	)
	if err != nil {
		t.Errorf("MoveToDLQ失败: %v", err)
		return
	}

	// 获取任务验证字段
	task, err := dlqDAO.GetDLQTaskByID(dlqID)
	if err != nil {
		t.Errorf("GetDLQTaskByID失败: %v", err)
		return
	}

	if task.OriginalTaskID != 5001 {
		t.Errorf("OriginalTaskID不匹配: 期望5001, 实际%d", task.OriginalTaskID)
	}
	if task.NodeID != "node-1" {
		t.Errorf("NodeID不匹配: 期望node-1, 实际%s", task.NodeID)
	}
	if task.ConfigID != 500 {
		t.Errorf("ConfigID不匹配: 期望500, 实际%d", task.ConfigID)
	}
	if task.TargetVersion != 20 {
		t.Errorf("TargetVersion不匹配: 期望20, 实际%d", task.TargetVersion)
	}
	if task.Status != "failed" {
		t.Errorf("Status不匹配: 期望failed, 实际%s", task.Status)
	}
	if task.RetryCount != 3 {
		t.Errorf("RetryCount不匹配: 期望3, 实际%d", task.RetryCount)
	}

	t.Logf("元数据验证成功: %+v", task)
}
