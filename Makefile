.PHONY: help build run test clean deps fmt vet lint

# 默认目标
help:
	@echo "Cinexus 项目管理命令:"
	@echo "  build    - 构建可执行文件"
	@echo "  run      - 运行开发服务器"
	@echo "  test     - 运行测试"
	@echo "  clean    - 清理构建文件和日志"
	@echo "  deps     - 下载依赖"
	@echo "  fmt      - 格式化代码"
	@echo "  vet      - 运行 go vet"
	@echo "  lint     - 运行 golangci-lint (需要安装)"

# 构建可执行文件
build:
	@echo "🔨 构建 Cinexus..."
	go build -o bin/cinexus main.go
	@echo "✅ 构建完成: bin/cinexus"

# 运行开发服务器
run:
	@echo "🚀 启动开发服务器..."
	go run main.go server

# 运行测试
test:
	@echo "🧪 运行测试..."
	go test -v ./...

# 清理文件
clean:
	@echo "🧹 清理文件..."
	rm -rf bin/
	rm -rf logs/
	go clean
	@echo "✅ 清理完成"

# 下载依赖
deps:
	@echo "📦 下载依赖..."
	go mod tidy
	go mod download
	@echo "✅ 依赖下载完成"

# 格式化代码
fmt:
	@echo "🎨 格式化代码..."
	go fmt ./...
	@echo "✅ 代码格式化完成"

# 运行 go vet
vet:
	@echo "🔍 运行 go vet..."
	go vet ./...
	@echo "✅ go vet 检查完成"

# 运行 golangci-lint
lint:
	@echo "📋 运行 golangci-lint..."
	golangci-lint run
	@echo "✅ lint 检查完成"

# 快速开发
dev: deps fmt vet run

# 完整检查
check: deps fmt vet test

# 发布构建
release: clean deps fmt vet test build
	@echo "🎉 发布构建完成!"