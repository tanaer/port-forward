#!/bin/bash

# goForward v2.0.0 状态检查脚本
# 检查服务运行状态、监听端口和可访问性

echo "================================================"
echo "goForward v2.0.0 - 服务状态检查"
echo "================================================"
echo ""

# 检查进程状态
echo "📌 进程状态:"
PIDS=$(pgrep -f "goForward_v2" | grep -v grep)

if [ -z "$PIDS" ]; then
    echo "   ❌ goForward_v2 未运行"
    PROCESS_RUNNING=0
else
    echo "   ✅ goForward_v2 正在运行 (PID: $PIDS)"
    PROCESS_RUNNING=1
fi

echo ""

# 检查监听端口
echo "🔌 监听端口状态:"

# 尝试检查 Web 服务端口 (默认 8889)
if ss -tlnp 2>/dev/null | grep -E ":(8889|8890)" > /dev/null 2>&1 || netstat -tlnp 2>/dev/null | grep -E ":(8889|8890)" > /dev/null 2>&1; then
    WEB_PORT=$(ss -tlnp 2>/dev/null | grep -E ":(8889|8890)" | awk '{print $4}' | grep -oE "[0-9]+$" | head -1)
    if [ -z "$WEB_PORT" ]; then
        WEB_PORT=$(netstat -tlnp 2>/dev/null | grep -E ":(8889|8890)" | awk '{print $4}' | grep -oE "[0-9]+$" | head -1)
    fi
    if [ ! -z "$WEB_PORT" ]; then
        echo "   ✅ Web 管理界面: http://localhost:$WEB_PORT"
    fi
else
    echo "   ⚠️  Web 管理界面: 未检测到监听"
fi

# 检查 gRPC 端口
if ss -tlnp 2>/dev/null | grep ":50051" > /dev/null 2>&1 || netstat -tlnp 2>/dev/null | grep ":50051" > /dev/null 2>&1; then
    echo "   ✅ gRPC 控制端: :50051"
else
    echo "   ⚠️  gRPC 控制端: 未检测到监听"
fi

# 检查 Prometheus 端口
if ss -tlnp 2>/dev/null | grep ":9090" > /dev/null 2>&1 || netstat -tlnp 2>/dev/null | grep ":9090" > /dev/null 2>&1; then
    echo "   ✅ Prometheus 指标: http://localhost:9090/metrics"
else
    echo "   ⚠️  Prometheus 指标: 未检测到监听"
fi

echo ""

# 尝试访问服务
echo "🌐 服务可访问性测试:"

# 测试 Web 界面
if [ ! -z "$WEB_PORT" ]; then
    if timeout 2 curl -s http://localhost:$WEB_PORT/ > /dev/null 2>&1; then
        echo "   ✅ Web 管理界面: 可访问"
    else
        echo "   ⚠️  Web 管理界面: 无法访问"
    fi
fi

# 测试 Prometheus
if timeout 2 curl -s http://localhost:9090/metrics > /dev/null 2>&1; then
    echo "   ✅ Prometheus 指标: 可访问"
else
    echo "   ⚠️  Prometheus 指标: 无法访问"
fi

# 测试 gRPC (使用 nc)
if nc -zv localhost 50051 > /dev/null 2>&1; then
    echo "   ✅ gRPC 服务: 端口开放"
else
    echo "   ⚠️  gRPC 服务: 端口未开放"
fi

echo ""

# 显示数据库路径
echo "💾 数据库信息:"
DB_PATH="/tmp/goForward_control.db"
if [ -f "$DB_PATH" ]; then
    DB_SIZE=$(du -h "$DB_PATH" | cut -f1)
    echo "   ✅ 数据库文件: $DB_PATH ($DB_SIZE)"
else
    echo "   ℹ️  数据库文件: 不存在 (首次启动)"
fi

echo ""

# 总体状态
if [ $PROCESS_RUNNING -eq 1 ]; then
    echo "📊 整体状态: 🟢 运行中"
else
    echo "📊 整体状态: 🔴 未运行"
fi

echo ""
