package storage

import (
	"fmt"
	"time"
)

// ExampleUsage 展示如何使用 token 存储功能
func ExampleUsage() {
	// 1. 写入新的 tokens
	err := WriteTokens("your_refresh_token_here", "your_access_token_here")
	if err != nil {
		fmt.Printf("写入 tokens 失败: %v\n", err)
		return
	}
	fmt.Println("✅ Tokens 写入成功")

	// 2. 读取 tokens
	tokens, err := ReadTokens()
	if err != nil {
		fmt.Printf("读取 tokens 失败: %v\n", err)
		return
	}
	fmt.Printf("📖 读取到的 tokens: RefreshToken=%s, AccessToken=%s, UpdatedAt=%s\n",
		tokens.RefreshToken, tokens.AccessToken, tokens.UpdatedAt.Format("2006-01-02 15:04:05"))

	// 3. 获取单独的 token 值
	refreshToken, accessToken, updatedAt, err := GetTokens()
	if err != nil {
		fmt.Printf("获取 tokens 失败: %v\n", err)
		return
	}
	fmt.Printf("🔑 RefreshToken: %s\n", refreshToken)
	fmt.Printf("🔑 AccessToken: %s\n", accessToken)
	fmt.Printf("⏰ 最后更新时间: %s\n", updatedAt.Format("2006-01-02 15:04:05"))

	// 4. 只更新 AccessToken
	err = UpdateTokens("", "new_access_token_here")
	if err != nil {
		fmt.Printf("更新 AccessToken 失败: %v\n", err)
		return
	}
	fmt.Println("✅ AccessToken 更新成功")

	// 5. 检查 token 是否有效（24小时内更新的视为有效）
	valid, err := IsTokenValid(24 * time.Hour)
	if err != nil {
		fmt.Printf("检查 token 有效性失败: %v\n", err)
		return
	}
	if valid {
		fmt.Println("✅ Token 仍然有效")
	} else {
		fmt.Println("❌ Token 已过期或无效")
	}
}
