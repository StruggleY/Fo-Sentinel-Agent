# ── 阶段1：构建前端 ──────────────────────────────────────────
FROM node:20-alpine AS frontend
WORKDIR /web
# 直接复制含 node_modules 的完整 web 目录（离线构建）
COPY web/ ./
RUN npx vite build

# ── 阶段2：编译后端 ──────────────────────────────────────────
FROM golang:1.24-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /app/server main.go

# ── 阶段3：最终镜像 ──────────────────────────────────────────
FROM golang:1.24-alpine
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=backend /app/server ./server
COPY --from=frontend /web/dist ./web/dist
COPY docs/ ./docs/
COPY manifest/config/config.docker.yaml ./manifest/config/config.yaml
RUN mkdir -p ./manifest/upload/knowledge

EXPOSE 8001
CMD ["./server"]
