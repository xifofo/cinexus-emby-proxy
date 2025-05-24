# Cinexus Emby Proxy

一个高性能的 Emby 媒体服务器代理工具，支持 Alist、115Open

## Docker 部署指南

本文档介绍如何使用 Docker 部署 Cinexus Emby Proxy 应用。

### 文件说明

- `Dockerfile` - Docker 镜像构建文件
- `.dockerignore` - Docker 构建时忽略的文件和目录
- `docker-compose.yml` - Docker Compose 配置文件

### 部署步骤

#### 1. 准备配置文件

首先复制示例配置文件并修改：

```bash
cp config.example.yaml config.yaml
```

编辑 `config.yaml` 文件，配置你的 Emby 服务器地址、API 密钥等参数。

#### 2. 使用 Docker Compose（推荐）

```bash
# 构建并启动服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f cinexus

# 停止服务
docker-compose down
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
