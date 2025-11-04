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
        if curl -fsSL "$SCRIPT_URL" -o "/tmp/$SCRIPT_NAME"; then
            print_success "脚本下载成功"
            chmod +x "/tmp/$SCRIPT_NAME"
            return 0
        fi
    fi

    # 尝试使用 wget
    if command -v wget &> /dev/null; then
        if wget -q "$SCRIPT_URL" -O "/tmp/$SCRIPT_NAME"; then
            print_success "脚本下载成功"
            chmod +x "/tmp/$SCRIPT_NAME"
            return 0
        fi
    fi

    print_error "脚本下载失败，请检查网络连接"
    exit 1
}

# 运行安装脚本
run_script() {
    print_info "启动自动安装（无需手动配置）..."
    echo ""

    # 自动运行模式，无需交互
    python3 "/tmp/$SCRIPT_NAME" --auto
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
