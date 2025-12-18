package xray

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Manager Xray进程管理器
type Manager struct {
	cmd        *exec.Cmd
	configPath string
	logDir     string
	pidFile    string
	running    bool
	mu         sync.Mutex
}

// NewManager 创建Xray管理器
func NewManager(configPath string, logDir string) *Manager {
	return &Manager{
		configPath: configPath,
		logDir:     logDir,
		pidFile:    pidFilePath(configPath),
		running:    false,
	}
}

// Start 启动Xray进程
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("Xray已在运行")
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", m.configPath)
	}

	// 清理可能残留的进程（例如 goForward 重启后桥接状态丢失）
	cleanupProcessByConfig(m.configPath)

	// 查找xray可执行文件
	xrayPath, err := findXrayBinary()
	if err != nil {
		return err
	}

	// 创建命令
	m.cmd = exec.Command(xrayPath, "run", "-config", m.configPath)
	m.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 设置输出
	logDir := m.logDir
	if logDir == "" {
		logDir = filepath.Join(filepath.Dir(m.configPath), "logs")
	}
	os.MkdirAll(logDir, 0755)

	logFile, err := os.OpenFile(
		filepath.Join(logDir, "xray.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %v", err)
	}

	m.cmd.Stdout = logFile
	m.cmd.Stderr = logFile

	// 启动进程
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("启动Xray失败: %v", err)
	}

	m.writePID()
	m.running = true
	fmt.Printf("[Xray] 进程已启动 PID=%d\n", m.cmd.Process.Pid)

	// 监控进程
	go m.monitor()

	return nil
}

// Stop 停止Xray进程
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running || m.cmd == nil || m.cmd.Process == nil {
		return fmt.Errorf("Xray未运行")
	}

	// 发送SIGTERM信号
	if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// 如果SIGTERM失败，强制SIGKILL
		m.cmd.Process.Kill()
	}

	// 等待进程退出
	done := make(chan error, 1)
	go func() {
		done <- m.cmd.Wait()
	}()

	select {
	case <-done:
		m.running = false
		removePIDFile(m.pidFile)
		fmt.Println("[Xray] 进程已停止")
		return nil
	case <-time.After(5 * time.Second):
		// 超时强制杀死
		m.cmd.Process.Kill()
		m.running = false
		removePIDFile(m.pidFile)
		return fmt.Errorf("停止Xray超时，已强制终止")
	}
}

// Restart 重启Xray进程
func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		fmt.Printf("[Xray] 停止失败: %v\n", err)
	}

	time.Sleep(1 * time.Second)

	return m.Start()
}

// IsRunning 检查Xray是否运行
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// monitor 监控进程状态
func (m *Manager) monitor() {
	if m.cmd == nil {
		return
	}

	err := m.cmd.Wait()

	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	removePIDFile(m.pidFile)

	if err != nil {
		fmt.Printf("[Xray] 进程异常退出: %v\n", err)
	} else {
		fmt.Println("[Xray] 进程正常退出")
	}
}

func (m *Manager) writePID() {
	if m.pidFile == "" || m.cmd == nil || m.cmd.Process == nil {
		return
	}
	_ = os.WriteFile(m.pidFile, []byte(strconv.Itoa(m.cmd.Process.Pid)), 0644)
}

func pidFilePath(configPath string) string {
	if configPath == "" {
		return ""
	}
	ext := filepath.Ext(configPath)
	base := strings.TrimSuffix(filepath.Base(configPath), ext)
	return filepath.Join(filepath.Dir(configPath), base+".pid")
}

func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func removePIDFile(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

func cleanupProcessByPIDFile(path string) {
	if path == "" {
		return
	}
	pid, err := readPIDFile(path)
	if err != nil || pid <= 0 {
		removePIDFile(path)
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			_ = syscall.Kill(pid, syscall.SIGKILL)
			removePIDFile(path)
			return
		case <-ticker.C:
			if err := syscall.Kill(pid, 0); err != nil {
				removePIDFile(path)
				return
			}
		}
	}
}

// cleanupProcessByConfig 尝试清理使用同一 configPath 的残留 xray 进程
func cleanupProcessByConfig(configPath string) {
	if configPath == "" {
		return
	}

	// 优先通过 pid 文件清理
	cleanupProcessByPIDFile(pidFilePath(configPath))

	// 再兜底：扫描 /proc cmdline 匹配相同 configPath 的 xray 进程并终止
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		cmdlinePath := filepath.Join("/proc", e.Name(), "cmdline")
		data, err := os.ReadFile(cmdlinePath)
		if err != nil || len(data) == 0 {
			continue
		}
		cmdline := string(bytes.ReplaceAll(data, []byte{0}, []byte{' '}))
		if !strings.Contains(cmdline, "xray") || !strings.Contains(cmdline, configPath) {
			continue
		}
		_ = syscall.Kill(pid, syscall.SIGTERM)
		time.Sleep(200 * time.Millisecond)
		if err := syscall.Kill(pid, 0); err == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// findXrayBinary 查找xray可执行文件
func findXrayBinary() (string, error) {
	// 优先查找本地bin目录
	execPath, err := os.Executable()
	if err == nil {
		localXray := filepath.Join(filepath.Dir(execPath), "bin", "xray")
		if _, err := os.Stat(localXray); err == nil {
			return localXray, nil
		}
	}

	// 查找系统PATH
	xrayPath, err := exec.LookPath("xray")
	if err == nil {
		return xrayPath, nil
	}

	return "", fmt.Errorf("未找到xray可执行文件，请安装xray或将其放在bin/目录下")
}
