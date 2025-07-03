package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config 保存应用程序的所有配置
type Config struct {
	Server      ServerConfig       `mapstructure:"server"`
	Proxy       ProxyConfig        `mapstructure:"proxy"`
	Log         LogConfig          `mapstructure:"log"`
	Alist       AlistConfig        `mapstructure:"alist"`
	Driver115   Driver115Config    `mapstructure:"driver115"`
	Open115     Open115Config      `mapstructure:"open115"`
	FileWatcher FileWatcherConfigs `mapstructure:"file_watcher"`
}

// ServerConfig 保存服务器配置
type ServerConfig struct {
	Port            string `mapstructure:"port"`
	Mode            string `mapstructure:"mode"`              // debug, release
	ProcessNewMedia bool   `mapstructure:"process_new_media"` // 是否处理新增媒体事件
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

type Open115Config struct {
	ClientID string `mapstructure:"client_id"`
}

// FileWatcherConfigs 保存文件监控配置
type FileWatcherConfigs struct {
	Enabled bool                `mapstructure:"enabled"` // 是否启用文件监控功能
	Configs []FileWatcherConfig `mapstructure:"configs"` // 多个监控配置
}

// FileWatcherConfig 保存单个文件监控配置
type FileWatcherConfig struct {
	Name                 string   `mapstructure:"name"`                   // 监控配置名称
	SourceDir            string   `mapstructure:"source_dir"`             // 监控的源目录
	TargetDir            string   `mapstructure:"target_dir"`             // 目标复制目录
	Extensions           []string `mapstructure:"extensions"`             // 监控的文件扩展名，空表示所有文件
	Recursive            bool     `mapstructure:"recursive"`              // 是否递归监控子目录
	CopyMode             string   `mapstructure:"copy_mode"`              // 复制模式: copy(复制), move(移动), link(硬链接)
	CreateDirs           bool     `mapstructure:"create_dirs"`            // 是否自动创建目标目录
	ProcessExistingFiles bool     `mapstructure:"process_existing_files"` // 是否在启动时处理已存在的文件
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

	// 验证文件监控配置
	if cfg.FileWatcher.Enabled {
		if len(cfg.FileWatcher.Configs) == 0 {
			return fmt.Errorf("启用文件监控时，至少需要配置一个监控项")
		}

		for i, watcherCfg := range cfg.FileWatcher.Configs {
			if watcherCfg.SourceDir == "" {
				return fmt.Errorf("第%d个监控配置的source_dir不能为空", i+1)
			}
			if watcherCfg.TargetDir == "" {
				return fmt.Errorf("第%d个监控配置的target_dir不能为空", i+1)
			}
			if watcherCfg.CopyMode != "" {
				if watcherCfg.CopyMode != "copy" && watcherCfg.CopyMode != "move" && watcherCfg.CopyMode != "link" {
					return fmt.Errorf("第%d个监控配置的copy_mode必须是 copy, move 或 link 之一", i+1)
				}
			}
			log.Printf("文件监控配置[%s]已启用: %s -> %s", watcherCfg.Name, watcherCfg.SourceDir, watcherCfg.TargetDir)
		}
	}

	return nil
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 服务器默认值
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.process_new_media", false) // 默认不处理新增媒体事件

	// 代理默认值
	viper.SetDefault("proxy.url", "")
	viper.SetDefault("proxy.api_key", "")
	viper.SetDefault("proxy.cache_time", 1)        // 缓存直链时间，单位：小时
	viper.SetDefault("proxy.cache_pickcode", true) // 默认启用pickcode缓存

	// 文件监控默认值
	viper.SetDefault("file_watcher.enabled", false)
	viper.SetDefault("file_watcher.configs", []map[string]interface{}{})

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
