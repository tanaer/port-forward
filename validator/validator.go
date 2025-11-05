package validator

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"goForward/conf"
)

// ValidationRule 验证规则接口
type ValidationRule interface {
	Validate(config *conf.ConnectionStats) error
}

// ConfigValidator 配置验证器
type ConfigValidator struct {
	rules []ValidationRule
}

// NewConfigValidator 创建新的配置验证器
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		rules: []ValidationRule{
			&PortRule{},
			&ProtocolRule{},
			&AddressRule{},
			&TimeoutRule{},
			&IPListRule{},
		},
	}
}

// Validate 验证配置
func (cv *ConfigValidator) Validate(config *conf.ConnectionStats) error {
	for _, rule := range cv.rules {
		if err := rule.Validate(config); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}
	return nil
}

// PortRule 端口验证规则
type PortRule struct{}

func (r *PortRule) Validate(config *conf.ConnectionStats) error {
	// 验证本地端口
	localPort, err := strconv.Atoi(config.LocalPort)
	if err != nil || localPort < 1 || localPort > 65535 {
		return fmt.Errorf("invalid local port: %s (must be 1-65535)", config.LocalPort)
	}

	// 验证远程端口
	remotePort, err := strconv.Atoi(config.RemotePort)
	if err != nil || remotePort < 1 || remotePort > 65535 {
		return fmt.Errorf("invalid remote port: %s (must be 1-65535)", config.RemotePort)
	}

	return nil
}

// ProtocolRule 协议验证规则
type ProtocolRule struct{}

func (r *ProtocolRule) Validate(config *conf.ConnectionStats) error {
	protocol := strings.ToLower(config.Protocol)
	if protocol != "tcp" && protocol != "udp" {
		return fmt.Errorf("invalid protocol: %s (must be tcp or udp)", config.Protocol)
	}
	return nil
}

// AddressRule 地址验证规则
type AddressRule struct{}

func (r *AddressRule) Validate(config *conf.ConnectionStats) error {
	if config.RemoteAddr == "" {
		return fmt.Errorf("remote address cannot be empty")
	}

	// 验证是否为有效的IP地址或域名
	if net.ParseIP(config.RemoteAddr) == nil {
		// 不是IP，检查是否为有效域名
		if !isValidDomain(config.RemoteAddr) {
			return fmt.Errorf("invalid remote address: %s", config.RemoteAddr)
		}
	}

	return nil
}

// TimeoutRule 超时验证规则
type TimeoutRule struct{}

func (r *TimeoutRule) Validate(config *conf.ConnectionStats) error {
	if config.OutTime < 0 || config.OutTime > 3600 {
		return fmt.Errorf("invalid timeout: %d (must be 0-3600 seconds)", config.OutTime)
	}
	return nil
}

// IPListRule IP列表验证规则
type IPListRule struct{}

func (r *IPListRule) Validate(config *conf.ConnectionStats) error {
	// 验证白名单
	if config.Whitelist != "" {
		if err := validateIPList(config.Whitelist); err != nil {
			return fmt.Errorf("invalid whitelist: %w", err)
		}
	}

	// 验证黑名单
	if config.Blacklist != "" {
		if err := validateIPList(config.Blacklist); err != nil {
			return fmt.Errorf("invalid blacklist: %w", err)
		}
	}

	return nil
}

// validateIPList 验证IP列表格式
func validateIPList(ipList string) error {
	items := strings.Split(ipList, ";")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		// 检查是否为CIDR格式
		if strings.Contains(item, "/") {
			_, _, err := net.ParseCIDR(item)
			if err != nil {
				return fmt.Errorf("invalid CIDR: %s", item)
			}
		} else {
			// 检查是否为有效IP
			if net.ParseIP(item) == nil {
				return fmt.Errorf("invalid IP: %s", item)
			}
		}
	}
	return nil
}

// isValidDomain 简单的域名验证
func isValidDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// 域名不能以点开头或结尾
	if domain[0] == '.' || domain[len(domain)-1] == '.' {
		return false
	}

	// 简单检查：包含至少一个点，且不包含非法字符
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}

	for _, part := range parts {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
	}

	return true
}
