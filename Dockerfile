# 使用多阶段构建来优化镜像大小
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的系统依赖和编译工具
RUN apk add --no-cache git ca-certificates tzdata sqlite bash gcc musl-dev

# 复制 go mod 文件
COPY go.mod go.sum ./

# 设置代理
ENV GOPROXY=https://goproxy.cn,direct

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用（启用 CGO 以支持 sqlite3）
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o cinexus main.go

# 运行阶段
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata sqlite bash

# 设置时区
ENV TZ=Asia/Shanghai

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/cinexus .

# 复制配置文件模板
COPY config.example.yaml ./config.example.yaml

# 启动应用
CMD ["./cinexus", "server"]
