package hysteria

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Client Hysteria2客户端管理器
type Client struct {
	cmd        *exec.Cmd
	configPath string
	running    bool
	mu         sync.Mutex
}

// NewClient 创建Hysteria2客户端
func NewClient(configPath string) *Client {
	return &Client{
		configPath: configPath,
		running:    false,
	}
}

// Start 启动Hysteria2客户端
func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("Hysteria2已在运行")
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(c.configPath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", c.configPath)
	}

	// 查找hysteria2可执行文件
	hy2Path, err := findHysteria2Binary()
	if err != nil {
		return err
	}

	// 创建命令
	c.cmd = exec.Command(hy2Path, "client", "-c", c.configPath)
	c.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 设置输出
	logDir := filepath.Join(filepath.Dir(c.configPath), "logs")
	os.MkdirAll(logDir, 0755)

	logFile, err := os.OpenFile(
		filepath.Join(logDir, "hysteria2.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %v", err)
	}

	c.cmd.Stdout = logFile
	c.cmd.Stderr = logFile

	// 启动进程
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("启动Hysteria2失败: %v", err)
	}

	c.running = true
	fmt.Printf("[Hysteria2] 进程已启动 PID=%d\n", c.cmd.Process.Pid)

	// 监控进程
	go c.monitor()

	return nil
}

// Stop 停止Hysteria2客户端
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running || c.cmd == nil || c.cmd.Process == nil {
		return fmt.Errorf("Hysteria2未运行")
	}

	// 发送SIGTERM信号
	if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// 如果SIGTERM失败，强制SIGKILL
		c.cmd.Process.Kill()
	}

	// 等待进程退出
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-done:
		c.running = false
		fmt.Println("[Hysteria2] 进程已停止")
		return nil
	case <-time.After(5 * time.Second):
		// 超时强制杀死
		c.cmd.Process.Kill()
		c.running = false
		return fmt.Errorf("停止Hysteria2超时，已强制终止")
	}
}

// Restart 重启Hysteria2客户端
func (c *Client) Restart() error {
	if err := c.Stop(); err != nil {
		fmt.Printf("[Hysteria2] 停止失败: %v\n", err)
	}

	time.Sleep(1 * time.Second)

	return c.Start()
}

// IsRunning 检查Hysteria2是否运行
func (c *Client) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// monitor 监控进程状态
func (c *Client) monitor() {
	if c.cmd == nil {
		return
	}

	err := c.cmd.Wait()

	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	if err != nil {
		fmt.Printf("[Hysteria2] 进程异常退出: %v\n", err)
	} else {
		fmt.Println("[Hysteria2] 进程正常退出")
	}
}

// findHysteria2Binary 查找hysteria2可执行文件
func findHysteria2Binary() (string, error) {
	// 优先查找本地bin目录
	execPath, err := os.Executable()
	if err == nil {
		localHy2 := filepath.Join(filepath.Dir(execPath), "bin", "hysteria2")
		if _, err := os.Stat(localHy2); err == nil {
			return localHy2, nil
		}
		// 尝试hysteria命名
		localHy := filepath.Join(filepath.Dir(execPath), "bin", "hysteria")
		if _, err := os.Stat(localHy); err == nil {
			return localHy, nil
		}
	}

	// 查找系统PATH
	hy2Path, err := exec.LookPath("hysteria2")
	if err == nil {
		return hy2Path, nil
	}

	hyPath, err := exec.LookPath("hysteria")
	if err == nil {
		return hyPath, nil
	}

	return "", fmt.Errorf("未找到hysteria2可执行文件，请安装hysteria2或将其放在bin/目录下")
}
