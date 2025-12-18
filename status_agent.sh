#!/bin/bash

# goForward Agent 状态检查脚本

echo "================================================"
echo "goForward Agent - 状态检查"
echo "================================================"
echo ""

# 检查进程状态
echo "📌 进程状态:"
PIDS=$(pgrep -f "goForward_agent" | grep -v grep)

if [ -z "$PIDS" ]; then
    echo "   ❌ Agent 未运行"
    RUNNING=0
else
    echo "   ✅ Agent 正在运行 (PID: $PIDS)"
    RUNNING=1
fi

echo ""

# 检查日志文件
echo "📄 日志文件:"
if [ -f "/tmp/goforward_agent.log" ]; then
    LOG_SIZE=$(du -h /tmp/goforward_agent.log | cut -f1)
    echo "   ✅ /tmp/goforward_agent.log ($LOG_SIZE)"

    if [ $RUNNING -eq 1 ]; then
        echo ""
        echo "📊 最近日志 (最后 10 行):"
        echo "   ────────────────────────────────────────"
        tail -10 /tmp/goforward_agent.log | sed 's/^/   /'
        echo "   ────────────────────────────────────────"
    fi
else
    echo "   ⚠️  日志文件不存在"
fi

echo ""

# 总体状态
if [ $RUNNING -eq 1 ]; then
    echo "📊 整体状态: 🟢 运行中"
else
    echo "📊 整体状态: 🔴 未运行"
fi

echo ""
