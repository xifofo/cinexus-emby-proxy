package server

import (
	"context"
	"net/http"
	"time"

	"cinexus/internal/config"
	"cinexus/internal/logger"
	"cinexus/internal/server/routes"
	"cinexus/internal/storage"
	"cinexus/internal/tokenrefresher"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// Server 表示 HTTP 服务器
type Server struct {
	echo           *echo.Echo
	config         *config.Config
	logger         *logger.Logger
	tokenRefresher *tokenrefresher.TokenRefresher
}

// New 创建新的服务器实例
func New(cfg *config.Config, log *logger.Logger) *Server {
	e := echo.New()

	// 创建服务器实例
	s := &Server{
		echo:   e,
		config: cfg,
		logger: log,
	}

	// 设置 echo
	s.setupEcho()

	// 设置中间件
	s.setupMiddleware()

	// 设置路由
	s.setupRoutes()

	// 初始化pickcode缓存数据库
	s.setupPickcodeCache()

	// 初始化并启动token刷新器
	s.setupTokenRefresher()

	return s
}

// setupPickcodeCache 初始化pickcode缓存数据库
func (s *Server) setupPickcodeCache() {
	if s.config.Proxy.CachePickcode {
		s.logger.Info("🗄️ 正在初始化pickcode缓存数据库...")
		if err := storage.InitPickcodeDB(); err != nil {
			s.logger.Errorf("❌ 初始化pickcode缓存数据库失败: %v", err)
		} else {
			s.logger.Info("✅ pickcode缓存数据库初始化成功")
			// 获取并显示缓存统计信息
			if count, err := storage.GetPickcodeCacheStats(); err == nil {
				s.logger.Infof("📊 当前缓存中有 %d 个pickcode记录", count)
			}
		}
	} else {
		s.logger.Info("⚠️ pickcode缓存功能已禁用")
	}
}

// setupTokenRefresher 设置token刷新器
func (s *Server) setupTokenRefresher() {
	// 创建token刷新器配置
	refresherConfig := tokenrefresher.Config{
		CheckInterval: 2 * time.Minute,  // 每10分钟检查一次
		MaxAge:        90 * time.Minute, // token有效期1小时30分钟
	}

	// 创建token刷新器
	s.tokenRefresher = tokenrefresher.New(s.logger, refresherConfig)

	// 设置全局token刷新器引用
	storage.SetTokenRefresher(s.tokenRefresher)

	// 启动token刷新器
	s.tokenRefresher.Start()
}

// setupEcho 配置 echo 实例
func (s *Server) setupEcho() {
	// 隐藏 echo 横幅
	s.echo.HideBanner = true

	// 根据配置设置调试模式
	if s.config.Server.Mode == "debug" {
		s.echo.Debug = true
	}

	// 自定义错误处理器
	s.echo.HTTPErrorHandler = s.customErrorHandler
}

// setupMiddleware 配置中间件
func (s *Server) setupMiddleware() {
	// 恢复中间件
	s.echo.Use(echomiddleware.Recover())

	// CORS 中间件
	s.echo.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// 请求 ID 中间件
	s.echo.Use(echomiddleware.RequestID())

	// 自定义日志中间件
	// s.echo.Use(middleware.Logger(s.logger))

	// 请求超时中间件
	// s.echo.Use(echomiddleware.TimeoutWithConfig(echomiddleware.TimeoutConfig{
	// 	Timeout: 30 * 1000000000, // 30 秒
	// }))
}

// setupRoutes 配置应用程序路由
func (s *Server) setupRoutes() {
	routes.Setup(s.echo, s.config, s.logger)
}

// customErrorHandler 处理错误
func (s *Server) customErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := "内部服务器错误"

	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		message = he.Message.(string)
	}

	s.logger.WithFields(map[string]interface{}{
		"error":      err.Error(),
		"status":     code,
		"method":     c.Request().Method,
		"path":       c.Request().URL.Path,
		"request_id": c.Response().Header().Get(echo.HeaderXRequestID),
	}).Error("发生 HTTP 错误")

	if !c.Response().Committed {
		if c.Request().Method == http.MethodHead {
			err = c.NoContent(code)
		} else {
			err = c.JSON(code, map[string]interface{}{
				"error":      message,
				"status":     code,
				"request_id": c.Response().Header().Get(echo.HeaderXRequestID),
			})
		}
		if err != nil {
			s.logger.WithError(err).Error("发送错误响应失败")
		}
	}
}

// Start 启动服务器
func (s *Server) Start(address string) error {
	return s.echo.Start(address)
}

// Shutdown 优雅地关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("🔄 开始关闭服务器组件...")

	// 停止token刷新器
	if s.tokenRefresher != nil {
		s.logger.Info("🛑 正在停止token刷新器...")
		s.tokenRefresher.Stop()
		s.logger.Info("✅ token刷新器已停止")
	}

	s.logger.Info("🛑 正在关闭HTTP服务器...")
	err := s.echo.Shutdown(ctx)
	if err != nil {
		s.logger.Errorf("❌ HTTP服务器关闭失败: %v", err)
		return err
	}

	s.logger.Info("✅ HTTP服务器已关闭")
	return nil
}
