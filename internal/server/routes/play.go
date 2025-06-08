package routes

import (
	"cinexus/internal/config"
	"cinexus/internal/helper"
	"cinexus/internal/helper/alist"
	"cinexus/internal/logger"
	"cinexus/internal/storage"
	"context"
	"fmt"
	"net/http"
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
	log.Debugf("[EMBY PROXY] ProxyPlay 请求 URI: %s", currentURI)

	re := regexp.MustCompile(`/[Vv]ideos/(\S+)/(stream|original|master)`)
	matches := re.FindStringSubmatch(currentURI)

	if len(matches) < 1 {
		log.Debugf("[EMBY PROXY] ProxyPlay 请求 URI 不匹配: %s", currentURI)
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
	stepStart := time.Now()

	itemInfoUri, itemId, etag, mediaSourceId, apiKey := helper.GetItemPathInfo(c, cfg)
	log.Debugf("【EMBY PROXY】步骤1 - 解析请求参数耗时: %v", time.Since(stepStart))

	stepStart = time.Now()
	embyRes, err := helper.GetEmbyItems(itemInfoUri, itemId, etag, mediaSourceId, apiKey)
	if err != nil {
		log.Errorf("获取 EmbyItems 错误: %v", err)
		return "", true
	}
	log.Debugf("【EMBY PROXY】步骤2 - 获取EmbyItems耗时: %v", time.Since(stepStart))

	// EMBY 的播放地址, 兼容 Windows 的 Emby 路径
	embyPlayPath := helper.EnsureLeadingSlash(embyRes.Path)

	// log.Infof("【EMBY PROXY】Request URI: %s", currentURI)
	log.Infof("【EMBY PROXY】Emby 原地址: %s", embyPlayPath)

	stepStart = time.Now()
	originalHeaders := make(map[string]string)
	for key, value := range c.Request().Header {
		if len(value) > 0 {
			originalHeaders[key] = value[0]
		}
	}

	// 判断 embyPlayPath 是否是 alist url，如果是进行代理
	if strings.HasPrefix(embyPlayPath, cfg.Alist.URL) {
		log.Debugf("【EMBY PROXY】步骤3 - 检测为Alist路径，准备处理耗时: %v", time.Since(stepStart))
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
		log.Debugf("【EMBY PROXY】步骤3 - 路径匹配检查，无需代理耗时: %v", time.Since(stepStart))
		return "", true
	}

	log.Debugf("【EMBY PROXY】步骤3 - 路径匹配检查耗时: %v", time.Since(stepStart))

	// TODO 优先从数据库里获取 pickcode

	if cfg.Proxy.Method == "alist" {
		embyPlayPath = strings.Replace(embyPlayPath, matchPathConfig.Old, matchPathConfig.New, 1)

		return GetAlistRedirectURL(embyPlayPath, log, cfg, originalHeaders)
	}

	if cfg.Proxy.Method == "ck+115open" || cfg.Proxy.Method == "ck" {
		return CKAnd115Open(c, embyPlayPath, log, cfg, originalHeaders, matchPathConfig)
	}

	if cfg.Proxy.Method == "115open" {
		return Get115OpenRedirectURL(c, embyPlayPath, log, cfg, originalHeaders, matchPathConfig)
	}

	log.Warnln("不支持的代理方法")
	return "", true
}

// 通过 Alist 链接直接获取 302 重定向地址
func GetAlistRedirectURL(alistPath string, log *logger.Logger, cfg *config.Config, originalHeaders map[string]string) (string, bool) {
	stepStart := time.Now()

	alistUrl := fmt.Sprintf("%s/d%s", cfg.Alist.URL, alistPath)
	if strings.HasPrefix(alistPath, cfg.Alist.URL) {
		alistUrl = alistPath
	}

	if cfg.Alist.Sign {
		alistUrl = fmt.Sprintf("%s?sign=%s", alistUrl, alist.Sign(alistPath, 0, cfg.Alist.APIKey))
	}
	log.Debugf("【EMBY PROXY】Alist URL构建耗时: %v", time.Since(stepStart))

	stepStart = time.Now()
	redirectURL, err := alist.GetRedirectURL(alistUrl, originalHeaders)
	log.Debugf("【EMBY PROXY】Alist获取重定向URL耗时: %v", time.Since(stepStart))
	if err != nil {
		log.Errorf("获取 Alist 重定向 URL 错误: %v", err)
		return "", true
	}

	return redirectURL, false
}

// 通过 Cookie + 115open API 的方案。配置了 Alist 之后允许降级到 AList 302 方案
func CKAnd115Open(c echo.Context, embyPath string, log *logger.Logger, cfg *config.Config, originalHeaders map[string]string, matchPathConfig config.Path) (string, bool) {
	stepStart := time.Now()

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
	log.Debugf("【EMBY PROXY】步骤4 - 创建115凭证耗时: %v", time.Since(stepStart))

	stepStart = time.Now()
	client := driver115.Defalut().ImportCredential(cr)
	log.Debugf("【EMBY PROXY】步骤5 - 初始化115客户端耗时: %v", time.Since(stepStart))

	// 替换 embyPath 中的 old 为 real 字符串
	embyRealCloudPlayPath := strings.Replace(embyPlayPath, matchPathConfig.Old, matchPathConfig.Real, 1)

	fileName := filepath.Base(embyRealCloudPlayPath)
	dirPath := filepath.Dir(embyRealCloudPlayPath)

	// 优先从数据库里获取 pickcode
	stepStart = time.Now()
	pickcode := ""
	if cfg.Proxy.CachePickcode {
		if cachedPickcode, found := storage.GetPickcodeFromCache(embyRealCloudPlayPath); found {
			pickcode = cachedPickcode
			log.Debugf("【EMBY PROXY】步骤6a - 从缓存获取pickcode成功耗时: %v", time.Since(stepStart))
			log.Infof("【EMBY PROXY】从缓存命中 pickcode: %s -> %s", fileName, pickcode)
		} else {
			log.Debugf("【EMBY PROXY】步骤6a - 缓存中未找到pickcode耗时: %v", time.Since(stepStart))
		}
	}

	// 如果缓存中没有找到，从115API获取
	if pickcode == "" {
		stepStart = time.Now()
		dirRes, err := client.DirName2CID(dirPath)
		if err != nil {
			log.Errorf("获取目录 CID 错误: %v", err)
			return "", true
		}
		log.Debugf("【EMBY PROXY】步骤6b - 获取目录CID耗时: %v", time.Since(stepStart))

		dirID := string(dirRes.CategoryID)

		stepStart = time.Now()
		files, _ := client.ListWithLimit(dirID, 1150)
		log.Debugf("【EMBY PROXY】步骤7 - 列出目录文件耗时: %v", time.Since(stepStart))

		// 如果启用了缓存，异步缓存所有文件的pickcode
		if cfg.Proxy.CachePickcode && files != nil {
			go func() {
				cacheStart := time.Now()
				cachedCount := 0
				skippedCount := 0

				for _, file := range *files {
					// 构建文件的完整路径
					fullFilePath := filepath.Join(dirPath, file.Name)

					// 检查是否已经缓存，如果已存在就跳过
					if _, found := storage.GetPickcodeFromCache(fullFilePath); found {
						skippedCount++
						continue
					}

					// 保存到缓存
					if err := storage.SavePickcodeToCache(fullFilePath, file.PickCode); err != nil {
						log.Warnf("批量缓存失败 %s: %v", file.Name, err)
					} else {
						cachedCount++
					}
				}

				log.Infof("【EMBY PROXY】批量缓存完成 - 新缓存: %d, 跳过: %d, 耗时: %v",
					cachedCount, skippedCount, time.Since(cacheStart))
			}()
		}

		stepStart = time.Now()
		for _, file := range *files {
			if file.Name == fileName {
				pickcode = file.PickCode
				break
			}
		}
		log.Debugf("【EMBY PROXY】步骤8 - 查找文件pickcode耗时: %v", time.Since(stepStart))

		// 如果找到了pickcode且启用了缓存，保存到数据库
		if pickcode != "" && cfg.Proxy.CachePickcode {
			stepStart = time.Now()
			if err := storage.SavePickcodeToCache(embyRealCloudPlayPath, pickcode); err != nil {
				log.Warnf("保存 pickcode 到缓存失败: %v", err)
			} else {
				log.Debugf("【EMBY PROXY】保存pickcode到缓存成功: %s -> %s", fileName, pickcode)
			}
			log.Debugf("【EMBY PROXY】步骤9a - 保存pickcode到缓存耗时: %v", time.Since(stepStart))
		}
	}

	if pickcode == "" {
		log.Printf("找不到文件 %s 降级到 AList 302 方案", fileName)
		return GetAlistRedirectURL(strings.Replace(embyPath, matchPathConfig.Old, matchPathConfig.New, 1), log, cfg, originalHeaders)
	}

	if cfg.Proxy.Method == "ck" {
		stepStart = time.Now()
		downloadInfo, err := client.DownloadWithUA(pickcode, c.Request().UserAgent())
		log.Debugf("【EMBY PROXY】步骤10 - CK方案获取下载地址耗时: %v", time.Since(stepStart))
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
	}

	sdk115Client := sdk115.New(sdk115.WithRefreshToken(token115.RefreshToken),
		sdk115.WithAccessToken(token115.AccessToken),
		sdk115.WithOnRefreshToken(func(s1, s2 string) {
			storage.UpdateTokens(s2, s1)
		}))

	// 使用 OpenApi 去获取下载地址
	stepStart = time.Now()
	downloadUrlResp, err := sdk115Client.DownURL(context.Background(), pickcode, c.Request().UserAgent())
	log.Debugf("【EMBY PROXY】步骤11 - 115Open方案获取下载地址耗时: %v", time.Since(stepStart))
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

// 通过 115open API 的方案
func Get115OpenRedirectURL(c echo.Context, embyPath string, log *logger.Logger, cfg *config.Config, originalHeaders map[string]string, matchPathConfig config.Path) (string, bool) {
	embyPlayPath := embyPath
	// 替换 embyPath 中的 old 为 real 字符串
	embyRealCloudPlayPath := strings.Replace(embyPlayPath, matchPathConfig.Old, matchPathConfig.Real, 1)

	token115, err := storage.ReadTokens()
	if err != nil {
		log.Errorf("读取 115 凭证错误: %v", err)
	}

	sdk115Client := sdk115.New(sdk115.WithRefreshToken(token115.RefreshToken),
		sdk115.WithAccessToken(token115.AccessToken),
		sdk115.WithOnRefreshToken(func(s1, s2 string) {
			storage.UpdateTokens(s2, s1)
		}))

	var resp sdk115.GetFolderInfoResp

	sdk115Client.AuthRequest(context.Background(), sdk115.ApiFsGetFolderInfo, http.MethodPost, &resp, sdk115.ReqWithForm(map[string]string{
		"path": embyRealCloudPlayPath,
	}))

	if resp.PickCode == "" {
		log.Errorf("[Get115OpenRedirectURL] 获取 115 文件 PickCode 失败: %v", err)
		return GetAlistRedirectURL(strings.Replace(embyPath, matchPathConfig.Old, matchPathConfig.New, 1), log, cfg, originalHeaders)
	}

	downloadUrlResp, err := sdk115Client.DownURL(context.Background(), resp.PickCode, c.Request().UserAgent())
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
