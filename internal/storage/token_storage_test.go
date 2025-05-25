package storage

import (
	"os"
	"testing"
	"time"
)

func TestTokenStorage(t *testing.T) {
	// 使用临时目录进行测试
	tempDir := "/tmp/test_data"
	originalDataDir := DataDir

	// 修改变量为测试目录
	DataDir = tempDir

	// 测试完成后清理
	defer func() {
		os.RemoveAll(tempDir)
		DataDir = originalDataDir
	}()

	// 测试写入和读取
	testRefreshToken := "test_refresh_token_123"
	testAccessToken := "test_access_token_456"

	// 1. 测试写入 tokens
	err := WriteTokens(testRefreshToken, testAccessToken)
	if err != nil {
		t.Fatalf("WriteTokens 失败: %v", err)
	}

	// 2. 测试读取 tokens
	tokens, err := ReadTokens()
	if err != nil {
		t.Fatalf("ReadTokens 失败: %v", err)
	}

	if tokens.RefreshToken != testRefreshToken {
		t.Errorf("RefreshToken 不匹配. 期望: %s, 实际: %s", testRefreshToken, tokens.RefreshToken)
	}

	if tokens.AccessToken != testAccessToken {
		t.Errorf("AccessToken 不匹配. 期望: %s, 实际: %s", testAccessToken, tokens.AccessToken)
	}

	if tokens.UpdatedAt.IsZero() {
		t.Error("UpdatedAt 应该被设置")
	}

	// 3. 测试 GetTokens
	refreshToken, accessToken, updatedAt, err := GetTokens()
	if err != nil {
		t.Fatalf("GetTokens 失败: %v", err)
	}

	if refreshToken != testRefreshToken {
		t.Errorf("GetTokens RefreshToken 不匹配. 期望: %s, 实际: %s", testRefreshToken, refreshToken)
	}

	if accessToken != testAccessToken {
		t.Errorf("GetTokens AccessToken 不匹配. 期望: %s, 实际: %s", testAccessToken, accessToken)
	}

	if updatedAt.IsZero() {
		t.Error("GetTokens UpdatedAt 应该被设置")
	}

	// 4. 测试 UpdateTokens
	newAccessToken := "new_access_token_789"
	err = UpdateTokens("", newAccessToken)
	if err != nil {
		t.Fatalf("UpdateTokens 失败: %v", err)
	}

	// 验证更新结果
	tokens, err = ReadTokens()
	if err != nil {
		t.Fatalf("ReadTokens after update 失败: %v", err)
	}

	if tokens.RefreshToken != testRefreshToken {
		t.Errorf("更新后 RefreshToken 应该保持不变. 期望: %s, 实际: %s", testRefreshToken, tokens.RefreshToken)
	}

	if tokens.AccessToken != newAccessToken {
		t.Errorf("更新后 AccessToken 不匹配. 期望: %s, 实际: %s", newAccessToken, tokens.AccessToken)
	}

	// 5. 测试 IsTokenValid
	valid, err := IsTokenValid(time.Hour)
	if err != nil {
		t.Fatalf("IsTokenValid 失败: %v", err)
	}

	if !valid {
		t.Error("Token 应该是有效的")
	}

	// 测试过期情况
	valid, err = IsTokenValid(time.Nanosecond)
	if err != nil {
		t.Fatalf("IsTokenValid (expired) 失败: %v", err)
	}

	if valid {
		t.Error("Token 应该是过期的")
	}
}
