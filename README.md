# Cinexus Emby Proxy

一个高性能的 Emby 媒体服务器代理工具，支持 Alist、115Open

## Docker 部署指南

本文档介绍如何使用 Docker 部署 Cinexus Emby Proxy 应用。

### 部署步骤

#### 1. 准备配置文件

首先复制示例配置文件 config.yaml 并修改：

```yaml
server:
  port: "9096"
  mode: "release" # debug, release

proxy:
  url: "http://127.0.0.1:8096"
  api_key: "your_emby_api_key_here"
  cache_time: 1 # 缓存直链时间，单位：小时
  add_metadata: false # 是否在播放时利用 emby 补充元数据
  method: "alist" # alist, 115open（TODO: 115open 直链功能未实现）
  # 路径映射，用于将 Emby 的原始路径映射到代理的实际路径
  paths:
    - old: "/vol1/1000/CloudNAS/CloudDrive/115"
      new: "/115"

# 使用 alist 直链时，需要配置以下参数
alist:
  url: "http://127.0.0.1:5244"
  api_key: "your_alist_api_key_here"
  sign: true # Alist 是否使用签名

log:
  level: "info" # debug, info, warn, error
  format: "text" # json, text
  output: "file" # stdout, file
  file_path: "logs/app.log"
  max_size: 100 # MB
  max_backups: 0 # 0 means no limit
  max_age: 7 # days
  compress: true # compress old log files
```

编辑 `config.yaml` 文件，配置你的 Emby 服务器地址、API 密钥等参数。

#### 2. 使用 Docker Compose（推荐）

```bash
# 构建并启动服务
docker-compose up -d

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f cinexus-emby-proxy

# 停止服务
docker compose down
```

### 配置说明

#### 端口映射

- 容器内端口：9096
- 主机端口：9096（可根据需要修改）

#### 数据卷挂载

- `./config.yaml:/app/config.yaml:ro` - 配置文件（只读）
- `./logs:/app/logs` - 日志目录（读写）

#### 环境变量

- `TZ=Asia/Shanghai` - 设置时区

### 重定向方案

#### 1. ck+115open

> 利用 Cookie + 115open API 的方案。配置了 Alist 之后会降级到 AList 302 方案
