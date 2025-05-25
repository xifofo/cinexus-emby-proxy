package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Token115 定义 115open 的 token 结构
type Token115 struct {
	RefreshToken string    `json:"refresh_token"`
	AccessToken  string    `json:"access_token"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var (
	DataDir   = "/data"
	TokenFile = "115_tokens.json"
)

// getTokenPath 获取完整的 token 文件路径
func getTokenPath() string {
	return DataDir + "/" + TokenFile
}

// EnsureDataDir 确保 /data 目录存在
func EnsureDataDir() error {
	if _, err := os.Stat(DataDir); os.IsNotExist(err) {
		return os.MkdirAll(DataDir, 0755)
	}
	return nil
}

// ReadTokens 从 JSON 文件读取 115 tokens
func ReadTokens() (*Token115, error) {
	// 确保数据目录存在
	if err := EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(getTokenPath()); os.IsNotExist(err) {
		// 如果文件不存在，返回空的 token 结构
		return &Token115{}, nil
	}

	// 读取文件内容
	data, err := os.ReadFile(getTokenPath())
	if err != nil {
		return nil, fmt.Errorf("读取 token 文件失败: %w", err)
	}

	// 解析 JSON
	var tokens Token115
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("解析 token JSON 失败: %w", err)
	}

	return &tokens, nil
}

// WriteTokens 将 115 tokens 写入 JSON 文件
func WriteTokens(refreshToken, accessToken string) error {
	// 确保数据目录存在
	if err := EnsureDataDir(); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 创建 token 结构
	tokens := Token115{
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
		UpdatedAt:    time.Now(),
	}

	// 序列化为 JSON
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 token 为 JSON 失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(getTokenPath(), data, 0644); err != nil {
		return fmt.Errorf("写入 token 文件失败: %w", err)
	}

	return nil
}

// UpdateTokens 更新现有的 tokens，只更新非空值
func UpdateTokens(refreshToken, accessToken string) error {
	// 先读取现有的 tokens
	existingTokens, err := ReadTokens()
	if err != nil {
		return fmt.Errorf("读取现有 tokens 失败: %w", err)
	}

	// 只更新非空的值
	if refreshToken != "" {
		existingTokens.RefreshToken = refreshToken
	}
	if accessToken != "" {
		existingTokens.AccessToken = accessToken
	}

	// 更新时间戳
	existingTokens.UpdatedAt = time.Now()

	// 序列化为 JSON
	data, err := json.MarshalIndent(existingTokens, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 token 为 JSON 失败: %w", err)
	}

	// 确保数据目录存在
	if err := EnsureDataDir(); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(getTokenPath(), data, 0644); err != nil {
		return fmt.Errorf("写入 token 文件失败: %w", err)
	}

	return nil
}

// IsTokenValid 检查 token 是否有效（基于更新时间判断是否过期）
func IsTokenValid(maxAge time.Duration) (bool, error) {
	tokens, err := ReadTokens()
	if err != nil {
		return false, err
	}

	// 如果没有 token 或 token 为空
	if tokens.RefreshToken == "" || tokens.AccessToken == "" {
		return false, nil
	}

	// 如果更新时间为零值，说明是新创建的空结构
	if tokens.UpdatedAt.IsZero() {
		return false, nil
	}

	// 检查是否过期
	return time.Since(tokens.UpdatedAt) < maxAge, nil
}

// GetTokens 获取当前的 tokens
func GetTokens() (refreshToken, accessToken string, updatedAt time.Time, err error) {
	tokens, err := ReadTokens()
	if err != nil {
		return "", "", time.Time{}, err
	}

	return tokens.RefreshToken, tokens.AccessToken, tokens.UpdatedAt, nil
}
