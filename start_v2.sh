#!/bin/bash

# goForward v2.0.0 启动脚本
#
# 配置说明:
# 1. 修改 goforward.yaml 文件中的 server.port 来改变 Web 管理界面端口
# 2. 或者使用环境变量覆盖: export GOFORWARD_PORT=9999
#
# 常用配置示例:
#   goforward.yaml: server.port = "8890"    # 改变端口
#   环境变量: export GOFORWARD_PASSWORD=xxx # 设置密码
#

echo "================================================"
echo "goForward v2.0.0 - 分布式控制端"
echo "================================================"
echo ""

# 检查二进制文件是否存在
if [ ! -f "goForward_v2" ]; then
    echo "❌ 未找到 goForward_v2 二进制文件"
    echo "请先编译: go build -o goForward_v2 main_v2.go"
    exit 1
fi

# 检查配置文件
if [ ! -f "goforward.yaml" ]; then
    echo "⚠️  未找到 goforward.yaml 配置文件"
    echo "将使用默认配置启动"
    echo ""
else
    echo "📖 使用 goforward.yaml 配置文件启动"
    echo ""
fi

# 启动服务
echo "🚀 启动 goForward v2.0.0..."
echo ""
./goForward_v2

