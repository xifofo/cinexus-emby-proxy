package routes

import (
	"cinexus/internal/config"
	"cinexus/internal/logger"
	"cinexus/internal/storage"
	"encoding/json"
	"io"
	"time"

	"github.com/labstack/echo/v4"
)

// EmbyWebhookRequest 定义 Emby webhook 请求的数据结构
type EmbyWebhookRequest struct {
	Title       string     `json:"Title"`
	Description string     `json:"Description,omitempty"`
	Date        time.Time  `json:"Date"`
	Event       string     `json:"Event"`
	Severity    string     `json:"Severity"`
	Item        EmbyItem   `json:"Item"`
	Server      EmbyServer `json:"Server"`
}

// EmbyItem 定义 Emby 媒体项目的数据结构
type EmbyItem struct {
	Name                    string                 `json:"Name"`
	ServerId                string                 `json:"ServerId"`
	Id                      string                 `json:"Id"`
	DateCreated             time.Time              `json:"DateCreated"`
	SortName                string                 `json:"SortName"`
	PremiereDate            *time.Time             `json:"PremiereDate,omitempty"`
	ExternalUrls            []ExternalURL          `json:"ExternalUrls"`
	Path                    string                 `json:"Path"`
	Overview                string                 `json:"Overview,omitempty"`
	Taglines                []string               `json:"Taglines"`
	Genres                  []string               `json:"Genres"`
	FileName                string                 `json:"FileName"`
	ProductionYear          int                    `json:"ProductionYear,omitempty"`
	IndexNumber             *int                   `json:"IndexNumber,omitempty"`
	ParentIndexNumber       *int                   `json:"ParentIndexNumber,omitempty"`
	RemoteTrailers          []interface{}          `json:"RemoteTrailers"`
	ProviderIds             map[string]interface{} `json:"ProviderIds"`
	IsFolder                bool                   `json:"IsFolder"`
	ParentId                string                 `json:"ParentId,omitempty"`
	Type                    string                 `json:"Type"`
	Studios                 []interface{}          `json:"Studios"`
	GenreItems              []interface{}          `json:"GenreItems"`
	TagItems                []interface{}          `json:"TagItems"`
	ParentLogoItemId        string                 `json:"ParentLogoItemId,omitempty"`
	ParentBackdropItemId    string                 `json:"ParentBackdropItemId,omitempty"`
	ParentBackdropImageTags []string               `json:"ParentBackdropImageTags,omitempty"`
	SeriesName              string                 `json:"SeriesName,omitempty"`
	SeriesId                string                 `json:"SeriesId,omitempty"`
	SeasonId                string                 `json:"SeasonId,omitempty"`
	PrimaryImageAspectRatio float64                `json:"PrimaryImageAspectRatio,omitempty"`
	SeriesPrimaryImageTag   string                 `json:"SeriesPrimaryImageTag,omitempty"`
	SeasonName              string                 `json:"SeasonName,omitempty"`
	ImageTags               map[string]string      `json:"ImageTags,omitempty"`
	BackdropImageTags       []string               `json:"BackdropImageTags"`
	ParentLogoImageTag      string                 `json:"ParentLogoImageTag,omitempty"`
	ParentThumbItemId       string                 `json:"ParentThumbItemId,omitempty"`
	ParentThumbImageTag     string                 `json:"ParentThumbImageTag,omitempty"`
	MediaType               string                 `json:"MediaType"`
}

// ExternalURL 定义外部链接的数据结构
type ExternalURL struct {
	Name string `json:"Name"`
	Url  string `json:"Url"`
}

// EmbyServer 定义 Emby 服务器的数据结构
type EmbyServer struct {
	Name    string `json:"Name"`
	Id      string `json:"Id"`
	Version string `json:"Version"`
}

// HandleEmbyWebhook 处理 Emby webhook 请求
func HandleEmbyWebhook(c echo.Context, cfg *config.Config, log *logger.Logger) error {
	// 读取请求体
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Errorf("读取 webhook 请求体失败: %v", err)
		return c.JSON(400, map[string]string{"error": "读取请求体失败"})
	}

	// 解析 webhook 数据
	var webhookData EmbyWebhookRequest
	if err := json.Unmarshal(body, &webhookData); err != nil {
		log.Errorf("Emby webhook JSON 解析失败: %v", err)
		return c.JSON(400, map[string]string{"error": "JSON 解析失败"})
	}

	// 处理不同类型的事件
	switch webhookData.Event {
	case "library.new":
		handleLibraryNew(webhookData, cfg, log)
	default:
		log.Infof("收到事件类型: %s，暂不处理", webhookData.Event)
	}

	return c.JSON(200, map[string]string{
		"message": "ok",
		"event":   webhookData.Event,
		"status":  "已处理",
	})
}

// handleLibraryNew 处理新增媒体事件
func handleLibraryNew(data EmbyWebhookRequest, cfg *config.Config, log *logger.Logger) {
	// 判断是否处理该事件
	if !cfg.Server.ProcessNewMedia {
		log.Infof("新增媒体事件处理已禁用，跳过处理: %s", data.Item.Name)
		return
	}

	// 获取持久化任务队列并添加任务
	taskQueue := storage.GetTaskQueue()
	if taskQueue == nil {
		log.Error("任务队列未初始化，无法添加任务")
		return
	}

	// 添加任务到持久化队列
	if err := taskQueue.AddTask(data.Item.Id); err != nil {
		log.Errorf("添加媒体处理任务失败: %v", err)
	} else {
		log.Infof("媒体处理任务已添加到队列: ItemID=%s", data.Item.Id)
	}
}
