package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// LogEntry 日志条目
type LogEntry struct {
	ID        int       `json:"id"`
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Module    string    `json:"module"`
	Context   string    `json:"context,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Meta      string    `json:"meta,omitempty"`
}

// Logger 日志管理器
type Logger struct {
	mu        sync.Mutex
	entries   []LogEntry
	logFile   string
	maxEntries int
	nextID    int
}

// NewLogger 创建新的日志管理器
func NewLogger() *Logger {
	execPath, _ := os.Executable()
	logDir := filepath.Join(filepath.Dir(execPath), "logs")
	os.MkdirAll(logDir, 0755)

	logFile := filepath.Join(logDir, "app.log")

	return &Logger{
		entries:   make([]LogEntry, 0),
		logFile:   logFile,
		maxEntries: 10000,
		nextID:    1,
	}
}

// Debug 记录调试日志
func (l *Logger) Debug(module, message, context string) {
	l.log(DEBUG, module, message, context, "")
}

// Info 记录信息日志
func (l *Logger) Info(module, message, context string) {
	l.log(INFO, module, message, context, "")
}

// Warn 记录警告日志
func (l *Logger) Warn(module, message, context string) {
	l.log(WARN, module, message, context, "")
}

// Error 记录错误日志
func (l *Logger) Error(module, message, context, meta string) {
	l.log(ERROR, module, message, context, meta)
}

// log 内部日志记录方法
func (l *Logger) log(level LogLevel, module, message, context, meta string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		ID:        l.nextID,
		Level:     level,
		Message:   message,
		Module:    module,
		Context:   context,
		Timestamp: time.Now(),
		Meta:      meta,
	}

	l.entries = append(l.entries, entry)
	l.nextID++

	// 限制内存中的日志条目数量
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[1000:] // 保留最近的1000条
	}

	// 写入文件
	l.writeToFile(entry)
}

// writeToFile 写入日志到文件
func (l *Logger) writeToFile(entry LogEntry) {
	levelStr := [...]string{"DEBUG", "INFO", "WARN", "ERROR"}[entry.Level]
	logLine := fmt.Sprintf("[%s] [%s] [%s] %s %s %s\n",
		entry.Timestamp.Format("2006-01-02 15:04:05"),
		levelStr,
		entry.Module,
		entry.Context,
		entry.Message,
		entry.Meta,
	)

	f, err := os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(logLine)
}

// GetLogs 获取日志
func (l *Logger) GetLogs(level LogLevel, module, keyword string, limit int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var filtered []LogEntry
	for i := len(l.entries) - 1; i >= 0 && len(filtered) < limit; i-- {
		entry := l.entries[i]

		// 过滤级别
		if level >= 0 && entry.Level < level {
			continue
		}

		// 过滤模块
		if module != "" && entry.Module != module {
			continue
		}

		// 过滤关键词
		if keyword != "" {
			if entry.Message != keyword && entry.Context != keyword {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	// 反转结果
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered
}

// GetStats 获取日志统计
func (l *Logger) GetStats() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()

	stats := map[string]interface{}{
		"total":     len(l.entries),
		"debug":     0,
		"info":      0,
		"warn":      0,
		"error":     0,
		"modules":   make(map[string]int),
		"last_hour": 0,
	}

	now := time.Now()
	hourAgo := now.Add(-time.Hour)

	for _, entry := range l.entries {
		switch entry.Level {
		case DEBUG:
			stats["debug"] = stats["debug"].(int) + 1
		case INFO:
			stats["info"] = stats["info"].(int) + 1
		case WARN:
			stats["warn"] = stats["warn"].(int) + 1
		case ERROR:
			stats["error"] = stats["error"].(int) + 1
		}

		// 统计模块
		stats["modules"].(map[string]int)[entry.Module]++

		// 统计最近一小时的日志
		if entry.Timestamp.After(hourAgo) {
			stats["last_hour"] = stats["last_hour"].(int) + 1
		}
	}

	return stats
}

// ExportLogs 导出日志为 JSON
func (l *Logger) ExportLogs(level LogLevel, module, keyword string) string {
	entries := l.GetLogs(level, module, keyword, 10000)
	data, _ := json.MarshalIndent(map[string]interface{}{
		"exported_at": time.Now().Format("2006-01-02 15:04:05"),
		"total":       len(entries),
		"entries":     entries,
	}, "", "  ")
	return string(data)
}

// 全局日志管理器
var globalLogger = NewLogger()

// 全局方法
func Debug(module, message, context string) {
	globalLogger.Debug(module, message, context)
}

func Info(module, message, context string) {
	globalLogger.Info(module, message, context)
}

func Warn(module, message, context string) {
	globalLogger.Warn(module, message, context)
}

func Error(module, message, context, meta string) {
	globalLogger.Error(module, message, context, meta)
}

func GetLogs(level LogLevel, module, keyword string, limit int) []LogEntry {
	return globalLogger.GetLogs(level, module, keyword, limit)
}

func GetStats() map[string]interface{} {
	return globalLogger.GetStats()
}

func ExportLogs(level LogLevel, module, keyword string) string {
	return globalLogger.ExportLogs(level, module, keyword)
}
