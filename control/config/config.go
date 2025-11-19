package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
)

// Config 系统总体配置
type Config struct {
	// 服务配置
	Server ServerConfig `yaml:"server"`

	// 回滚系统配置
	Rollback RollbackConfig `yaml:"rollback"`

	// 指标配置
	Metrics MetricsConfig `yaml:"metrics"`

	// 日志配置
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port     string `yaml:"port"`
	Password string `yaml:"password"`
}

// RollbackConfig 回滚系统配置
type RollbackConfig struct {
	Enabled             bool          `yaml:"enabled"`
	MaxRetries          int           `yaml:"max_retries"`
	ProcessingTimeout   time.Duration `yaml:"processing_timeout"`
	StalledScanInterval time.Duration `yaml:"stalled_scan_interval"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled        bool          `yaml:"enabled"`
	Port           string        `yaml:"port"`
	ScrapeInterval time.Duration `yaml:"scrape_interval"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:     "8889",
			Password: "",
		},
		Rollback: RollbackConfig{
			Enabled:             true,
			MaxRetries:          5,
			ProcessingTimeout:   600 * time.Second,
			StalledScanInterval: 60 * time.Second,
		},
		Metrics: MetricsConfig{
			Enabled:        true,
			Port:           "9090",
			ScrapeInterval: 15 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
	}
}

// LoadFromEnv 从环境变量加载配置
func (c *Config) LoadFromEnv() {
	// 服务配置
	if port := os.Getenv("GOFORWARD_PORT"); port != "" {
		c.Server.Port = port
	}
	if pass := os.Getenv("GOFORWARD_PASSWORD"); pass != "" {
		c.Server.Password = pass
	}

	// 回滚系统配置
	if enabled := os.Getenv("GOFORWARD_ROLLBACK_ENABLED"); enabled != "" {
		c.Rollback.Enabled = parseBoolean(enabled)
	}
	if maxRetries := os.Getenv("GOFORWARD_MAX_RETRIES"); maxRetries != "" {
		if v, err := strconv.Atoi(maxRetries); err == nil {
			c.Rollback.MaxRetries = v
		}
	}
	if timeout := os.Getenv("GOFORWARD_PROCESSING_TIMEOUT"); timeout != "" {
		if v, err := time.ParseDuration(timeout); err == nil {
			c.Rollback.ProcessingTimeout = v
		}
	}
	if scanInterval := os.Getenv("GOFORWARD_STALLED_SCAN_INTERVAL"); scanInterval != "" {
		if v, err := time.ParseDuration(scanInterval); err == nil {
			c.Rollback.StalledScanInterval = v
		}
	}

	// 指标配置
	if metricsEnabled := os.Getenv("GOFORWARD_METRICS_ENABLED"); metricsEnabled != "" {
		c.Metrics.Enabled = parseBoolean(metricsEnabled)
	}
	if metricsPort := os.Getenv("GOFORWARD_METRICS_PORT"); metricsPort != "" {
		c.Metrics.Port = metricsPort
	}

	// 日志配置
	if logLevel := os.Getenv("GOFORWARD_LOG_LEVEL"); logLevel != "" {
		c.Logging.Level = logLevel
	}
	if logFormat := os.Getenv("GOFORWARD_LOG_FORMAT"); logFormat != "" {
		c.Logging.Format = logFormat
	}
}

// LoadFromCommandLine 从命令行参数加载配置
func (c *Config) LoadFromCommandLine(args map[string]string) {
	// 服务配置
	if v, ok := args["port"]; ok {
		c.Server.Port = v
	}
	if v, ok := args["pass"]; ok {
		c.Server.Password = v
	}

	// 回滚系统配置
	if v, ok := args["rollback-enabled"]; ok {
		c.Rollback.Enabled = parseBoolean(v)
	}
	if v, ok := args["max-retries"]; ok {
		if val, err := strconv.Atoi(v); err == nil {
			c.Rollback.MaxRetries = val
		}
	}
	if v, ok := args["processing-timeout"]; ok {
		if val, err := time.ParseDuration(v); err == nil {
			c.Rollback.ProcessingTimeout = val
		}
	}
	if v, ok := args["stalled-scan-interval"]; ok {
		if val, err := time.ParseDuration(v); err == nil {
			c.Rollback.StalledScanInterval = val
		}
	}

	// 指标配置
	if v, ok := args["metrics-enabled"]; ok {
		c.Metrics.Enabled = parseBoolean(v)
	}
	if v, ok := args["metrics-port"]; ok {
		c.Metrics.Port = v
	}

	// 日志配置
	if v, ok := args["log-level"]; ok {
		c.Logging.Level = v
	}
	if v, ok := args["log-format"]; ok {
		c.Logging.Format = v
	}
}

// LoadFromFile 从 YAML 配置文件加载配置
//
// 配置优先级:
//  1. DefaultConfig() 默认值
//  2. YAML 文件 (LoadFromFile)
//  3. 环境变量 (LoadFromEnv)
//  4. 命令行参数 (LoadFromCommandLine)
//
// 如果文件不存在则不会返回错误, 以便保持默认配置和更高优先级配置生效。
func (c *Config) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在时保持静默, 使用默认值/环境变量/命令行参数
			return nil
		}
		return fmt.Errorf("read config file %s: %w", path, err)
	}

	// 在当前配置副本上反序列化, 解析成功后再整体替换, 避免部分更新
	cfgCopy := *c
	if err := yaml.Unmarshal(data, &cfgCopy); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}

	// time.Duration 字段使用 yaml.v2 对 encoding.TextUnmarshaler 的支持,
	// 可以正确解析 "10s", "1m" 这类字符串
	*c = cfgCopy
	return nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server.port must not be empty")
	}

	if c.Rollback.MaxRetries < 1 {
		return fmt.Errorf("rollback.max_retries must be >= 1")
	}

	if c.Rollback.ProcessingTimeout <= 0 {
		return fmt.Errorf("rollback.processing_timeout must be > 0")
	}

	if c.Rollback.StalledScanInterval <= 0 {
		return fmt.Errorf("rollback.stalled_scan_interval must be > 0")
	}

	if c.Metrics.Port == "" {
		return fmt.Errorf("metrics.port must not be empty")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error")
	}

	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("logging.format must be one of: json, text")
	}

	return nil
}

// String 返回配置的字符串表示
func (c *Config) String() string {
	var sb strings.Builder
	sb.WriteString("Configuration:\n")
	sb.WriteString(fmt.Sprintf("  Server.Port: %s\n", c.Server.Port))
	sb.WriteString(fmt.Sprintf("  Rollback.Enabled: %v\n", c.Rollback.Enabled))
	sb.WriteString(fmt.Sprintf("  Rollback.MaxRetries: %d\n", c.Rollback.MaxRetries))
	sb.WriteString(fmt.Sprintf("  Rollback.ProcessingTimeout: %v\n", c.Rollback.ProcessingTimeout))
	sb.WriteString(fmt.Sprintf("  Rollback.StalledScanInterval: %v\n", c.Rollback.StalledScanInterval))
	sb.WriteString(fmt.Sprintf("  Metrics.Enabled: %v\n", c.Metrics.Enabled))
	sb.WriteString(fmt.Sprintf("  Metrics.Port: %s\n", c.Metrics.Port))
	sb.WriteString(fmt.Sprintf("  Logging.Level: %s\n", c.Logging.Level))
	return sb.String()
}

// parseBoolean 解析布尔值
func parseBoolean(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}
