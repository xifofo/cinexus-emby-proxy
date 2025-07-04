package routes

import (
	"cinexus/internal/config"
	"cinexus/internal/helper/emby"
	"fmt"
)

// GETPlaybackInfo 获取播放信息，使用新的emby客户端方法
func GETPlaybackInfo(itemID string, cfg *config.Config) error {
	// 创建emby客户端
	embyClient := emby.New(cfg)

	// 使用新的GetPlaybackInfo方法获取媒体源信息
	mediaSources, err := embyClient.GetPlaybackInfo(itemID)
	if err != nil {
		return fmt.Errorf("获取播放信息失败: %w", err)
	}

	// 检查是否有媒体源
	if len(mediaSources) == 0 {
		return fmt.Errorf("MediaSources not found or empty")
	}

	// 记录成功获取的信息
	fmt.Printf("媒体播放信息获取成功: ItemID=%s, MediaSources数量=%d\n", itemID, len(mediaSources))

	return nil
}
