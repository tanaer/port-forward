package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// APIClient API客户端
type APIClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewAPIClient 创建新的API客户端
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		Timeout: 10 * time.Second,
	}
}

// NewAPIClientWithToken 创建带Token的API客户端
func NewAPIClientWithToken(baseURL, token string) *APIClient {
	return &APIClient{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Timeout:    10 * time.Second,
	}
}

// addAuthHeader 添加认证头到请求
func (c *APIClient) addAuthHeader(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("X-API-Token", c.Token)
	}
}


// BatchStartRequest 批量启动请求
type BatchStartRequest struct {
	IDs []int `json:"ids"`
}

// BatchStopRequest 批量停止请求
type BatchStopRequest struct {
	IDs []int `json:"ids"`
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []int `json:"ids"`
}

// BatchResponse 批量响应
type BatchResponse struct {
	Success []int              `json:"success"`
	Failed  map[int]string     `json:"failed"`
	Message string             `json:"message"`
}

// StatusQueryRequest 状态查询请求
type StatusQueryRequest struct {
	IDs []int `json:"ids"`
}

// StatusQueryResponse 状态查询响应
type StatusQueryResponse struct {
	Statuses []ProxyStatus `json:"statuses"`
}

// ProxyStatus 代理状态
type ProxyStatus struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Status     int    `json:"status"`
	InboundPort int   `json:"inbound_port"`
	OutboundType string `json:"outbound_type"`
}

// TrafficStats 流量统计
type TrafficStats struct {
	Total       int64  `json:"total"`
	Active      int64  `json:"active"`
	TotalTraffic int64 `json:"total_traffic"`
}

// ProxyInfo 代理信息
type ProxyInfo struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Status       int    `json:"status"`
	InboundPort  int    `json:"inbound_port"`
	OutboundType string `json:"outbound_type"`
	TotalBytes   uint64 `json:"total_bytes"`
}

// NetworkTestResult 网络测试结果
type NetworkTestResult struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
	OK   bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// DiagnosisResult 诊断结果
type DiagnosisResult struct {
	Ports      []PortStatus      `json:"ports"`
	Proxies    []ProxyInfo       `json:"proxies"`
	Database   TrafficStats      `json:"database"`
	Network    []NetworkTestResult `json:"network"`
}

// PortStatus 端口状态
type PortStatus struct {
	Port       int  `json:"port"`
	ProxyID    int  `json:"proxy_id"`
	InUse      bool `json:"in_use"`
	MultipleProxies bool `json:"multiple_proxies"`
}

// BatchStart 批量启动代理
func (c *APIClient) BatchStart(ids []int) (*BatchResponse, error) {
	req := BatchStartRequest{IDs: ids}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/batch/start", c.BaseURL), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.addAuthHeader(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result BatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BatchStop 批量停止代理
func (c *APIClient) BatchStop(ids []int) (*BatchResponse, error) {
	req := BatchStopRequest{IDs: ids}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/batch/stop", c.BaseURL), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.addAuthHeader(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result BatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BatchDelete 批量删除代理
func (c *APIClient) BatchDelete(ids []int) (*BatchResponse, error) {
	req := BatchDeleteRequest{IDs: ids}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/batch/delete", c.BaseURL), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.addAuthHeader(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result BatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProxyList 获取代理列表
func (c *APIClient) GetProxyList() ([]ProxyInfo, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/proxy/list", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	c.addAuthHeader(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []ProxyInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetTrafficStats 获取流量统计
func (c *APIClient) GetTrafficStats() (*TrafficStats, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/stats/system", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	c.addAuthHeader(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result TrafficStats
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDiagnosis 获取诊断结果
func (c *APIClient) GetDiagnosis() (*DiagnosisResult, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/diagnosis", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	c.addAuthHeader(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result DiagnosisResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
