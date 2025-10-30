package xray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Manager Xray进程管理器
type Manager struct {
	cmd        *exec.Cmd
	configPath string
	running    bool
	mu         sync.Mutex
}

// NewManager 创建Xray管理器
func NewManager(configPath string) *Manager {
	return &Manager{
		configPath: configPath,
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

	// 查找xray可执行文件
	xrayPath, err := findXrayBinary()
	if err != nil {
		return err
	}

	// 创建命令
	m.cmd = exec.Command(xrayPath, "run", "-config", m.configPath)
	m.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 设置输出
	logDir := filepath.Join(filepath.Dir(m.configPath), "logs")
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
		fmt.Println("[Xray] 进程已停止")
		return nil
	case <-time.After(5 * time.Second):
		// 超时强制杀死
		m.cmd.Process.Kill()
		m.running = false
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

	if err != nil {
		fmt.Printf("[Xray] 进程异常退出: %v\n", err)
	} else {
		fmt.Println("[Xray] 进程正常退出")
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
