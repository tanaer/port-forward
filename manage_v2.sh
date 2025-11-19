#!/bin/bash

# goForward v2.0.0 完整服务管理脚本
# 功能: 启动、停止、重启、状态检查、端口清理

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 显示帮助信息
show_help() {
    cat << EOF
================================================
goForward v2.0.0 服务管理工具
================================================

使用方法: ./manage_v2.sh <命令> [选项]

命令:
  start       启动 v2.0.0 服务
  stop        停止运行中的服务
  restart     重启服务 (停止后立即启动)
  status      检查服务状态
  logs        显示实时日志
  clean       清理被占用的端口 (强制)
  help        显示此帮助信息

示例:
  ./manage_v2.sh start           # 启动服务
  ./manage_v2.sh restart         # 重启服务
  ./manage_v2.sh status          # 查看状态
  ./manage_v2.sh clean           # 清理端口

EOF
}

# 启动服务
start_service() {
    echo "================================================"
    echo "goForward v2.0.0 - 启动服务"
    echo "================================================"
    echo ""

    # 检查是否已在运行
    if pgrep -f "goForward_v2" > /dev/null 2>&1; then
        echo "⚠️  服务已在运行中"
        echo "请先停止: ./manage_v2.sh stop"
        echo "或重启: ./manage_v2.sh restart"
        return 1
    fi

    # 检查二进制文件
    if [ ! -f "goForward_v2" ]; then
        echo "❌ 未找到 goForward_v2 二进制文件"
        echo "请先编译: go build -o goForward_v2 main_v2.go"
        return 1
    fi

    # 检查配置文件
    if [ ! -f "goforward.yaml" ]; then
        echo "⚠️  未找到 goforward.yaml，使用默认配置"
    else
        echo "📖 使用 goforward.yaml 配置"
    fi

    echo ""
    echo "🚀 启动 goForward v2.0.0..."
    echo ""

    # 后台启动，并保存 PID
    nohup ./goForward_v2 > /tmp/goforward_v2.log 2>&1 &
    PID=$!
    echo $PID > /tmp/goforward_v2.pid

    # 等待服务启动
    sleep 2

    # 检查是否启动成功
    if ps -p $PID > /dev/null 2>&1; then
        echo "✅ 服务启动成功 (PID: $PID)"
        echo ""
        echo "查看详细日志: tail -f /tmp/goforward_v2.log"
        return 0
    else
        echo "❌ 服务启动失败"
        echo "错误日志:"
        tail -20 /tmp/goforward_v2.log
        return 1
    fi
}

# 停止服务
stop_service() {
    echo "================================================"
    echo "goForward v2.0.0 - 停止服务"
    echo "================================================"
    echo ""

    # 查找运行中的进程
    PIDS=$(pgrep -f "goForward_v2" | grep -v grep)

    if [ -z "$PIDS" ]; then
        echo "ℹ️  没有找到运行中的 goForward_v2 进程"
        return 0
    fi

    echo "找到运行中的进程: $PIDS"
    echo ""

    # 优雅停止
    echo "📡 发送停止信号 (SIGTERM)..."
    kill -TERM $PIDS 2>/dev/null

    # 等待优雅关闭
    for i in {1..10}; do
        if ! pgrep -f "goForward_v2" > /dev/null 2>&1; then
            echo "✅ 服务已正常关闭"
            echo ""
            return 0
        fi
        sleep 1
    done

    # 强制杀死
    echo "⚠️  进程未能及时关闭，执行强制关闭..."
    kill -9 $PIDS 2>/dev/null

    sleep 1

    if ! pgrep -f "goForward_v2" > /dev/null 2>&1; then
        echo "✅ 服务已被强制关闭"
        echo ""
        return 0
    else
        echo "❌ 无法关闭服务"
        return 1
    fi
}

# 重启服务
restart_service() {
    echo "重启服务..."
    stop_service
    echo ""
    sleep 2
    start_service
}

