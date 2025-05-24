package server

import (
	"context"
	"net/http"

	"cinexus/internal/config"
	"cinexus/internal/logger"
	"cinexus/internal/server/routes"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// Server 表示 HTTP 服务器
type Server struct {
	echo   *echo.Echo
	config *config.Config
	logger *logger.Logger
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

	return s
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
	return s.echo.Shutdown(ctx)
}
