package routes

import (
	"cinexus/internal/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func GETPlaybackInfo(itemID string, cfg *config.Config) (response map[string]any, err error) {
	url := fmt.Sprintf("%s/emby/Items/%s/PlaybackInfo?MaxStreamingBitrate=42000000&api_key=%s",
		cfg.Proxy.URL,
		itemID,
		cfg.Proxy.APIKey,
	)

	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return response, err
	}
	req.Header.Add("accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return response, err
	}
	defer res.Body.Close()

	bb, err := io.ReadAll(res.Body)
	if err != nil {
		return response, err
	}

	var resp map[string]any
	err = json.Unmarshal(bb, &resp)
	if err != nil {
		return response, err
	}

	if len(resp["MediaSources"].([]any)) == 0 {
		return response, fmt.Errorf("MediaSources not found or empty")
	}

	for _, item := range resp["MediaSources"].([]any) {
		if item.(map[string]any)["ItemId"] == itemID {
			return item.(map[string]any), nil
		}
	}

	return response, fmt.Errorf("请求错误")
}
