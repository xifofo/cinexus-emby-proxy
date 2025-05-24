package logger

import (
	"io"
	"os"
	"path/filepath"

	"cinexus/internal/config"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 包装 logrus.Logger
type Logger struct {
	*logrus.Logger
}

// New 使用给定配置创建新的日志记录器实例
func New(cfg config.LogConfig) *Logger {
	logger := logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// 设置日志格式
	if cfg.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}

	// 设置输出
	switch cfg.Output {
	case "stdout":
		logger.SetOutput(os.Stdout)
	case "file":
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logger.Fatalf("创建日志目录失败: %v", err)
		}

		// 配置 lumberjack 进行日志轮转
		lumberjackLogger := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,    // 兆字节
			MaxBackups: cfg.MaxBackups, // 备份数量
			MaxAge:     cfg.MaxAge,     // 天数
			Compress:   cfg.Compress,   // 压缩旧文件
		}

		// 在调试模式下使用 MultiWriter 同时写入文件和标准输出
		if cfg.Level == "debug" {
			multiWriter := io.MultiWriter(os.Stdout, lumberjackLogger)
			logger.SetOutput(multiWriter)
		} else {
			logger.SetOutput(lumberjackLogger)
		}
	default:
		logger.SetOutput(os.Stdout)
	}

	return &Logger{logger}
}

// WithField 向日志记录器添加字段
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}

// WithFields 向日志记录器添加多个字段
func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// WithError 向日志记录器添加错误字段
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err)
}