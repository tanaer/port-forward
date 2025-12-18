#!/bin/bash

# goForward Agent 停止脚本

echo "================================================"
echo "goForward Agent - 停止服务"
echo "================================================"
echo ""

# 查找运行中的进程
PIDS=$(pgrep -f "goForward_agent" | grep -v grep)

if [ -z "$PIDS" ]; then
    echo "ℹ️  没有找到运行中的 Agent 进程"
    exit 0
fi

echo "找到运行中的进程: $PIDS"
echo ""

# 优雅停止
echo "📡 发送停止信号 (SIGTERM)..."
kill -TERM $PIDS 2>/dev/null

# 等待优雅关闭
for i in {1..10}; do
    if ! pgrep -f "goForward_agent" > /dev/null 2>&1; then
        echo "✅ Agent 已正常关闭"
        echo ""
        exit 0
    fi
    sleep 1
done

# 强制杀死
echo "⚠️  进程未能及时关闭，执行强制关闭..."
kill -9 $PIDS 2>/dev/null

sleep 1

if ! pgrep -f "goForward_agent" > /dev/null 2>&1; then
    echo "✅ Agent 已被强制关闭"
    echo ""
    exit 0
else
    echo "❌ 无法关闭 Agent"
    exit 1
fi