# 检查状态
check_status() {
    echo "================================================"
    echo "goForward v2.0.0 - 服务状态"
    echo "================================================"
    echo ""

    # 进程状态
    echo "📌 进程状态:"
    if pgrep -f "goForward_v2" > /dev/null 2>&1; then
        PIDS=$(pgrep -f "goForward_v2")
        echo "   ✅ 运行中 (PID: $PIDS)"
    else
        echo "   ❌ 未运行"
    fi

    echo ""
    echo "🔌 端口监听状态:"

    # 检查各个端口
    for port in 8889 8890 9090 50051; do
        if ss -tlnp 2>/dev/null | grep ":$port" > /dev/null 2>&1; then
            case $port in
                8889|8890) echo "   ✅ Web 管理界面 (端口 $port)" ;;
                9090) echo "   ✅ Prometheus 指标 (端口 $port)" ;;
                50051) echo "   ✅ gRPC 服务 (端口 $port)" ;;
            esac
        elif netstat -tlnp 2>/dev/null | grep ":$port" > /dev/null 2>&1; then
            case $port in
                8889|8890) echo "   ✅ Web 管理界面 (端口 $port)" ;;
                9090) echo "   ✅ Prometheus 指标 (端口 $port)" ;;
                50051) echo "   ✅ gRPC 服务 (端口 $port)" ;;
            esac
        fi
    done

    echo ""
    echo "🌐 服务可访问性:"

    # 测试 Web
    if timeout 1 curl -s http://localhost:8890/ > /dev/null 2>&1; then
        echo "   ✅ Web 管理界面 (8890): 可访问"
    elif timeout 1 curl -s http://localhost:8889/ > /dev/null 2>&1; then
        echo "   ✅ Web 管理界面 (8889): 可访问"
    else
        echo "   ⚠️  Web 管理界面: 无法访问"
    fi

    # 测试 Prometheus
    if timeout 1 curl -s http://localhost:9090/metrics > /dev/null 2>&1; then
        echo "   ✅ Prometheus 指标: 可访问"
    else
        echo "   ⚠️  Prometheus 指标: 无法访问"
    fi

    echo ""
    echo "📊 数据库:"
    if [ -f "/tmp/goForward_control.db" ]; then
        SIZE=$(du -h "/tmp/goForward_control.db" | cut -f1)
        echo "   ✅ $SIZE"
    else
        echo "   ℹ️  未创建 (首次启动会自动创建)"
    fi

    echo ""
}

# 显示日志
show_logs() {
    echo "显示实时日志 (按 Ctrl+C 退出)..."
    echo ""
    tail -f /tmp/goforward_v2.log
}

# 清理端口
clean_ports() {
    echo "================================================"
    echo "goForward v2.0.0 - 清理被占用的端口"
    echo "================================================"
    echo ""

    # 强制停止
    stop_service

    echo ""
    echo "检查并释放被占用的端口..."

    CLEANED=0

    # 检查 Web 端口
    for port in 8889 8890; do
        PID=$(ss -tlnp 2>/dev/null | grep ":$port" | grep -oE 'pid=[0-9]+' | cut -d= -f2)
        if [ ! -z "$PID" ]; then
            echo "发现被占用的端口 $port (进程 $PID)，正在强制关闭..."
            kill -9 $PID 2>/dev/null
            CLEANED=$((CLEANED + 1))
        fi
    done

    # 检查 Prometheus 端口
    PID=$(ss -tlnp 2>/dev/null | grep ":9090" | grep -oE 'pid=[0-9]+' | cut -d= -f2)
    if [ ! -z "$PID" ]; then
        echo "发现被占用的端口 9090 (进程 $PID)，正在强制关闭..."
        kill -9 $PID 2>/dev/null
        CLEANED=$((CLEANED + 1))
    fi

    # 检查 gRPC 端口
    PID=$(ss -tlnp 2>/dev/null | grep ":50051" | grep -oE 'pid=[0-9]+' | cut -d= -f2)
    if [ ! -z "$PID" ]; then
        echo "发现被占用的端口 50051 (进程 $PID)，正在强制关闭..."
        kill -9 $PID 2>/dev/null
        CLEANED=$((CLEANED + 1))
    fi

    if [ $CLEANED -gt 0 ]; then
        echo ""
        echo "✅ 已清理 $CLEANED 个被占用的端口"
    else
        echo ""
        echo "ℹ️  没有找到被占用的端口"
    fi

    echo ""
    echo "现在可以启动新的服务: ./manage_v2.sh start"
    echo ""
}

# 主程序
case "${1:-help}" in
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    restart)
        restart_service
        ;;
    status)
        check_status
        ;;
    logs)
        show_logs
        ;;
    clean)
        clean_ports
        ;;
    help|*)
        show_help
        ;;
esac
