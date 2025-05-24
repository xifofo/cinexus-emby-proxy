package helper

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
)

// RemoveQueryParams 移除请求链接上的参数
func RemoveQueryParams(originalURL string) string {
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}
	parsedURL.RawQuery = ""
	return parsedURL.String()
}

func Md5CacheKey(data string) string {
	// 创建一个 MD5 哈希实例
	hash := md5.New()

	// 写入数据
	hash.Write([]byte(data))

	// 获取哈希结果
	hashBytes := hash.Sum(nil)

	// 将结果转换为十六进制字符串
	hashString := hex.EncodeToString(hashBytes)

	return hashString
}
