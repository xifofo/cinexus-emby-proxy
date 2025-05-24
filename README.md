# Cinexus

基于 Echo、Cobra 和 Viper 构建的现代化 Go Web 应用框架，支持 Emby 媒体服务器代理功能。

## 特性

- 🚀 基于 Echo v4 的高性能 Web 框架
- 🔧 使用 Cobra 的强大 CLI 支持
- ⚙️ 通过 Viper 的灵活配置管理
- 📝 结构化日志记录，支持按天分割和自动清理
- 🛡️ 内置中间件：CORS、日志、恢复、请求ID等
- 🔄 优雅的服务器关闭
- 📊 健康检查端点
- 🎬 Emby 媒体服务器代理功能

## 项目结构

```
cinexus/
├── cmd/                    # Cobra 命令定义
│   ├── root.go            # 根命令
│   └── server.go          # 服务器启动命令
├── internal/              # 内部包
│   ├── config/            # 配置管理
│   ├── logger/            # 日志管理
│   └── server/            # 服务器相关
│       ├── middleware/    # 中间件
│       └── routes/        # 路由定义
├── logs/                  # 日志文件目录
├── config.yaml           # 配置文件
├── go.mod                 # Go 模块文件
├── main.go               # 主程序入口
└── README.md             # 项目说明
```

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置项目

复制示例配置文件并修改：

```bash
cp config.example.yaml config.yaml
```

然后编辑 `config.yaml` 文件，配置你的 Emby 服务器地址和 API 密钥。

### 3. 运行服务器

```bash
# 使用默认配置启动服务器
go run main.go server

# 或者指定配置文件
go run main.go server --config config.yaml
```

### 4. 测试 API

```bash
# 健康检查
curl http://localhost:9096/health

# Hello API
curl http://localhost:9096/api/v1/hello?name=世界

# Echo API
curl -X POST http://localhost:9096/api/v1/echo \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello, World!"}'
```

## 配置说明

项目使用 YAML 格式的配置文件，支持以下配置项：

### 服务器配置

```yaml
server:
  port: "8080"        # 服务器端口
  mode: "debug"       # 运行模式：debug/release
```

### 代理配置

```yaml
proxy:
  url: "http://192.168.0.2:8096"          # Emby 服务器地址
  api_key: "your_emby_api_key_here"       # Emby API 密钥
```

### 日志配置

```yaml
log:
  level: "info"           # 日志级别：debug/info/warn/error
  format: "text"          # 日志格式：json/text
  output: "file"          # 输出方式：stdout/file
  file_path: "logs/app.log"  # 日志文件路径
  max_size: 100           # 单个日志文件最大大小（MB）
  max_backups: 0          # 保留的备份文件数量（0表示不限制）
  max_age: 7              # 日志文件保留天数
  compress: true          # 是否压缩旧日志文件
```

## 日志管理

项目使用 `lumberjack` 实现日志的自动分割和清理：

- 📅 **按天分割**：当日志文件超过设定大小时自动分割
- 🗂️ **自动清理**：保留指定天数的日志文件，自动删除过期文件
- 🗜️ **压缩存储**：可选择压缩旧日志文件以节省空间
- 🔄 **格式支持**：支持 JSON 和文本格式的日志输出

## 中间件

框架内置了以下中间件：

- **Recovery**：自动恢复 panic
- **CORS**：跨域资源共享支持
- **Request ID**：为每个请求生成唯一 ID
- **Logger**：结构化请求日志记录
- **Timeout**：请求超时控制

## API 端点

### 健康检查

```
GET /health
```

返回服务器健康状态。

### Hello API

```
GET /api/v1/hello?name=<name>
```

简单的问候 API，支持自定义名称参数。

### Echo API

```
POST /api/v1/echo
Content-Type: application/json

{
  "key": "value"
}
```

回显请求数据的 API。

### 代理功能

所有不匹配 `/health` 和 `/api/v1/*` 的请求都会被代理到配置的 Emby 服务器。

- 自动转发所有 HTTP 方法（GET、POST、PUT、DELETE 等）
- 保持原始请求头
- 自动添加 Emby API Token（如果配置了）
- 透明地转发响应头和响应体
- 完整的错误处理和日志记录

## 扩展开发

### 添加新的路由

在 `internal/server/routes/routes.go` 中添加新的路由：

```go
func setupAPIRoutes(g *echo.Group, log *logger.Logger) {
    g.GET("/hello", helloHandler(log))
    g.POST("/echo", echoHandler(log))
    // 添加你的新路由
    g.GET("/newapi", newAPIHandler(log))
}
```

### 添加新的中间件

在 `internal/server/middleware/` 目录下创建新的中间件文件。

### 修改配置

在 `internal/config/config.go` 中添加新的配置选项。

## 构建和部署

### 构建二进制文件

```bash
go build -o cinexus main.go
```

### 运行

```bash
./cinexus server
```

## 许可证

[MIT License](LICENSE)