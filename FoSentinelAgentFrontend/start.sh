#!/bin/bash

# Fo-Sentinel-Agent Frontend 启动脚本

echo "🚀 启动 Fo-Sentinel-Agent Frontend..."
#echo "📁 当前目录: $(pwd)"
echo "🌐 前端服务将在 http://localhost:8080 启动"
echo "🔗 请确保后端服务运行在 http://localhost:6872"
echo ""

# 检查 Python 是否可用
if command -v python3 &> /dev/null; then
    echo "✅ 使用 Python3 启动服务器..."
    python3 -m http.server 8080
elif command -v python &> /dev/null; then
    echo "✅ 使用 Python 启动服务器..."
    python -m http.server 8080
elif command -v node &> /dev/null; then
    echo "✅ 使用 Node.js 启动服务器..."
    npx http-server -p 8080
else
    echo "❌ 错误: 未找到 Python 或 Node.js"
    echo "请安装 Python3 或 Node.js 来运行此项目"
    exit 1
fi
