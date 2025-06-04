# 115 Token 自动刷新器

## 概述

Cinexus 项目内置了一个智能的 115 Token 自动刷新器，它会在项目启动时自动运行，无需额外配置或单独启动。

## 特性

✅ **自动启动** - 与主程序一起启动，无需额外操作
✅ **智能检查** - 基于 `updated_at` 时间判断是否需要刷新
✅ **线程安全** - 支持并发访问，读取时自动等待刷新完成
✅ **优雅关闭** - 程序关闭时自动停止刷新器
✅ **错误处理** - 完善的错误处理和日志记录

## 工作原理

### 1. 自动启动

当你启动服务器时，token 刷新器会自动启动：

```bash
./cinexus server
```

启动日志会显示：

```
🔄 Token刷新器已启动，检查间隔: 10m0s, 最大有效期: 1h20m0s
```

### 2. 智能检查机制

刷新器采用智能检查机制：

- **检查间隔**: 每 10 分钟检查一次 token 状态
- **有效期判断**: Token 有效期设为 1 小时 20 分钟
- **按需刷新**: 只在 token 即将过期时才进行刷新

### 3. 等待机制

当其他代码读取 token 时：

```go
// 这个调用会自动等待刷新完成（如果正在刷新）
tokens, err := storage.GetTokens()
```

- 如果正在刷新，读取操作会等待刷新完成
- 如果没有刷新，立即返回当前 token
- 保证读取到的 token 总是最新的

## 配置参数

当前使用的默认配置：

```go
refresherConfig := tokenrefresher.Config{
    CheckInterval: 10 * time.Minute, // 每10分钟检查一次
    MaxAge:        80 * time.Minute, // token有效期1小时20分钟
}
```

### 参数说明

- **CheckInterval**: 多久检查一次 token 是否需要刷新
- **MaxAge**: Token 的最大有效期，超过此时间将触发刷新

## 使用方法

### 1. 基本使用

启动服务器，刷新器会自动运行：

```bash
./cinexus server
```

### 2. 查看 Token 状态

```bash
./cinexus token show
```

### 3. 手动刷新（如果需要）

```bash
./cinexus token refresh
```

## 日志监控

### 查看刷新器日志

刷新器的日志会记录在应用日志中：

```bash
# 查看实时日志
tail -f logs/app.log

# 过滤刷新器相关日志
grep "Token" logs/app.log
```

### 日志示例

**启动日志**:

```
🔄 Token刷新器已启动，检查间隔: 10m0s, 最大有效期: 1h20m0s
```

**检查日志**:

```
✅ Token仍然有效，剩余有效时间: 45m0s
```

**刷新日志**:

```
⚠️  Token已过期或即将过期，开始刷新...
🔄 开始刷新115 token...
🔄 收到新的token，正在保存...
✅ 新token保存成功
✅ Token刷新成功！更新时间: 2025-06-03 21:45:30
```

## 故障排除

### 1. Token 刷新失败

**可能原因**:

- 网络连接问题
- refresh_token 已过期
- 115 API 服务异常

**解决方法**:

```bash
# 1. 检查网络连接
curl -I https://115.com

# 2. 查看错误日志
grep "ERROR" logs/app.log

# 3. 手动刷新测试
./cinexus token refresh

# 4. 重新设置 token（如果必要）
./cinexus token set --refresh-token "new_token" --access-token "new_access_token"
```

### 2. 刷新器未启动

**检查方法**:

```bash
# 查看启动日志
grep "Token刷新器" logs/app.log
```

如果没有看到启动日志，可能是：

- 服务器启动失败
- 配置问题

### 3. Token 读取缓慢

**可能原因**:

- 正在进行 token 刷新
- 网络或 API 响应慢

**这是正常行为**，系统会等待刷新完成后返回最新的 token。

## 高级配置

如果需要修改刷新器配置，可以编辑 `internal/server/server.go` 中的配置：

```go
// setupTokenRefresher 设置token刷新器
func (s *Server) setupTokenRefresher() {
    // 创建token刷新器配置
    refresherConfig := tokenrefresher.Config{
        CheckInterval: 5 * time.Minute,  // 改为每5分钟检查一次
        MaxAge:        60 * time.Minute, // 改为1小时有效期
    }
    // ... 其余代码
}
```

## 最佳实践

1. **监控日志**: 定期查看日志确保刷新器正常工作
2. **备份 Token**: 定期备份 `data/115_tokens.json` 文件
3. **网络稳定**: 确保服务器网络连接稳定
4. **及时更新**: 如果 refresh_token 过期，及时手动更新

## API 使用示例

如果你在代码中需要使用 token：

```go
package main

import (
    "cinexus/internal/storage"
    "fmt"
)

func main() {
    // 获取 token（会自动等待刷新完成）
    refreshToken, accessToken, updatedAt, err := storage.GetTokens()
    if err != nil {
        fmt.Printf("获取 token 失败: %v\n", err)
        return
    }

    fmt.Printf("Refresh Token: %s\n", refreshToken)
    fmt.Printf("Access Token: %s\n", accessToken)
    fmt.Printf("更新时间: %s\n", updatedAt.Format("2006-01-02 15:04:05"))
}
```

## 总结

115 Token 自动刷新器提供了：

- 🚀 **零配置启动** - 与主程序一起自动启动
- 🧠 **智能刷新** - 只在需要时刷新，避免不必要的 API 调用
- 🔒 **线程安全** - 支持并发访问，确保数据一致性
- 📋 **详细日志** - 完整记录刷新过程，便于调试
- ⚡ **高性能** - 最小化对主程序性能的影响

这确保了你的 115 Token 始终保持有效，无需手动干预！
