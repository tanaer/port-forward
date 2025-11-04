# 多阶段构建优化镜像大小
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git ca-certificates

# 复制依赖文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用（启用CGO用于SQLite）
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o goforward .

# 运行时阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates sqlite tzdata && \
    apk add --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/main sqlite

# 创建非root用户
RUN adduser -D -s /bin/sh goforward

# 设置工作目录
WORKDIR /home/goforward

# 从构建阶段复制二进制文件
COPY --from=builder /app/goforward .

# 创建数据目录
RUN mkdir -p /home/goforward/data

# 设置权限
RUN chown -R goforward:goforward /home/goforward

# 切换到非root用户
USER goforward

# 暴露端口
EXPOSE 8889

# 设置环境变量
ENV WEB_PORT=8889
ENV DATA_DIR=/home/goforward/data

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${WEB_PORT}/ || exit 1

# 启动命令
CMD ["./goforward"]