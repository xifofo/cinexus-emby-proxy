package emby

import (
	"cinexus/internal/config"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// Client emby 客户端结构体
type Client struct {
	client *resty.Client
	config *config.Config
}

// New 创建新的 emby 客户端
func New(cfg *config.Config) *Client {
	client := resty.New()

	// 设置基础配置
	client.SetBaseURL(cfg.Proxy.URL)
	client.SetHeader("Accept", "application/json")
	client.SetQueryParam("api_key", cfg.Proxy.APIKey)

	return &Client{
		client: client,
		config: cfg,
	}
}

// GetPlaybackInfo 获取播放信息
func (c *Client) GetPlaybackInfo(itemID string) ([]any, error) {
	var response map[string]any

	resp, err := c.client.R().
		SetResult(&response).
		Get(fmt.Sprintf("/emby/Items/%s/PlaybackInfo", itemID))

	if err != nil {
		return nil, fmt.Errorf("请求播放信息失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("获取播放信息失败，状态码: %d, 响应: %s", resp.StatusCode(), resp.String())
	}

	// 检查响应是否为空
	if response == nil {
		return nil, fmt.Errorf("播放信息响应为空")
	}

	// 检查 MediaSources 是否存在
	mediaSourcesRaw, exists := response["MediaSources"]
	if !exists {
		return nil, fmt.Errorf("响应中不包含 MediaSources 字段")
	}

	// 转换为 []any 类型
	mediaSources, ok := mediaSourcesRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("MediaSources 字段格式错误，无法转换为数组")
	}

	// 检查是否有媒体源
	if len(mediaSources) == 0 {
		return nil, fmt.Errorf("MediaSources 为空，itemID: %s", itemID)
	}

	// 记录调试信息
	fmt.Printf("成功获取媒体播放信息: ItemID=%s, MediaSources数量=%d\n", itemID, len(mediaSources))

	return mediaSources, nil
}

// GetItem 获取项目信息
func (c *Client) GetItem(itemID string) (map[string]any, error) {
	var response map[string]any

	resp, err := c.client.R().
		SetResult(&response).
		Get(fmt.Sprintf("/emby/Items/%s", itemID))

	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode())
	}

	return response, nil
}

// GetItems 获取项目列表
func (c *Client) GetItems(parentID string, params map[string]string) (map[string]any, error) {
	var response map[string]any

	req := c.client.R().SetResult(&response)

	// 设置 parentId
	if parentID != "" {
		req.SetQueryParam("ParentId", parentID)
	}

	// 设置其他参数
	for key, value := range params {
		req.SetQueryParam(key, value)
	}

	resp, err := req.Get("/emby/Items")

	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode())
	}

	return response, nil
}

// GetUserViews 获取用户视图
func (c *Client) GetUserViews(userID string) (map[string]any, error) {
	var response map[string]any

	resp, err := c.client.R().
		SetResult(&response).
		Get(fmt.Sprintf("/emby/Users/%s/Views", userID))

	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode())
	}

	return response, nil
}

// PostPlaybackStart 发送播放开始事件
func (c *Client) PostPlaybackStart(data map[string]any) error {
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(data).
		Post("/emby/Sessions/Playing")

	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 204 && resp.StatusCode() != 200 {
		return fmt.Errorf("请求失败，状态码: %d", resp.StatusCode())
	}

	return nil
}

// PostPlaybackProgress 发送播放进度事件
func (c *Client) PostPlaybackProgress(data map[string]any) error {
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(data).
		Post("/emby/Sessions/Playing/Progress")

	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 204 && resp.StatusCode() != 200 {
		return fmt.Errorf("请求失败，状态码: %d", resp.StatusCode())
	}

	return nil
}

// PostPlaybackStop 发送播放停止事件
func (c *Client) PostPlaybackStop(data map[string]any) error {
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(data).
		Post("/emby/Sessions/Playing/Stopped")

	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 204 && resp.StatusCode() != 200 {
		return fmt.Errorf("请求失败，状态码: %d", resp.StatusCode())
	}

	return nil
}

// GetStreamInfo 获取流信息 (通用方法)
func (c *Client) GetStreamInfo(endpoint string, params map[string]string) ([]byte, error) {
	req := c.client.R()

	// 设置参数
	for key, value := range params {
		req.SetQueryParam(key, value)
	}

	resp, err := req.Get(endpoint)

	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode())
	}

	return resp.Body(), nil
}

// CustomRequest 自定义请求方法
func (c *Client) CustomRequest(method, endpoint string, body any, params map[string]string) (*resty.Response, error) {
	req := c.client.R()

	// 设置参数
	for key, value := range params {
		req.SetQueryParam(key, value)
	}

	// 设置请求体
	if body != nil {
		req.SetBody(body)
	}

	var resp *resty.Response
	var err error

	switch method {
	case "GET":
		resp, err = req.Get(endpoint)
	case "POST":
		resp, err = req.Post(endpoint)
	case "PUT":
		resp, err = req.Put(endpoint)
	case "DELETE":
		resp, err = req.Delete(endpoint)
	default:
		return nil, fmt.Errorf("不支持的 HTTP 方法: %s", method)
	}

	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	return resp, nil
}
