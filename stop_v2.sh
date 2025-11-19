#!/bin/bash

# goForward v2.0.0 停止脚本
# 停止所有运行中的 goForward v2.0.0 进程

echo "================================================"
echo "goForward v2.0.0 - 停止服务"
echo "================================================"
echo ""

# 查找运行中的进程
PIDS=$(pgrep -f "goForward_v2" | grep -v grep)

if [ -z "$PIDS" ]; then
    echo "ℹ️  没有找到运行中的 goForward_v2 进程"
    exit 0
fi

echo "找到运行中的进程: $PIDS"
echo ""

# 优雅停止 - 先尝试发送 SIGTERM 信号
echo "📡 发送停止信号 (SIGTERM)..."
kill -TERM $PIDS 2>/dev/null

# 等待进程优雅关闭（最多 10 秒）
for i in {1..10}; do
    if ! pgrep -f "goForward_v2" > /dev/null; then
        echo "✅ 服务已正常关闭"
        echo ""
        exit 0
    fi
    sleep 1
done

# 如果进程还在运行，强制杀死
echo "⚠️  进程未在规定时间内关闭，强制杀死..."
kill -9 $PIDS 2>/dev/null

# 验证进程是否已杀死
if ! pgrep -f "goForward_v2" > /dev/null; then
    echo "✅ 服务已被强制关闭"
else
    echo "❌ 无法关闭服务"
    exit 1
fi

echo ""
echo "可以安全地重启或修改配置了"
echo ""
