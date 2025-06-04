package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"
	"time"
)

// Token115 定义 115open 的 token 结构
type Token115 struct {
	RefreshToken string    `json:"refresh_token"`
	AccessToken  string    `json:"access_token"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TokenRefresher 接口，用于检查是否正在刷新
type TokenRefresher interface {
	IsRefreshing() bool
	WaitForRefreshComplete()
}

var (
	DataDir   = "./data"
	TokenFile = "115_tokens.json"
	// 全局互斥锁，保护同一进程内的并发访问
	tokenMutex sync.Mutex
	// 文件锁超时时间 - 缩短超时时间以避免长时间阻塞
	FileLockTimeout = 5 * time.Second
	// 全局token刷新器引用
	globalRefresher TokenRefresher
)

// SetTokenRefresher 设置全局token刷新器
func SetTokenRefresher(refresher TokenRefresher) {
	globalRefresher = refresher
}

// getTokenPath 获取完整的 token 文件路径
func getTokenPath() string {
	return DataDir + "/" + TokenFile
}

// getLockPath 获取锁文件路径
func getLockPath() string {
	return DataDir + "/" + TokenFile + ".lock"
}

// waitForRefreshIfNeeded 如果正在刷新，等待刷新完成
func waitForRefreshIfNeeded() {
	if globalRefresher != nil && globalRefresher.IsRefreshing() {
		log.Println("检测到正在进行token刷新，等待刷新完成...")
		startTime := time.Now()
		globalRefresher.WaitForRefreshComplete()
		duration := time.Since(startTime)
		log.Printf("token刷新等待完成，耗时: %v", duration)
	}
}

// acquireFileLock 获取文件锁，防止跨进程并发修改（带超时）
func acquireFileLock() (*os.File, error) {
	// 如果超时时间为0，使用非阻塞模式
	if FileLockTimeout == 0 {
		return acquireFileLockNonBlocking()
	}
	// 否则使用带超时的阻塞模式
	return acquireFileLockWithTimeout(FileLockTimeout)
}

// acquireFileLockWithTimeout 获取文件锁，带自定义超时时间
func acquireFileLockWithTimeout(timeout time.Duration) (*os.File, error) {
	// 确保数据目录存在
	if err := EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	lockFile, err := os.OpenFile(getLockPath(), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("创建锁文件失败: %w", err)
	}

	// 使用带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 在单独的协程中尝试获取锁
	lockChan := make(chan error, 1)
	go func() {
		// 尝试获取独占锁（阻塞模式）
		err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX)
		lockChan <- err
	}()

	// 等待锁获取成功或超时
	select {
	case err := <-lockChan:
		if err != nil {
			lockFile.Close()
			return nil, fmt.Errorf("获取文件锁失败: %w", err)
		}
		return lockFile, nil
	case <-ctx.Done():
		// 超时，尝试关闭文件并返回错误
		lockFile.Close()
		return nil, fmt.Errorf("获取文件锁超时 (等待了 %v)，可能有其他进程正在使用", timeout)
	}
}

// acquireFileLockNonBlocking 非阻塞方式获取文件锁
func acquireFileLockNonBlocking() (*os.File, error) {
	// 确保数据目录存在
	if err := EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	lockFile, err := os.OpenFile(getLockPath(), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("创建锁文件失败: %w", err)
	}

	// 尝试获取非阻塞锁
	err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		lockFile.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, fmt.Errorf("文件锁被占用，其他进程正在修改 tokens")
		}
		return nil, fmt.Errorf("获取文件锁失败: %w", err)
	}

	return lockFile, nil
}

// releaseFileLock 释放文件锁
func releaseFileLock(lockFile *os.File) error {
	if lockFile == nil {
		return nil
	}

	// 释放锁
	err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	if err != nil {
		lockFile.Close()
		return fmt.Errorf("释放文件锁失败: %w", err)
	}

	return lockFile.Close()
}

// EnsureDataDir 确保 /data 目录存在
func EnsureDataDir() error {
	if _, err := os.Stat(DataDir); os.IsNotExist(err) {
		return os.MkdirAll(DataDir, 0755)
	}
	return nil
}

// readTokensInternal 内部读取函数，不等待刷新完成，用于避免死锁
func readTokensInternal() (*Token115, error) {
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

// ReadTokens 从 JSON 文件读取 115 tokens
func ReadTokens() (*Token115, error) {
	// 如果正在刷新，等待刷新完成
	waitForRefreshIfNeeded()

	return readTokensInternal()
}

// ReadTokensForRefresh 专门用于刷新过程中读取token，不等待刷新完成
func ReadTokensForRefresh() (*Token115, error) {
	return readTokensInternal()
}

// WriteTokens 将 115 tokens 写入 JSON 文件（带锁保护）
func WriteTokens(refreshToken, accessToken string) error {
	// 获取进程内锁
	tokenMutex.Lock()
	defer tokenMutex.Unlock()

	// 获取文件锁
	lockFile, err := acquireFileLock()
	if err != nil {
		return fmt.Errorf("获取文件锁失败: %w", err)
	}
	defer func() {
		if releaseErr := releaseFileLock(lockFile); releaseErr != nil {
			fmt.Printf("警告: 释放文件锁失败: %v\n", releaseErr)
		}
	}()

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

// UpdateTokens 更新现有的 tokens，只更新非空值（带锁保护）
func UpdateTokens(refreshToken, accessToken string) error {
	// 获取进程内锁
	tokenMutex.Lock()
	defer tokenMutex.Unlock()

	// 获取文件锁
	lockFile, err := acquireFileLock()
	if err != nil {
		return fmt.Errorf("获取文件锁失败: %w", err)
	}
	defer func() {
		if releaseErr := releaseFileLock(lockFile); releaseErr != nil {
			fmt.Printf("警告: 释放文件锁失败: %v\n", releaseErr)
		}
	}()

	// 先读取现有的 tokens（使用内部方法，避免死锁）
	existingTokens, err := readTokensInternal()
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

// GetTokensForRefresh 专门用于刷新过程中获取token，不等待刷新完成
func GetTokensForRefresh() (refreshToken, accessToken string, updatedAt time.Time, err error) {
	tokens, err := ReadTokensForRefresh()
	if err != nil {
		return "", "", time.Time{}, err
	}

	return tokens.RefreshToken, tokens.AccessToken, tokens.UpdatedAt, nil
}
