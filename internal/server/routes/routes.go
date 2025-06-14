package routes

import (
	"bytes"
	"cinexus/internal/config"
	"cinexus/internal/helper"
	"cinexus/internal/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"resty.dev/v3"
)

// Setup 配置所有应用程序路由
func Setup(e *echo.Echo, cfg *config.Config, log *logger.Logger) {
	goCache := cache.New(time.Duration(cfg.Proxy.CacheTime)*time.Minute, 1*time.Minute)
	embyURL, _ := url.Parse(cfg.Proxy.URL)
	proxy := httputil.NewSingleHostReverseProxy(embyURL)

	e.Any("/*actions", func(c echo.Context) error {
		currentURI := c.Request().RequestURI
		cacheKey := helper.Md5CacheKey(fmt.Sprintf("%s-%s", helper.RemoveQueryParams(currentURI), c.Request().UserAgent()))

		u, err := url.Parse(currentURI)
		removeEmbyRequestPath := strings.Replace(u.Path, "/emby", "", 1) // 替换一次

		if err == nil && removeEmbyRequestPath == "/Sessions/Playing" && cfg.Proxy.AddMetadata {
			return Playing(c, proxy, cfg, log)
		}

		if cacheLink, found := goCache.Get(cacheKey); found {
			return c.Redirect(302, cacheLink.(string))
		}

		url, skip := ProxyPlay(c, proxy, cfg, log)
		if !skip {
			goCache.Set(cacheKey, url, cache.DefaultExpiration)
			return c.Redirect(302, url)
		}

		proxy.ServeHTTP(c.Response().Writer, c.Request())
		return nil
	})

}

type SimpleStartInfo struct {
	ItemId string
}

func Playing(c echo.Context, proxy *httputil.ReverseProxy, cfg *config.Config, log *logger.Logger) error {
	// 创建记录器来存储响应内容
	recorder := httptest.NewRecorder()

	var startInfo SimpleStartInfo

	// 使用 io.Copy 复制请求正文到 recorder
	io.Copy(recorder, c.Request().Body)

	// 将请求正文指针重置到开头
	c.Request().Body = io.NopCloser(bytes.NewReader(recorder.Body.Bytes()))

	if err := json.Unmarshal(recorder.Body.Bytes(), &startInfo); err == nil {
		_, err := GETPlaybackInfo(startInfo.ItemId, cfg)
		if err != nil {
			log.Warnln("补充媒体信息失败了")
		}

		// 使用 goroutine 获取下一集的媒体信息
		go func() {
			GetNextMediaInfo(startInfo.ItemId, cfg, log)
		}()
	}

	// 代理请求
	proxy.ServeHTTP(recorder, c.Request())
	return c.NoContent(204)
}

type SimpleEmbyItemResponse struct {
	Id          string
	SeasonId    string
	ParentId    string
	IndexNumber int
}

type SimpleEmbyItemResponseList struct {
	Items []SimpleEmbyItemResponse
}

func GetNextMediaInfo(itemID string, cfg *config.Config, log *logger.Logger) {
	if !cfg.Proxy.AddNextMediaInfo {
		return
	}

	if cfg.Proxy.AdminUserID == "" {
		log.Error("获取下一集的媒体信息失败，因为 proxy.admin_user_id 未设置")
		return
	}

	// 获取当前播放详情获取 SeasonId
	client := resty.New()
	defer client.Close()

	res, err := client.R().
		SetQueryParams(map[string]string{
			"api_key": cfg.Proxy.APIKey,
		}).
		SetResult(&SimpleEmbyItemResponse{}).
		Get(fmt.Sprintf("%s/emby/Users/%s/Items/%s", cfg.Proxy.URL, cfg.Proxy.AdminUserID, itemID))

	if err != nil {
		log.Errorf("获取下一集的媒体信息失败，因为 %s", err)
		return
	}

	if res.StatusCode() != 200 {
		log.Errorf("获取下一集的媒体信息失败，因为 %s", res.String())
		return
	}

	response := res.Result().(*SimpleEmbyItemResponse)

	// 请求所有集数
	responseList := &SimpleEmbyItemResponseList{}
	res, err = client.R().
		SetQueryParams(map[string]string{
			"api_key":   cfg.Proxy.APIKey,
			"ParentId":  response.SeasonId,
			"Recursive": "true",
			"IsFolder":  "false",
		}).
		SetResult(responseList).
		Get(fmt.Sprintf("%s/emby/Users/%s/Items", cfg.Proxy.URL, cfg.Proxy.AdminUserID))

	if err != nil {
		log.Errorf("获取下一集的媒体信息失败，因为 %s", err)
		return
	}

	if res.StatusCode() != 200 {
		log.Errorf("获取下一集的媒体信息失败，因为 %s", res.String())
		return
	}

	// 根据 IndexNumber 去获取下一集，并补充媒体信息, 可能存在最后一集没有下一集的情况
	if response.IndexNumber >= len(responseList.Items) {
		return
	}

	nextItem := responseList.Items[response.IndexNumber]

	_, err = GETPlaybackInfo(nextItem.Id, cfg)
	if err != nil {
		log.Warnln("补充媒体信息失败了")
	}

	return
}
