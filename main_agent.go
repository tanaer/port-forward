//go:build agent
// +build agent

package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"goForward/agent/client"
)

// generateHardwareID 生成基于硬件的稳定节点ID
func generateHardwareID() string {
	var identifiers []string

	// 方法1: MAC地址（最稳定的硬件标识）
	if mac := getMACAddress(); mac != "" {
		identifiers = append(identifiers, mac)
	}

	// 方法2: 机器ID（Linux系统）
	if machineID := getMachineID(); machineID != "" {
		identifiers = append(identifiers, machineID)
	}

	// 方法3: 主机名（备选）
	if hostname, err := os.Hostname(); err == nil {
		identifiers = append(identifiers, hostname)
	}

	// 如果没有获取到任何标识，使用主机名作为备选
	if len(identifiers) == 0 {
		hostname, _ := os.Hostname()
		return fmt.Sprintf("agent-%s", hostname)
	}

	// 使用获取到的标识生成稳定的哈希
	combined := strings.Join(identifiers, "-")
	hash := md5.Sum([]byte(combined))
	hashStr := fmt.Sprintf("%x", hash)[:12] // 取前12位

	hostname, _ := os.Hostname()
	return fmt.Sprintf("agent-%s-%s", hostname, hashStr)
}

// getMACAddress 获取第一个非本地MAC地址
func getMACAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过本地接口和未启用的接口
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// 获取MAC地址
		if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
			// 跳过虚拟网卡（根据常见的虚拟网卡MAC前缀）
			macStr := iface.HardwareAddr.String()
			if !strings.HasPrefix(macStr, "00:00") && !strings.HasPrefix(macStr, "52:54") {
				return strings.ReplaceAll(macStr, ":", "")
			}
		}
	}

	return ""
}

// getMachineID 从系统获取机器ID
func getMachineID() string {
	// 尝试从 /etc/machine-id 读取（Linux）
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		return strings.TrimSpace(string(data))
	}

	// 尝试从 /var/db/dbus/machine-id 读取（某些系统）
	if data, err := os.ReadFile("/var/db/dbus/machine-id"); err == nil {
		return strings.TrimSpace(string(data))
	}

	// 尝试使用 hostid 命令（Unix/Linux）
	if cmd := exec.Command("hostid"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			return strings.TrimSpace(string(output))
		}
	}

	return ""
}

func main() {
	// 命令行参数
	controlAddr := flag.String("control", "localhost:50051", "控制端地址 (格式: host:port)")
	nodeID := flag.String("node", "", "节点ID (留空则自动生成)")
	flag.Parse()

	// 自动生成节点ID（基于硬件标识，保证同一台机器的ID稳定且唯一）
	if *nodeID == "" {
		*nodeID = generateHardwareID()
		log.Printf("[生成] 基于硬件特征生成节点ID")
	}

	log.Printf("============================================================")
	log.Printf("goForward Agent 启动")
	log.Printf("============================================================")
	log.Printf("")
	log.Printf("节点ID: %s", *nodeID)
	log.Printf("控制端: %s", *controlAddr)
	log.Printf("")

	// 创建 Agent 客户端
	agent := client.NewAgentClient(*controlAddr, *nodeID)

	// 连接到控制端
	log.Printf("[启动] 连接到控制端...")
	if err := agent.Connect(); err != nil {
		log.Fatalf("❌ 连接控制端失败: %v", err)
	}
	log.Printf("✅ 成功连接到控制端")

	// 注册节点
	log.Printf("[启动] 注册节点...")
	if err := agent.RegisterNode(); err != nil {
		log.Fatalf("❌ 节点注册失败: %v", err)
	}
	log.Printf("✅ 节点注册成功")

	// 启动心跳
	log.Printf("[启动] 启动心跳机制...")
	if err := agent.StartHeartbeat(); err != nil {
		log.Fatalf("❌ 启动心跳失败: %v", err)
	}
	log.Printf("✅ 心跳机制已启动")

	// 启动配置流
	log.Printf("[启动] 启动配置订阅...")
	if err := agent.StartConfigStream(); err != nil {
		log.Fatalf("❌ 启动配置流失败: %v", err)
	}
	log.Printf("✅ 配置订阅已启动")

	log.Printf("")
	log.Printf("============================================================")
	log.Printf("🎉 Agent 启动成功！")
	log.Printf("============================================================")
	log.Printf("")
	log.Printf("📍 连接信息:")
	log.Printf("   节点ID: %s", *nodeID)
	log.Printf("   控制端: %s", *controlAddr)
	log.Printf("")
	log.Printf("💡 Agent 正在运行:")
	log.Printf("   - 心跳: 每 10 秒发送一次")
	log.Printf("   - 配置: 实时监听控制端推送")
	log.Printf("   - 状态: 自动上报节点状态")
	log.Printf("")
	log.Printf("按 Ctrl+C 停止 Agent")
	log.Printf("")

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh

	log.Printf("")
	log.Printf("🛑 收到停止信号，正在关闭 Agent...")

	// 优雅关闭
	if err := agent.Stop(); err != nil {
		log.Printf("⚠️  停止 Agent 时出错: %v", err)
	}

	log.Printf("✅ Agent 已安全关闭")
}
