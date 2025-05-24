package routes

import (
	"io"
	"net/http"
	"strings"

	"cinexus/internal/config"
	"cinexus/internal/logger"

	"github.com/labstack/echo/v4"
)

// Setup 配置所有应用程序路由
func Setup(e *echo.Echo, cfg *config.Config, log *logger.Logger) {
	// 健康检查端点
	e.GET("/health", healthCheck(log))

	// API 路由组
	api := e.Group("/api/v1")
	setupAPIRoutes(api, log)

	// 代理路由组 - 所有其他请求都通过代理
	if cfg.Proxy.URL != "" {
		setupProxyRoutes(e, cfg, log)
	}

	// 静态文件（如果需要）
	// e.Static("/static", "public")
}

// setupAPIRoutes 配置 API 路由
func setupAPIRoutes(g *echo.Group, log *logger.Logger) {
	// 示例端点
	g.GET("/hello", helloHandler(log))
	g.POST("/echo", echoHandler(log))
}

// setupProxyRoutes 配置代理路由
func setupProxyRoutes(e *echo.Echo, cfg *config.Config, log *logger.Logger) {
	// 代理所有未匹配的路由到 Emby 服务器
	e.Any("/*", proxyHandler(cfg, log))
}

// healthCheck 返回健康检查处理器
func healthCheck(log *logger.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"message": "服务正在运行",
		})
	}
}

// helloHandler 返回简单的问候处理器
func helloHandler(log *logger.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.QueryParam("name")
		if name == "" {
			name = "世界"
		}

		log.WithField("name", name).Info("调用了 Hello 端点")

		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "你好, " + name + "!",
		})
	}
}

// echoHandler 返回回显处理器，返回请求体
func echoHandler(log *logger.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var data map[string]interface{}
		if err := c.Bind(&data); err != nil {
			log.WithError(err).Error("绑定请求数据失败")
			return echo.NewHTTPError(http.StatusBadRequest, "无效的请求数据")
		}

		log.WithField("data", data).Info("调用了 Echo 端点")

		return c.JSON(http.StatusOK, map[string]interface{}{
			"echo": data,
		})
	}
}

// proxyHandler 返回代理处理器，将请求转发到 Emby 服务器
func proxyHandler(cfg *config.Config, log *logger.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		// 跳过API路由和健康检查
		path := c.Request().URL.Path
		if path == "/health" || strings.HasPrefix(path, "/api/v1/") {
			return echo.NewHTTPError(http.StatusNotFound, "Not Found")
		}

		// 创建代理URL
		targetURL := cfg.Proxy.URL + c.Request().URL.Path
		if c.Request().URL.RawQuery != "" {
			targetURL += "?" + c.Request().URL.RawQuery
		}

		// 创建新的请求
		req, err := http.NewRequest(c.Request().Method, targetURL, c.Request().Body)
		if err != nil {
			log.WithError(err).Error("创建代理请求失败")
			return echo.NewHTTPError(http.StatusInternalServerError, "代理请求失败")
		}

		// 复制请求头
		for key, values := range c.Request().Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// 添加 API Key 如果配置了
		if cfg.Proxy.APIKey != "" {
			req.Header.Set("X-Emby-Token", cfg.Proxy.APIKey)
		}

		// 设置正确的 Host 头
		req.Host = ""
		req.Header.Set("Host", req.URL.Host)

		// 发送请求
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.WithError(err).Error("代理请求失败")
			return echo.NewHTTPError(http.StatusBadGateway, "无法连接到后端服务")
		}
		defer resp.Body.Close()

		// 复制响应头
		for key, values := range resp.Header {
			for _, value := range values {
				c.Response().Header().Add(key, value)
			}
		}

		// 设置状态码
		c.Response().WriteHeader(resp.StatusCode)

		// 复制响应体
		_, err = io.Copy(c.Response().Writer, resp.Body)
		if err != nil {
			log.WithError(err).Error("复制响应体失败")
		}

		log.WithFields(map[string]interface{}{
			"method":     c.Request().Method,
			"path":       path,
			"target_url": targetURL,
			"status":     resp.StatusCode,
		}).Info("代理请求完成")

		return nil
	}
}