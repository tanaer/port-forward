package conf

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ProxyImportJobStatus 导入任务状态
type ProxyImportJobStatus string

const (
	ProxyImportQueued    ProxyImportJobStatus = "queued"
	ProxyImportRunning   ProxyImportJobStatus = "running"
	ProxyImportSucceeded ProxyImportJobStatus = "succeeded"
	ProxyImportFailed    ProxyImportJobStatus = "failed"
)

// ProxyImportJob 导入任务状态信息
type ProxyImportJob struct {
	ID         string               `json:"id"`
	Status     ProxyImportJobStatus `json:"status"`
	Total      int                  `json:"total"`
	Imported   int                  `json:"imported"`
	Failed     int                  `json:"failed"`
	Error      string               `json:"error,omitempty"`
	CreatedAt  time.Time            `json:"createdAt"`
	StartedAt  *time.Time           `json:"startedAt,omitempty"`
	FinishedAt *time.Time           `json:"finishedAt,omitempty"`
	UpdatedAt  *time.Time           `json:"updatedAt,omitempty"`
}

// ProxyImportJobStore 管理导入任务状态
type ProxyImportJobStore struct {
	mu   sync.RWMutex
	jobs map[string]*ProxyImportJob
}

// ProxyImportJobs 全局导入任务存储
var ProxyImportJobs = NewProxyImportJobStore()

var proxyImportJobCounter uint64

// NewProxyImportJobStore 创建导入任务存储
func NewProxyImportJobStore() *ProxyImportJobStore {
	return &ProxyImportJobStore{
		jobs: make(map[string]*ProxyImportJob),
	}
}

// Create 创建新导入任务
func (store *ProxyImportJobStore) Create() ProxyImportJob {
	now := time.Now()
	job := &ProxyImportJob{
		ID:        nextProxyImportJobID(),
		Status:    ProxyImportQueued,
		CreatedAt: now,
	}

	store.mu.Lock()
	store.jobs[job.ID] = job
	store.mu.Unlock()

	return cloneProxyImportJob(job)
}

// Get 获取导入任务信息
func (store *ProxyImportJobStore) Get(id string) (ProxyImportJob, bool) {
	store.mu.RLock()
	job, ok := store.jobs[id]
	store.mu.RUnlock()
	if !ok {
		return ProxyImportJob{}, false
	}

	return cloneProxyImportJob(job), true
}

// Update 更新导入任务信息
func (store *ProxyImportJobStore) Update(id string, fn func(job *ProxyImportJob)) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	job, ok := store.jobs[id]
	if !ok {
		return false
	}

	fn(job)
	now := time.Now()
	job.UpdatedAt = &now
	return true
}

func nextProxyImportJobID() string {
	return fmt.Sprintf("import-%d-%d", time.Now().Unix(), atomic.AddUint64(&proxyImportJobCounter, 1))
}

func cloneProxyImportJob(job *ProxyImportJob) ProxyImportJob {
	cloned := *job
	if job.StartedAt != nil {
		t := *job.StartedAt
		cloned.StartedAt = &t
	}
	if job.FinishedAt != nil {
		t := *job.FinishedAt
		cloned.FinishedAt = &t
	}
	if job.UpdatedAt != nil {
		t := *job.UpdatedAt
		cloned.UpdatedAt = &t
	}
	return cloned
}
