package helper

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
)

// 正则表达式用于匹配 Windows 盘符格式
var driveLetterPattern = regexp.MustCompile(`^[a-zA-Z]:[\\/]+`)

// ReplaceIgnoreCase 不区分大小写替换字符串
func ReplaceIgnoreCase(input string, oldStr string, newStr string) string {
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(oldStr))
	return re.ReplaceAllString(input, newStr)
}

// IsURL 正则匹配是不是链接
func IsURL(str string) bool {
	// 使用 regexp 包匹配 URL 模式
	// 匹配协议、主机名、路径和可选的端口号
	// 实际应用中可能需要根据需求修改正则表达式
	re := regexp.MustCompile(`^(http|https):\/\/[a-zA-Z0-9]+\.[a-zA-Z0-9]+(:[0-9]+)?\/?.*$`)

	return re.MatchString(str)
}

func EnsureLeadingSlash(path string) string {
	path = ConvertToLinuxPath(path)

	if !strings.HasPrefix(path, "/") {
		path = "/" + path // 不是以 / 开头，加上 /
	}

	return path
}

func RemoveDriveLetter(path string) string {
	// 检查输入是否为空字符串
	if path == "" {
		return ""
	}

	// 使用预编译的正则表达式移除盘符
	return driveLetterPattern.ReplaceAllString(path, "")
}

func ConvertToLinuxPath(windowsPath string) string {
	// 将所有的反斜杠转换成正斜杠
	linuxPath := strings.ReplaceAll(RemoveDriveLetter(windowsPath), "\\", "/")
	return linuxPath
}

func ConvertToWindowsPath(path string) string {
	return strings.ReplaceAll(path, "/", "\\")
}

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
