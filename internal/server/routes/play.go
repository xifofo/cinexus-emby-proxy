package routes

import (
	"cinexus/internal/config"
	"cinexus/internal/helper"
	"cinexus/internal/helper/alist"
	"cinexus/internal/logger"
	"cinexus/internal/storage"
	"context"
	"fmt"
	"net/http/httputil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	driver115 "github.com/SheltonZhu/115driver/pkg/driver"
	"github.com/labstack/echo/v4"
	sdk115 "github.com/xhofe/115-sdk-go"
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

	// 开始计时
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		log.Infof("【EMBY PROXY】ProxyPlay 执行时间: %v", duration)
	}()

	return proxyPlayInternal(c, proxy, cfg, log)
}

func proxyPlayInternal(c echo.Context, proxy *httputil.ReverseProxy, cfg *config.Config, log *logger.Logger) (string, bool) {
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

	// 判断 embyPlayPath 是否是 alist url，如果是进行代理
	if strings.HasPrefix(embyPlayPath, cfg.Alist.URL) {
		return GetAlistRedirectURL(embyPlayPath, log, cfg, originalHeaders)
	}

	// 匹配 embyPlayPath 是否在 cfg.Proxy.Paths 中，如果存在，则替换为 cfg.Proxy.Paths 中的 new
	// 不存在 old 开头的说明不需要代理
	needProxy := false
	matchPathConfig := config.Path{}
	for _, path := range cfg.Proxy.Paths {
		if strings.HasPrefix(embyPlayPath, path.Old) {
			matchPathConfig = path
			needProxy = true
			break
		}
	}

	if !needProxy {
		return "", true
	}

	// TODO 优先从数据库里获取 pickcode

	if cfg.Proxy.Method == "alist" {
		embyPlayPath = strings.Replace(embyPlayPath, matchPathConfig.Old, matchPathConfig.New, 1)

		return GetAlistRedirectURL(embyPlayPath, log, cfg, originalHeaders)
	}

	if cfg.Proxy.Method == "ck+115open" || cfg.Proxy.Method == "ck" {
		return CKAnd115Open(c, embyPlayPath, log, cfg, originalHeaders, matchPathConfig)
	}

	log.Warnln("不支持的代理方法")
	return "", true
}

// 通过 Alist 链接直接获取 302 重定向地址
func GetAlistRedirectURL(alistPath string, log *logger.Logger, cfg *config.Config, originalHeaders map[string]string) (string, bool) {
	alistUrl := fmt.Sprintf("%s/d%s", cfg.Alist.URL, alistPath)
	if strings.HasPrefix(alistPath, cfg.Alist.URL) {
		alistUrl = alistPath
	}

	if cfg.Alist.Sign {
		alistUrl = fmt.Sprintf("%s?sign=%s", alistUrl, alist.Sign(alistPath, 0, cfg.Alist.APIKey))
	}

	redirectURL, err := alist.GetRedirectURL(alistUrl, originalHeaders)
	if err != nil {
		log.Errorf("获取 Alist 重定向 URL 错误: %v", err)
		return "", true
	}

	return redirectURL, false
}

// 通过 Cookie + 115open API 的方案。配置了 Alist 之后允许降级到 AList 302 方案
func CKAnd115Open(c echo.Context, embyPath string, log *logger.Logger, cfg *config.Config, originalHeaders map[string]string, matchPathConfig config.Path) (string, bool) {
	cr := &driver115.Credential{}
	embyPlayPath := embyPath

	err := cr.FromCookie(cfg.Driver115.Cookie)
	if err != nil {
		log.Errorf("从 Cookie 获取 115 凭证错误: %v", err)
		// TODO 发起通知
		// 降级到 AList 302 方案
		embyPlayPath = strings.Replace(embyPath, matchPathConfig.Old, matchPathConfig.New, 1)

		return GetAlistRedirectURL(embyPlayPath, log, cfg, originalHeaders)
	}

	client := driver115.Defalut().ImportCredential(cr)

	// 替换 embyPath 中的 old 为 real 字符串
	embyRealCloudPlayPath := strings.Replace(embyPlayPath, matchPathConfig.Old, matchPathConfig.Real, 1)

	fileName := filepath.Base(embyRealCloudPlayPath)
	dirPath := filepath.Dir(embyRealCloudPlayPath)

	dirRes, err := client.DirName2CID(dirPath)
	if err != nil {
		log.Errorf("获取目录 CID 错误: %v", err)
		return "", true
	}

	dirID := string(dirRes.CategoryID)

	files, _ := client.ListWithLimit(dirID, 1150)

	pickcode := ""
	for _, file := range *files {
		if file.Name == fileName {
			pickcode = file.PickCode
			break
		}
	}

	if pickcode == "" {
		log.Printf("找不到文件 %s 降级到 AList 302 方案", fileName)
		return GetAlistRedirectURL(strings.Replace(embyPath, matchPathConfig.Old, matchPathConfig.New, 1), log, cfg, originalHeaders)
	}

	if cfg.Proxy.Method == "ck" {
		downloadInfo, err := client.DownloadWithUA(pickcode, c.Request().UserAgent())
		if err == nil {
			log.Infof("CK 方案成功，使用 CDN 地址：%s", downloadInfo.Url.Url)
			return downloadInfo.Url.Url, false
		}

		log.Printf("CK 方案失败，获取 CDN 地址失败：%e", err)
		log.Infof("CK 方案失败，降级到 115Open 方案")
	}

	token115, err := storage.ReadTokens()
	if err != nil {
		log.Errorf("读取 115 凭证错误: %v", err)
		return "", true
	}

	// 使用 OpenApi 去获取下载地址
	sdkClient := sdk115.New(sdk115.WithRefreshToken(token115.RefreshToken),
		sdk115.WithAccessToken(token115.AccessToken),
		sdk115.WithOnRefreshToken(func(s1, s2 string) {
			storage.UpdateTokens(s2, s1)
		}))

	downloadUrlResp, err := sdkClient.DownURL(context.Background(), pickcode, c.Request().UserAgent())
	if err != nil {
		log.Errorf("115Open 方案失败，降级到 AList 302 方案，获取下载地址失败: %v", err)
		return GetAlistRedirectURL(strings.Replace(embyPath, matchPathConfig.Old, matchPathConfig.New, 1), log, cfg, originalHeaders)
	}

	var firstKey string
	for key := range downloadUrlResp {
		firstKey = key
		break
	}

	u, ok := downloadUrlResp[firstKey]
	if !ok {
		log.Infof("115Open 方案失败，降级到 AList 302 方案")
		return GetAlistRedirectURL(strings.Replace(embyPath, matchPathConfig.Old, matchPathConfig.New, 1), log, cfg, originalHeaders)
	}

	log.Infof("115Open 方案成功，使用 CDN 地址：%s", u.URL.URL)
	return u.URL.URL, false
}
