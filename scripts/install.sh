#!/bin/bash
# goForward 一键安装脚本
# 版本: v1.6.0

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 版本信息
VERSION="v1.6.0"
INSTALL_DIR="/opt/goforward"
SERVICE_NAME="goforward"

# 脚本下载地址（使用 GitHub 原始文件 URL）
SCRIPT_URL="https://raw.githubusercontent.com/tanaer/port-forward/master/scripts/hysteria2-install.py"
SCRIPT_NAME="hysteria2-install.py"
SCRIPT_PATH="/tmp/$SCRIPT_NAME"

# 打印信息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# 显示横幅
show_banner() {
    clear
    echo -e "${BLUE}"
    cat << "EOF"
╦ ╦╦ ╦╔═╗╔╦╗╔═╗╦═╗╦╔═╗  ╔═╗  ╦╔╗╔╔═╗╔╦╗╔═╗╦  ╦
╠═╣╚╦╝╚═╗ ║ ║╣ ╠╦╝║╠═╣  ╔═╝  ║║║║╚═╗ ║ ╠═╣║  ║
╩ ╩ ╩ ╚═╝ ╩ ╚═╝╩╚═╩╩ ╩  ╚═╝  ╩╝╚╝╚═╝ ╩ ╩ ╩╩═╝╩═╝

     Hysteria2 一键安装脚本 v2.0
     安全 · 快速 · 易用
EOF
    echo -e "${NC}"
    echo "=================================================="
    echo ""
}

# 检查root权限
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "请使用 root 权限运行此脚本"
        echo "使用: sudo $0"
        exit 1
    fi
}

# 检查系统
check_system() {
    print_info "检测系统..."

    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VER=$VERSION_ID
        print_success "检测到系统: $PRETTY_NAME"
    else
        print_error "无法检测系统类型"
        exit 1
    fi

    # 检查支持的系统
    case $OS in
        ubuntu|debian|centos|rocky|fedora)
            print_success "系统支持"
            ;;
        *)
            print_warning "未测试的系统: $OS"
            read -p "是否继续？[y/N]: " continue
            if [ "$continue" != "y" ] && [ "$continue" != "Y" ]; then
                exit 0
            fi
            ;;
    esac
}

# 检查网络
check_network() {
    print_info "检测网络连接..."

    if ping -c 1 -W 3 8.8.8.8 &> /dev/null || ping -c 1 -W 3 1.1.1.1 &> /dev/null; then
        print_success "网络连接正常"
    else
        print_error "网络连接失败，请检查网络设置"
        exit 1
    fi
}

