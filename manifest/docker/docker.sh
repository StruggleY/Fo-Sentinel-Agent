#!/bin/bash
# 一键构建并启动所有服务
set -e

cd "$(dirname "$0")"

echo "==> 启动基础设施 + 构建应用镜像..."
docker compose up -d --build

echo "==> 等待应用就绪..."
until docker exec fo-sentinel-agent-app wget -qO- http://localhost:8001/api.json > /dev/null 2>&1; do
  sleep 3
done

echo "==> 部署完成，访问地址："
echo "    后端 API:  http://localhost:8001"
echo "    Swagger:   http://localhost:8001/swagger"
echo "    Attu:      http://localhost:8000"
