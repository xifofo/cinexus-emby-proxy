package middleware

import (
	"time"

	"cinexus/internal/logger"

	"github.com/labstack/echo/v4"
)

// Logger 返回记录 HTTP 请求的中间件
func Logger(log *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			end := time.Now()
			latency := end.Sub(start)

			req := c.Request()
			res := c.Response()

			fields := map[string]interface{}{
				"method":     req.Method,
				"path":       req.URL.Path,
				"status":     res.Status,
				"latency":    latency.String(),
				"request_id": res.Header().Get(echo.HeaderXRequestID),
				"remote_ip":  c.RealIP(),
				"user_agent": req.UserAgent(),
				"bytes_in":   req.Header.Get(echo.HeaderContentLength),
				"bytes_out":  res.Size,
			}

			if err != nil {
				fields["error"] = err.Error()
				log.WithFields(fields).Error("请求完成时出错")
			} else {
				if res.Status >= 500 {
					log.WithFields(fields).Error("请求完成")
				} else if res.Status >= 400 {
					log.WithFields(fields).Warn("请求完成")
				} else {
					log.WithFields(fields).Info("请求完成")
				}
			}

			return err
		}
	}
}