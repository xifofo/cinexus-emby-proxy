#!/bin/bash

echo "=== 测试服务器关闭功能 ==="

# 启动服务器
echo "启动服务器..."
go run main.go server &
SERVER_PID=$!

echo "服务器PID: $SERVER_PID"

# 等待5秒让服务器完全启动
echo "等待5秒让服务器启动..."
sleep 5

# 发送SIGTERM信号
echo "发送SIGTERM信号关闭服务器..."
kill -TERM $SERVER_PID

# 等待服务器关闭，最多等待15秒
echo "等待服务器关闭（最多15秒）..."
timeout 15 tail --pid=$SERVER_PID -f /dev/null

if kill -0 $SERVER_PID 2>/dev/null; then
    echo "❌ 服务器未能正常关闭，强制杀死进程"
    kill -9 $SERVER_PID
    exit 1
else
    echo "✅ 服务器已成功关闭"
    exit 0
fi