#!/bin/bash

# goForward Agent 启动脚本

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 默认参数
CONTROL_ADDR="${CONTROL_ADDR:-localhost:50051}"
NODE_ID="${NODE_ID:-}"

echo "================================================"
echo "goForward Agent - 启动脚本"
echo "================================================"
echo ""

# 检查二进制文件
if [ ! -f "goForward_agent" ]; then
    echo "❌ 未找到 goForward_agent 二进制文件"
    echo "请先编译: go build -o goForward_agent main_agent.go"
    exit 1
fi

# 检查控制端是否可访问
CONTROL_HOST=$(echo $CONTROL_ADDR | cut -d: -f1)
CONTROL_PORT=$(echo $CONTROL_ADDR | cut -d: -f2)

echo "🔍 检查控制端连接..."
if ! nc -zv $CONTROL_HOST $CONTROL_PORT > /dev/null 2>&1; then
    echo "⚠️  警告: 无法连接到控制端 $CONTROL_ADDR"
    echo "   请确保控制端服务正在运行"
    echo ""
    read -p "是否继续启动? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo "✅ 控制端连接正常"
fi

echo ""
echo "🚀 启动 Agent..."
echo ""
echo "配置信息:"
echo "  控制端: $CONTROL_ADDR"
if [ -n "$NODE_ID" ]; then
    echo "  节点ID: $NODE_ID"
else
    echo "  节点ID: (自动生成)"
fi
echo ""

# 启动 Agent
if [ -n "$NODE_ID" ]; then
    nohup ./goForward_agent -control "$CONTROL_ADDR" -node "$NODE_ID" > /tmp/goforward_agent.log 2>&1 &
else
    nohup ./goForward_agent -control "$CONTROL_ADDR" > /tmp/goforward_agent.log 2>&1 &
fi

PID=$!
echo $PID > /tmp/goforward_agent.pid

# 等待启动
sleep 2

# 检查是否启动成功
if ps -p $PID > /dev/null 2>&1; then
    echo "✅ Agent 启动成功 (PID: $PID)"
    echo ""
    echo "查看日志: tail -f /tmp/goforward_agent.log"
    echo "停止服务: ./stop_agent.sh"
    echo ""
else
    echo "❌ Agent 启动失败"
    echo "错误日志:"
    tail -20 /tmp/goforward_agent.log
    exit 1
fi
