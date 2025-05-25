# Token Storage 模块

这个模块提供了 115open 的 RefreshToken 和 AccessToken 的持久化存储功能。

## 功能特性

- 🔐 安全存储 115open 的 RefreshToken 和 AccessToken
- ⏰ 自动记录最后更新时间
- 📁 自动创建 `/data` 目录
- ✅ 检查 token 有效性
- 🔄 支持部分更新（只更新其中一个 token）

## 存储位置

- 文件路径: `/data/115_tokens.json`
- 文件格式: JSON

## JSON 结构

```json
{
  "refresh_token": "your_refresh_token_here",
  "access_token": "your_access_token_here",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

## API 方法

### WriteTokens(refreshToken, accessToken string) error
写入新的 tokens，会覆盖现有的所有值。

```go
err := storage.WriteTokens("new_refresh_token", "new_access_token")
if err != nil {
    log.Printf("写入失败: %v", err)
}
```

### ReadTokens() (*Token115, error)
读取完整的 token 结构。

```go
tokens, err := storage.ReadTokens()
if err != nil {
    log.Printf("读取失败: %v", err)
    return
}
fmt.Printf("RefreshToken: %s\n", tokens.RefreshToken)
fmt.Printf("AccessToken: %s\n", tokens.AccessToken)
fmt.Printf("更新时间: %s\n", tokens.UpdatedAt)
```

### GetTokens() (refreshToken, accessToken string, updatedAt time.Time, err error)
分别获取 token 值。

```go
refreshToken, accessToken, updatedAt, err := storage.GetTokens()
if err != nil {
    log.Printf("获取失败: %v", err)
    return
}
```

### UpdateTokens(refreshToken, accessToken string) error
部分更新 tokens，只更新非空值。

```go
// 只更新 AccessToken，保持 RefreshToken 不变
err := storage.UpdateTokens("", "new_access_token")

// 只更新 RefreshToken，保持 AccessToken 不变
err := storage.UpdateTokens("new_refresh_token", "")

// 同时更新两者
err := storage.UpdateTokens("new_refresh_token", "new_access_token")
```

### IsTokenValid(maxAge time.Duration) (bool, error)
检查 token 是否仍然有效（基于更新时间）。

```go
// 检查 token 是否在24小时内更新过
valid, err := storage.IsTokenValid(24 * time.Hour)
if err != nil {
    log.Printf("检查失败: %v", err)
    return
}

if valid {
    fmt.Println("Token 仍然有效")
} else {
    fmt.Println("Token 已过期，需要刷新")
}
```

## 使用示例

```go
package main

import (
    "cinexus/internal/storage"
    "fmt"
    "time"
)

func main() {
    // 初次写入 tokens
    err := storage.WriteTokens("initial_refresh_token", "initial_access_token")
    if err != nil {
        fmt.Printf("写入失败: %v\n", err)
        return
    }

    // 检查是否有效（1小时内）
    valid, err := storage.IsTokenValid(time.Hour)
    if err != nil {
        fmt.Printf("检查失败: %v\n", err)
        return
    }

    if !valid {
        fmt.Println("Token 已过期，需要刷新")
        // 刷新 token 逻辑...
        err = storage.UpdateTokens("new_refresh_token", "new_access_token")
        if err != nil {
            fmt.Printf("更新失败: %v\n", err)
            return
        }
    }

    // 获取当前有效的 tokens
    refreshToken, accessToken, _, err := storage.GetTokens()
    if err != nil {
        fmt.Printf("获取失败: %v\n", err)
        return
    }

    fmt.Printf("当前 RefreshToken: %s\n", refreshToken)
    fmt.Printf("当前 AccessToken: %s\n", accessToken)
}
```

## 运行测试

```bash
go test ./internal/storage -v
```

## 注意事项

1. 确保程序有权限写入 `/data` 目录
2. token 文件权限为 `0644`，目录权限为 `0755`
3. 如果文件不存在，`ReadTokens()` 会返回空的结构体而不是错误
4. `UpdateTokens()` 传入空字符串的参数不会被更新