package tokenrefresher

import (
	"context"
	"sync"
	"time"

	"cinexus/internal/logger"
	"cinexus/internal/storage"

	sdk115 "github.com/xhofe/115-sdk-go"
)

// TokenRefresher 负责定期检查和刷新115 tokens
type TokenRefresher struct {
	logger        *logger.Logger
	checkInterval time.Duration
	maxAge        time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
	isRefreshing  bool
}

// Config 刷新器配置
type Config struct {
	CheckInterval time.Duration // 检查间隔，默认10分钟
	MaxAge        time.Duration // Token最大有效期，默认1小时20分钟
}

// New 创建新的token刷新器
func New(logger *logger.Logger, config Config) *TokenRefresher {
	// 设置默认值
	if config.CheckInterval == 0 {
		config.CheckInterval = 10 * time.Minute
	}
	if config.MaxAge == 0 {
		config.MaxAge = 80 * time.Minute // 1小时20分钟
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TokenRefresher{
		logger:        logger,
		checkInterval: config.CheckInterval,
		maxAge:        config.MaxAge,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start 启动token刷新器
func (r *TokenRefresher) Start() {
	r.wg.Add(1)
	go r.run()
	r.logger.Infof("🔄 Token刷新器已启动，检查间隔: %v, 最大有效期: %v", r.checkInterval, r.maxAge)
}

// Stop 停止token刷新器
func (r *TokenRefresher) Stop() {
	r.logger.Info("🛑 正在停止Token刷新器...")

	// 取消上下文
	r.cancel()

	// 使用通道来实现带超时的等待
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	// 等待最多5秒钟
	select {
	case <-done:
		r.logger.Info("🛑 Token刷新器已正常停止")
	case <-time.After(5 * time.Second):
		r.logger.Warn("🛑 Token刷新器停止超时，强制退出")
	}
}

// IsRefreshing 检查是否正在刷新
func (r *TokenRefresher) IsRefreshing() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRefreshing
}

// WaitForRefreshComplete 等待刷新完成（带超时机制）
func (r *TokenRefresher) WaitForRefreshComplete() {
	const maxWaitTime = 60 * time.Second // 最大等待60秒
	const checkInterval = 100 * time.Millisecond

	timeout := time.NewTimer(maxWaitTime)
	defer timeout.Stop()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			// 超时，记录警告并强制退出
			r.logger.Warnf("⚠️  等待Token刷新完成超时（%v），可能刷新过程卡住了", maxWaitTime)
			return
		case <-ticker.C:
			if !r.IsRefreshing() {
				return // 刷新已完成
			}
		case <-r.ctx.Done():
			// 如果整个刷新器被取消，也应该退出等待
			r.logger.Info("🔄 等待Token刷新被取消")
			return
		}
	}
}

// run 运行刷新器主循环
func (r *TokenRefresher) run() {
	defer r.wg.Done()

	// 立即执行一次检查
	r.checkAndRefreshIfNeeded()

	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkAndRefreshIfNeeded()
		case <-r.ctx.Done():
			return
		}
	}
}

// checkAndRefreshIfNeeded 检查token是否需要刷新，如果需要则刷新
func (r *TokenRefresher) checkAndRefreshIfNeeded() {
	// 检查token是否有效
	valid, err := storage.IsTokenValid(r.maxAge)
	if err != nil {
		r.logger.Errorf("❌ 检查token有效性失败: %v", err)
		return
	}

	if valid {
		_, _, updatedAt, err := storage.GetTokens()
		if err != nil {
			r.logger.Errorf("❌ 获取token信息失败: %v", err)
			return
		}

		remainingTime := r.maxAge - time.Since(updatedAt)
		r.logger.Debugf("✅ Token仍然有效，剩余有效时间: %v", remainingTime.Round(time.Minute))
		return
	}

	r.logger.Info("⚠️  Token已过期或即将过期，开始刷新...")
	r.refreshToken()
}

// refreshToken 刷新115 token
func (r *TokenRefresher) refreshToken() {
	r.mu.Lock()
	r.isRefreshing = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.isRefreshing = false
		r.mu.Unlock()
	}()

	r.logger.Info("🔄 开始刷新115 token...")

	// 检查是否已被取消
	select {
	case <-r.ctx.Done():
		r.logger.Info("🔄 Token刷新被取消")
		return
	default:
	}

	// 读取当前token
	tokens, err := storage.ReadTokensForRefresh()
	if err != nil {
		r.logger.Errorf("❌ 读取当前token失败: %v", err)
		return
	}

	if tokens.RefreshToken == "" {
		r.logger.Error("❌ RefreshToken为空，无法刷新")
		return
	}

	// 创建115 SDK客户端
	client := sdk115.New(
		sdk115.WithRefreshToken(tokens.RefreshToken),
		sdk115.WithAccessToken(tokens.AccessToken),
	)

	// 创建一个带超时的上下文，并确保能响应主上下文的取消
	apiCtx, apiCancel := context.WithTimeout(r.ctx, 30*time.Second)
	defer apiCancel()

	// 再次检查是否已被取消
	select {
	case <-r.ctx.Done():
		r.logger.Info("🔄 Token刷新在API调用前被取消")
		return
	default:
	}

	// 使用RefreshToken方法刷新
	r.logger.Debug("📞 调用RefreshToken API...")

	newTokens, err := client.RefreshToken(apiCtx)
	if err != nil {
		// 检查是否是因为取消导致的错误
		if r.ctx.Err() != nil {
			r.logger.Info("🔄 Token刷新因关闭而取消")
			return
		}
		r.logger.Errorf("❌ 刷新token失败: %v", err)
		return
	}

	// 最后检查是否已被取消
	select {
	case <-r.ctx.Done():
		r.logger.Info("🔄 Token刷新在保存前被取消")
		return
	default:
	}

	// 保存新的tokens
	if err := storage.UpdateTokens(newTokens.RefreshToken, newTokens.AccessToken); err != nil {
		r.logger.Errorf("❌ 保存新token失败: %v", err)
		return
	}

	r.logger.Info("✅ Token刷新成功！")
}