# 系统网络优化
optimize_network() {
    print_info "优化系统网络参数..."

    # 检查是否已优化过
    if grep -q "# 网络性能优化 (goForward" /etc/sysctl.conf 2>/dev/null; then
        print_success "网络参数已优化，跳过"
        print_info "当前拥塞控制: $(sysctl -n net.ipv4.tcp_congestion_control)"
        return 0
    fi

    # 检查 BBR 是否已启用
    current_cc=$(sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null || echo "unknown")
    if [ "$current_cc" = "bbr" ]; then
        print_success "BBR 已启用"
    else
        # 检查内核是否支持 BBR
        if modprobe tcp_bbr 2>/dev/null && lsmod | grep -q tcp_bbr; then
            print_info "启用 BBR..."
        elif grep -q bbr /proc/sys/net/ipv4/tcp_available_congestion_control 2>/dev/null; then
            print_info "启用 BBR..."
        else
            print_warning "内核不支持 BBR，跳过 BBR 配置"
        fi
    fi

    # 备份原配置
    if [ -f /etc/sysctl.conf ]; then
        cp /etc/sysctl.conf /etc/sysctl.conf.bak.$(date +%Y%m%d%H%M%S)
    fi

    # 写入优化参数
    cat >> /etc/sysctl.conf << 'SYSCTL_EOF'

###################################################################
# 网络性能优化 (goForward 安装脚本自动配置)
###################################################################

# BBR 拥塞控制
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr

# 系统级别 socket 缓冲区 (64MB)
net.core.rmem_max=67108864
net.core.wmem_max=67108864
net.core.rmem_default=1048576
net.core.wmem_default=1048576

# TCP 缓冲区
net.ipv4.tcp_rmem=4096 87380 67108864
net.ipv4.tcp_wmem=4096 65536 67108864

# UDP 缓冲区 (Hysteria2/QUIC)
net.ipv4.udp_rmem_min=8192
net.ipv4.udp_wmem_min=8192

# 连接队列优化
net.core.somaxconn=65535
net.core.netdev_max_backlog=65535
net.ipv4.tcp_max_syn_backlog=65535

# TIME_WAIT 优化
net.ipv4.tcp_tw_reuse=1
net.ipv4.tcp_fin_timeout=15
net.ipv4.tcp_max_tw_buckets=65535

# Keepalive 优化
net.ipv4.tcp_keepalive_time=300
net.ipv4.tcp_keepalive_probes=3
net.ipv4.tcp_keepalive_intvl=30

# 性能调优
net.ipv4.tcp_slow_start_after_idle=0
net.ipv4.tcp_mtu_probing=1

# IP 转发
net.ipv4.ip_forward=1
net.ipv6.conf.all.forwarding=1

# SYN Flood 防护
net.ipv4.tcp_syncookies=1
net.ipv4.tcp_syn_retries=2
net.ipv4.tcp_synack_retries=2

# 本地端口范围
net.ipv4.ip_local_port_range=1024 65535

# 文件描述符
fs.file-max=2097152

# 内存优化
vm.swappiness=10
SYSCTL_EOF

    # 应用配置
    sysctl -p > /dev/null 2>&1

    # 配置文件描述符限制
    if ! grep -q "# goForward network optimization" /etc/security/limits.conf 2>/dev/null; then
        cat >> /etc/security/limits.conf << 'LIMITS_EOF'

# goForward network optimization
* soft nofile 1048576
* hard nofile 1048576
* soft nproc 65535
* hard nproc 65535
root soft nofile 1048576
root hard nofile 1048576
LIMITS_EOF
    fi

    print_success "网络优化完成"
    print_info "当前拥塞控制: $(sysctl -n net.ipv4.tcp_congestion_control)"
}

# 安装Python3
install_python() {
    print_info "检查 Python3..."

    if command -v python3 &> /dev/null; then
        PYTHON_VER=$(python3 --version | awk '{print $2}')
        print_success "Python3 已安装: $PYTHON_VER"
    else
        print_info "正在安装 Python3..."
        case $OS in
            ubuntu|debian)
                apt update && apt install -y python3 python3-pip
                ;;
            centos|rocky|fedora)
                dnf install -y python3 python3-pip
                ;;
        esac

        if command -v python3 &> /dev/null; then
            print_success "Python3 安装成功"
        else
            print_error "Python3 安装失败"
            exit 1
        fi
    fi
}

# 下载脚本
download_script() {
    print_info "下载安装脚本..."

    # 尝试使用 curl
    if command -v curl &> /dev/null; then
        if curl -fsSL "$SCRIPT_URL" -o "$SCRIPT_PATH"; then
            print_success "脚本下载成功"
            chmod +x "$SCRIPT_PATH"
            return 0
        else
            print_error "curl 下载失败"
        fi
    fi

    # 尝试使用 wget
    if command -v wget &> /dev/null; then
        if wget -q "$SCRIPT_URL" -O "$SCRIPT_PATH"; then
            print_success "脚本下载成功"
            chmod +x "$SCRIPT_PATH"
            return 0
        else
            print_error "wget 下载失败"
        fi
    fi

    print_error "脚本下载失败，请检查网络连接"
    exit 1
}

# 运行安装脚本
run_script() {
    print_info "启动自动安装（无需手动配置）..."
    echo ""

    # 自动运行模式，使用 --auto 参数
    python3 "$SCRIPT_PATH" --auto
}

# 清理
cleanup() {
    if [ -f "/tmp/$SCRIPT_NAME" ]; then
        rm -f "/tmp/$SCRIPT_NAME"
    fi
}

# 主函数
main() {
    show_banner

    check_root
    check_system
    check_network
    optimize_network
    install_python
    download_script

    echo ""
    print_success "准备完成，即将启动安装向导..."
    echo ""
    sleep 2

    run_script

    cleanup
}

# 捕获中断信号
trap cleanup EXIT INT TERM

# 运行主函数
main
