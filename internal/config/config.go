package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config 保存应用程序的所有配置
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Proxy     ProxyConfig     `mapstructure:"proxy"`
	Log       LogConfig       `mapstructure:"log"`
	Alist     AlistConfig     `mapstructure:"alist"`
	Driver115 Driver115Config `mapstructure:"driver115"`
}

// ServerConfig 保存服务器配置
type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release
}

// ProxyConfig 保存代理配置
type ProxyConfig struct {
	URL              string `mapstructure:"url"`                 // 代理目标 URL
	APIKey           string `mapstructure:"api_key"`             // API 密钥
	CacheTime        int    `mapstructure:"cache_time"`          // 缓存直链时间，单位：分钟
	CachePickcode    bool   `mapstructure:"cache_pickcode"`      // 缓存 pickcode 到 sqlite 数据库，提高服务速度
	AddMetadata      bool   `mapstructure:"add_metadata"`        // 补充元数据
	Method           string `mapstructure:"method"`              // alist, 115open
	Paths            []Path `mapstructure:"paths"`               // 路径映射
	AdminUserID      string `mapstructure:"admin_user_id"`       // EMBY 管理员用户 ID
	AddNextMediaInfo bool   `mapstructure:"add_next_media_info"` // 播放时提前获取下一集的媒体信息，提高播放速度
}

type Path struct {
	Old  string `mapstructure:"old"`
	New  string `mapstructure:"new"`
	Real string `mapstructure:"real"`
}

type AlistConfig struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
	Sign   bool   `mapstructure:"sign"`
}

// LogConfig 保存日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Format     string `mapstructure:"format"`      // json, text
	Output     string `mapstructure:"output"`      // stdout, file
	FilePath   string `mapstructure:"file_path"`   // 日志文件路径
	MaxSize    int    `mapstructure:"max_size"`    // 最大大小（兆字节）
	MaxBackups int    `mapstructure:"max_backups"` // 要保留的旧日志文件的最大数量
	MaxAge     int    `mapstructure:"max_age"`     // 保留的最大天数
	Compress   bool   `mapstructure:"compress"`    // 是否压缩旧日志文件
}

type Driver115Config struct {
	Cookie string `mapstructure:"cookie"`
}

// Load 从各种来源加载配置
func Load() *Config {
	// 设置默认值
	setDefaults()

	// 读取配置
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("未找到配置文件，使用默认配置")
		} else {
			log.Fatalf("读取配置文件出错: %v", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("无法解码配置: %v", err)
	}

	// 验证配置
	if err := validateConfig(&config); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	return &config
}

// validateConfig 验证配置的有效性
func validateConfig(cfg *Config) error {
	// 验证代理配置
	if cfg.Proxy.URL != "" {
		// 简单的URL格式验证
		if !strings.HasPrefix(cfg.Proxy.URL, "http://") && !strings.HasPrefix(cfg.Proxy.URL, "https://") {
			return fmt.Errorf("代理URL必须以http://或https://开头")
		}

		log.Printf("代理配置已启用: %s", cfg.Proxy.URL)
		if cfg.Proxy.APIKey != "" {
			log.Println("检测到API密钥配置")
		}
	}

	return nil
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 服务器默认值
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")

	// 代理默认值
	viper.SetDefault("proxy.url", "")
	viper.SetDefault("proxy.api_key", "")
	viper.SetDefault("proxy.cache_time", 1)        // 缓存直链时间，单位：小时
	viper.SetDefault("proxy.cache_pickcode", true) // 默认启用pickcode缓存

	// 日志默认值
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "text")
	viper.SetDefault("log.output", "file")
	viper.SetDefault("log.file_path", "logs/app.log")
	viper.SetDefault("log.max_size", 100)  // 100MB
	viper.SetDefault("log.max_backups", 0) // 不限制备份数量
	viper.SetDefault("log.max_age", 7)     // 7天
	viper.SetDefault("log.compress", true) // 压缩旧文件
}
