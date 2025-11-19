#!/bin/bash

# goForward v2.0.0 重启脚本
# 停止现有服务，然后启动新的服务

echo "================================================"
echo "goForward v2.0.0 - 重启服务"
echo "================================================"
echo ""

# 获取脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 检查必要的脚本
if [ ! -f "stop_v2.sh" ]; then
    echo "❌ 未找到 stop_v2.sh 脚本"
    exit 1
fi

if [ ! -f "start_v2.sh" ]; then
    echo "❌ 未找到 start_v2.sh 脚本"
    exit 1
fi

# 停止旧服务
echo "🛑 第 1 步: 停止现有服务..."
echo ""
bash stop_v2.sh
STOP_RESULT=$?

if [ $STOP_RESULT -ne 0 ]; then
    echo "⚠️  停止服务时出错，继续启动新实例..."
fi

# 等待一秒，确保端口释放
echo ""
echo "⏳ 等待端口释放..."
sleep 2

# 启动新服务
echo "🚀 第 2 步: 启动新服务..."
echo ""
bash start_v2.sh
