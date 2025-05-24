package routes

import (
	"cinexus/internal/config"
	"cinexus/internal/helper"
	"cinexus/internal/helper/alist"
	"cinexus/internal/logger"
	"fmt"
	"net/http/httputil"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
)

func IsPlayURI(uri string) bool {
	re := regexp.MustCompile(`/[Vv]ideos/(\S+)/(stream|original)`)
	matches := re.FindStringSubmatch(uri)

	return len(matches) > 0
}

func ProxyPlay(c echo.Context, proxy *httputil.ReverseProxy, cfg *config.Config, log *logger.Logger) (string, bool) {
	currentURI := c.Request().RequestURI
	// 暂时移除 master 的匹配
	re := regexp.MustCompile(`/[Vv]ideos/(\S+)/(stream|original)`)
	matches := re.FindStringSubmatch(currentURI)

	if len(matches) < 1 {
		return "", true
	}

	itemInfoUri, itemId, etag, mediaSourceId, apiKey := helper.GetItemPathInfo(c, cfg)
	embyRes, err := helper.GetEmbyItems(itemInfoUri, itemId, etag, mediaSourceId, apiKey)

	if err != nil {
		log.Errorf("获取 EmbyItems 错误: %v", err)
		return "", true
	}

	// EMBY 的播放地址
	embyPlayPath := embyRes.Path

	// log.Infof("【EMBY PROXY】Request URI: %s", currentURI)
	log.Infof("【EMBY PROXY】Emby 原地址: %s", embyPlayPath)

	originalHeaders := make(map[string]string)
	for key, value := range c.Request().Header {
		if len(value) > 0 {
			originalHeaders[key] = value[0]
		}
	}

	// 匹配 embyPlayPath 是否在 cfg.Proxy.Paths 中，如果存在，则替换为 cfg.Proxy.Paths 中的 new
	// 不存在 old 开头的说明不需要代理
	needProxy := false
	for _, path := range cfg.Proxy.Paths {
		if strings.HasPrefix(embyPlayPath, path.Old) {
			embyPlayPath = strings.Replace(embyPlayPath, path.Old, path.New, 1)
			needProxy = true
			break
		}
	}

	if !needProxy {
		return "", true
	}

	// userAgent := strings.ToLower(c.Request().Header.Get("User-Agent"))
	if cfg.Proxy.Method == "alist" {
		// TODO infuse 不知道还是不是和以前一样

		alistUrl := fmt.Sprintf("%s/d%s", cfg.Alist.URL, embyPlayPath)
		if cfg.Alist.Sign {
			alistUrl = fmt.Sprintf("%s?sign=%s", alistUrl, alist.Sign(embyPlayPath, 0, cfg.Alist.APIKey))
		}

		log.Infof("【ALIST PROXY】Request URI: %s", alistUrl)
		redirectURL, err := alist.GetRedirectURL(alistUrl, originalHeaders)
		if err != nil {
			log.Errorf("获取 Alist 重定向 URL 错误: %v", err)
			return "", true
		}

		return redirectURL, false
	}

	log.Warnln("不支持的代理方法")
	return "", true
}
