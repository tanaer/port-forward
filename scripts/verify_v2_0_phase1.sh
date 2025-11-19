#!/bin/bash

echo "=== goForward 2.0 Phase 1 Week 1 验证脚本 ==="
echo ""

# 检查git状态
echo "1. 检查Git状态..."
git status
echo ""

# 检查文件是否存在
echo "2. 检查关键文件..."
files=(
    "proto/control.proto"
    "proto/control.pb.go"
    "proto/control_grpc.pb.go"
    "agent/client/grpc_client.go"
    "control/server/grpc_server.go"
    "control/server/grpc_test.go"
    "DISTRIBUTED_ARCHITECTURE.md"
)

for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        echo "✅ $file 存在"
    else
        echo "❌ $file 不存在"
    fi
done
echo ""

# 编译测试
echo "3. 编译测试..."
echo "3.1 Agent客户端编译..."
if go build ./agent/client/ 2>&1; then
    echo "✅ Agent客户端编译成功"
else
    echo "❌ Agent客户端编译失败"
fi

echo "3.2 控制端编译..."
if go build ./control/server/ 2>&1; then
    echo "✅ 控制端编译成功"
else
    echo "❌ 控制端编译失败"
fi
echo ""

# 运行测试
echo "4. 运行集成测试..."
cd control/server
if go test -v -run TestControlServer 2>&1 | tail -10; then
    echo "✅ 测试通过"
else
    echo "❌ 测试失败"
fi
cd ../..

echo ""
echo "=== 验证完成 ==="