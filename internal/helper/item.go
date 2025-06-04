package helper

import (
	"cinexus/internal/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
)

func GetItemPathInfo(c echo.Context, cfg *config.Config) (itemInfoUri string, itemId string, etag string, mediaSourceId string, apiKey string) {
	embyHost := cfg.Proxy.URL
	regex := regexp.MustCompile("[A-Za-z0-9]+")

	// 从URI中解析itemId，移除"emby"和"Sync"，以及所有连字符"-"。
	pathParts := regex.FindAllString(strings.ReplaceAll(strings.ReplaceAll(c.Request().RequestURI, "emby", ""), "Sync", ""), -1)
	if len(pathParts) > 1 {
		itemId = pathParts[1]
	}

	values := c.Request().URL.Query()
	c.Logger().Infof(c.Request().URL.RawQuery)
	if values.Get("MediaSourceId") != "" {
		mediaSourceId = values.Get("MediaSourceId")
	}

	etag = values.Get("Tag")
	apiKey = values.Get("X-Emby-Token")
	if apiKey == "" {
		apiKey = values.Get("api_key")
	}

	if apiKey == "" {
		apiKey = cfg.Proxy.APIKey
	}

	// Construct the itemInfoUri based on the URI and parameters
	if strings.Contains(c.Request().RequestURI, "JobItems") {
		itemInfoUri = embyHost + "/Sync/JobItems?api_key=" + apiKey
	} else {
		if mediaSourceId != "" {
			newMediaSourceId := mediaSourceId
			if strings.HasPrefix(mediaSourceId, "mediasource_") {
				newMediaSourceId = strings.Replace(mediaSourceId, "mediasource_", "", 1)
			}

			itemId = newMediaSourceId
			itemInfoUri = embyHost + "/Items?Ids=" + newMediaSourceId + "&Fields=Path,MediaSources&Limit=1&api_key=" + apiKey
		} else {
			itemInfoUri = embyHost + "/Items?Ids=" + itemId + "&Fields=Path,MediaSources&Limit=1&api_key=" + apiKey
		}
	}

	return itemInfoUri, itemId, etag, mediaSourceId, apiKey
}

type GetEmbyItemsResult struct {
	ID                  string
	Protocol            string
	Path                string
	NeedAddMediaStreams bool
}

func GetEmbyItems(itemInfoUri string, itemId string, etag string, mediaSourceId string, apiKey string) (GetEmbyItemsResult, error) {
	rvt := GetEmbyItemsResult{
		ID:                  "",
		Protocol:            "File",
		Path:                "",
		NeedAddMediaStreams: false,
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", itemInfoUri, nil)
	if err != nil {
		return rvt, fmt.Errorf("初始化代理请求 %s 错误 %v", itemInfoUri, err)
	}

	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		return rvt, fmt.Errorf("代理请求 mediaItemInfo 错误, %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return rvt, fmt.Errorf("请求 EMBY Api %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	var result map[string]any

	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return rvt, fmt.Errorf("error: emby_api response json unmarshal failed, %v", err)
	}

	items, ok := result["Items"].([]interface{})
	if !ok {
		return rvt, fmt.Errorf("error: emby_api invalid items format")
	}

	// 暂时不知道这个 JobItems 是什么
	if itemInfoUri[len(itemInfoUri)-9:] == "JobItems" {
		for _, item := range items {
			jobItem := item.(map[string]any)
			if jobItem["Id"] == itemId && jobItem["MediaSource"] != nil {
				mediaSource := jobItem["MediaSource"].(map[string]any)
				rvt.Protocol = mediaSource["Protocol"].(string)
				rvt.Path = mediaSource["Path"].(string)
				return rvt, nil
			}
		}

		return rvt, fmt.Errorf("错误：emby_api /Sync/JobItems response is null")
	}

	if len(items) == 0 {
		return rvt, fmt.Errorf("错误：emby_api /Items response is null")
	}

	item := items[0].(map[string]any)
	rvt.Path = item["Path"].(string)

	// Parse MediaSources if available
	mediaSources, exists := item["MediaSources"].([]interface{})

	if exists && len(mediaSources) > 0 {
		var mediaSource map[string]any
		for _, source := range mediaSources {
			ms := source.(map[string]any)

			// ETag only on Jellyfin
			if etag != "" && ms["etag"].(string) == etag {
				mediaSource = ms
				break
			}

			if mediaSourceId != "" && ms["Id"].(string) == mediaSourceId {
				mediaSource = ms
				break
			}
		}

		if mediaSource == nil {
			mediaSource = mediaSources[0].(map[string]any)
		}

		rvt.Protocol = mediaSource["Protocol"].(string)
		rvt.Path = mediaSource["Path"].(string)

		mediaStreams, exists := mediaSource["MediaStreams"]
		if !exists || len(mediaStreams.([]any)) == 0 {
			rvt.NeedAddMediaStreams = true
		}

		_, exists = mediaSource["Bitrate"]
		if !exists {
			rvt.NeedAddMediaStreams = true
		}
	}

	return rvt, nil
}
