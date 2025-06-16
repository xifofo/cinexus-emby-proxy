# Cinexus Emby Proxy

一个高性能的 Emby 媒体服务器代理工具，支持 Alist、115Open

## Docker 部署指南

本文档介绍如何使用 Docker 部署 Cinexus Emby Proxy 应用。

### 部署步骤

#### 1. 准备配置文件

首先复制示例配置文件 config.yaml 并修改：
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

#### 1. 115open

> 115open token 需要通过命令行设置

```bash
docker exec -it cinexus-emby-proxy bash

# 进入容器后
./cinexus token set --refresh-token "xxxx" --access-token "xxx"
```

> 进入 Docker 容器内执行 `./cinexus token set --refresh-token "xxxx" --access-token "xxx"` 设置 Token

> 利用 Cookie + 115open API 的方案。配置了 Alist 之后会降级到 AList 302 方案
