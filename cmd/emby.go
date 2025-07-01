package cmd

import (
	"fmt"
	"os"
	"time"

	"cinexus/internal/config"
	"cinexus/internal/logger"
	"cinexus/internal/server/routes"
	"cinexus/internal/storage"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"resty.dev/v3"
)

// embyCmd 表示 emby 命令
var embyCmd = &cobra.Command{
	Use:   "emby",
	Short: "管理 Emby 媒体信息",
	Long: `管理 Emby 媒体信息的命令。
可以用来批量完善媒体播放信息和元数据等。`,
}

// refreshMediaCmd 表示批量完善媒体信息的子命令
var refreshMediaCmd = &cobra.Command{
	Use:   "refresh-media [folder-id]",
	Short: "批量完善文件夹中的媒体信息",
	Long: `通过Emby文件夹ID，获取文件夹中的所有媒体项目，
并批量完善其播放信息和元数据。`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		folderID := args[0]

		// 读取配置
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 加载配置失败: %v\n", err)
			os.Exit(1)
		}

		// 初始化日志
		log, err := initLogger(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 初始化日志失败: %v\n", err)
			os.Exit(1)
		}

		// 初始化数据库
		if err := storage.InitDB(); err != nil {
			fmt.Fprintf(os.Stderr, "错误: 初始化数据库失败: %v\n", err)
			os.Exit(1)
		}

		// 创建任务队列
		playbackCallback := func(itemID string, cfg *config.Config) error {
			_, err := routes.GETPlaybackInfo(itemID, cfg)
			return err
		}
		taskQueue := storage.NewPersistentTaskQueue(cfg, log, playbackCallback)

		if err := batchRefreshMedia(folderID, cfg, taskQueue); err != nil {
			fmt.Fprintf(os.Stderr, "错误: 批量完善媒体信息失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ 批量完善媒体信息完成!")
	},
}

// EmbyFolderItem 定义 Emby 文件夹项目的数据结构
type EmbyFolderItem struct {
	Name     string `json:"Name"`
	ServerId string `json:"ServerId"`
	Id       string `json:"Id"`
	IsFolder bool   `json:"IsFolder"`
	Type     string `json:"Type"`
}

// EmbyFolderResponse 定义 Emby 文件夹响应的数据结构
type EmbyFolderResponse struct {
	Items            []EmbyFolderItem `json:"Items"`
	TotalRecordCount int              `json:"TotalRecordCount"`
}

// batchRefreshMedia 批量完善媒体信息
func batchRefreshMedia(folderID string, cfg *config.Config, taskQueue *storage.PersistentTaskQueue) error {
	fmt.Printf("🔍 正在获取文件夹 %s 的详情...\n", folderID)

	// 获取文件夹中的所有项目
	items, err := getFolderItems(folderID, cfg)
	if err != nil {
		return fmt.Errorf("获取文件夹详情失败: %w", err)
	}

	fmt.Printf("📁 找到 %d 个项目\n", len(items))

	successCount := 0
	errorCount := 0

	// 遍历所有项目并添加任务
	for _, item := range items {
		// 跳过文件夹类型的项目，只处理媒体文件
		if item.IsFolder {
			fmt.Printf("⏭️  跳过文件夹: %s (ID: %s)\n", item.Name, item.Id)
			continue
		}

		fmt.Printf("🔄 正在完善媒体信息: %s (ID: %s)\n", item.Name, item.Id)

		if err := taskQueue.AddTask(item.Id); err != nil {
			fmt.Printf("❌ 完善媒体信息失败: %s - %v\n", item.Name, err)
			errorCount++
		} else {
			fmt.Printf("✅ 媒体信息已加入完善队列: %s\n", item.Name)
			successCount++
		}

		// 添加小延迟，避免请求过于频繁
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("\n📊 批量完善媒体信息结果:\n")
	fmt.Printf("   成功: %d 个媒体项目\n", successCount)
	fmt.Printf("   失败: %d 个媒体项目\n", errorCount)
	fmt.Printf("   总计: %d 个项目\n", len(items))

	return nil
}

// getFolderItems 获取文件夹中的所有项目
func getFolderItems(folderID string, cfg *config.Config) ([]EmbyFolderItem, error) {
	if cfg.Proxy.AdminUserID == "" {
		return nil, fmt.Errorf("proxy.admin_user_id 未配置，无法获取文件夹详情")
	}

	client := resty.New()
	defer client.Close()

	var folderResp EmbyFolderResponse
	res, err := client.R().
		SetQueryParams(map[string]string{
			"api_key":   cfg.Proxy.APIKey,
			"ParentId":  folderID,
			"Recursive": "true",
		}).
		SetResult(&folderResp).
		Get(fmt.Sprintf("%s/emby/Users/%s/Items", cfg.Proxy.URL, cfg.Proxy.AdminUserID))

	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("请求失败，状态码: %d - %s", res.StatusCode(), res.String())
	}

	return folderResp.Items, nil
}

// loadConfig 加载配置
func loadConfig() (*config.Config, error) {
	var cfg config.Config

	// 初始化 Viper 配置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return &cfg, nil
}

// initLogger 初始化日志
func initLogger(cfg *config.Config) (*logger.Logger, error) {
	return logger.New(cfg.Log), nil
}

func init() {
	rootCmd.AddCommand(embyCmd)
	embyCmd.AddCommand(refreshMediaCmd)
}
