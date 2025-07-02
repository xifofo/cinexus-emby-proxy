package routes

import (
	"cinexus/internal/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func GETPlaybackInfo(itemID string, cfg *config.Config) (err error) {
	url := fmt.Sprintf("%s/emby/Items/%s/PlaybackInfo?MaxStreamingBitrate=42000000&api_key=%s",
		cfg.Proxy.URL,
		itemID,
		cfg.Proxy.APIKey,
	)

	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return err
	}
	req.Header.Add("accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	bb, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var resp map[string]any
	err = json.Unmarshal(bb, &resp)
	if err != nil {
		return err
	}

	if len(resp["MediaSources"].([]any)) == 0 {
		return fmt.Errorf("MediaSources not found or empty")
	}

	return fmt.Errorf("请求错误")
}
