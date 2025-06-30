package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"cinexus/internal/config"
	"cinexus/internal/logger"
	"cinexus/internal/storage"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
	sdk115 "github.com/xhofe/115-sdk-go"
	"resty.dev/v3"
)

// Login相关的响应结构体
type LoginResp[T any] struct {
	State   int    `json:"state"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type LoginQrCodeStatusResp struct {
	Msg     string `json:"msg"`
	Status  int    `json:"status"`
	Version string `json:"version"`
}

// generateLoginCodeVerifier 生成符合 OAuth2 PKCE 标准的随机 code verifier
// 长度在 43-128 个字符之间，使用 URL 安全的 base64 编码
func generateLoginCodeVerifier(length int) (string, error) {
	if length < 43 {
		length = 43 // 最小长度43
	}
	if length > 128 {
		length = 128 // 最大长度128
	}

	// 计算需要的字节数 (base64编码后长度约为原始字节数的4/3)
	byteLength := (length * 3) / 4

	// 生成随机字节
	randomBytes := make([]byte, byteLength)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("生成随机字节失败: %v", err)
	}

	// 使用 URL 安全的 base64 编码，并移除填充符
	codeVerifier := base64.RawURLEncoding.EncodeToString(randomBytes)

	// 确保长度符合要求
	if len(codeVerifier) < length {
		// 如果不够长，补充随机字符
		additionalBytes := make([]byte, length-len(codeVerifier))
		rand.Read(additionalBytes)
		codeVerifier += base64.RawURLEncoding.EncodeToString(additionalBytes)
	}

	// 截取到指定长度
	if len(codeVerifier) > length {
		codeVerifier = codeVerifier[:length]
	}

	return codeVerifier, nil
}

// loginCmd 表示 login 主命令
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "登录到各种云盘服务",
	Long: `登录到各种云盘服务。
支持的服务:
  115 - 通过115手机客户端扫码登录`,
}

// login115Cmd 表示 login 115 子命令
var login115Cmd = &cobra.Command{
	Use:   "115",
	Short: "通过115手机客户端扫码登录",
	Long: `通过115手机客户端扫码登录获取tokens。
这个命令会生成二维码，等待手机扫码确认后自动获取并保存tokens。`,
	Run: func(cmd *cobra.Command, args []string) {
		// 加载配置
		cfg := config.Load()

		// 初始化日志
		log := logger.New(cfg.Log)

		// 生成随机 code verifier (43-128位)
		codeVerifier, err := generateLoginCodeVerifier(43)
		if err != nil {
			fmt.Printf("❌ 生成 code verifier 错误: %v\n", err)
			return
		}
		log.Debugf("生成的 code verifier: %s (长度: %d)", codeVerifier, len(codeVerifier))

		// 读取现有tokens（可能为空）
		token115, err := storage.ReadTokens()
		if err != nil {
			log.Debugf("读取现有 115 凭证错误: %v", err)
		}

		sdk115Client := sdk115.New(sdk115.WithRefreshToken(token115.RefreshToken),
			sdk115.WithAccessToken(token115.AccessToken),
			sdk115.WithOnRefreshToken(func(s1, s2 string) {
				storage.UpdateTokens(s2, s1)
			}))

		deviceCode, err := sdk115Client.AuthDeviceCode(context.Background(), cfg.Open115.ClientID, codeVerifier)
		if err != nil {
			fmt.Printf("❌ 获取 115 设备码错误: %v\n", err)
			return
		}

		fmt.Println("📱 请使用115手机客户端扫描以下二维码:")
		fmt.Println()

		qr, err := qrcode.New(deviceCode.QrCode, qrcode.Medium)
		if err != nil {
			fmt.Printf("❌ 生成二维码错误: %v\n", err)
			return
		}
		qr.DisableBorder = true

		// 显示二维码
		fmt.Println(qr.ToSmallString(false))

		// 开始轮询二维码状态
		fmt.Println("⏳ 等待扫码...")
		client := resty.New()
		defer client.Close()

		for {
			resp, err := client.R().
				SetQueryParams(map[string]string{
					"sign": deviceCode.Sign,
					"time": strconv.FormatInt(deviceCode.Time, 10),
					"uid":  deviceCode.UID,
				}).
				Get("https://qrcodeapi.115.com/get/status/")

			if err != nil {
				log.Errorf("请求失败: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			// 手动解析 JSON
			var qrResponse LoginResp[LoginQrCodeStatusResp]
			if err := json.Unmarshal([]byte(resp.String()), &qrResponse); err != nil {
				log.Errorf("JSON 解析失败: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			log.Debugf("轮询状态: %+v", qrResponse.Data)

			switch qrResponse.Data.Status {
			case 1:
				fmt.Println("📲 扫码成功，等待确认...")
			case 2:
				fmt.Println("✅ 确认登录/授权成功！")

				// 获取token
				token, err := sdk115Client.CodeToToken(context.Background(), deviceCode.UID, codeVerifier)
				if err != nil {
					fmt.Printf("❌ 获取 token 错误: %v\n", err)
					return
				}

				// 保存token
				if err := storage.WriteTokens(token.RefreshToken, token.AccessToken); err != nil {
					fmt.Printf("❌ 保存 token 错误: %v\n", err)
					return
				}

				fmt.Println("🎉 登录成功！Token已保存")
				fmt.Printf("   Refresh Token: %s\n", maskToken(token.RefreshToken))
				fmt.Printf("   Access Token: %s\n", maskToken(token.AccessToken))
				return
			case -2:
				fmt.Println("❌ 已取消登录，请重新尝试")
				return
			default:
				fmt.Printf("🔄 未知状态: %d，继续轮询...\n", qrResponse.Data.Status)
			}

			// 避免频繁轮询，稍作延迟
			time.Sleep(1 * time.Second)
		}
	},
}

func init() {
	// 将 login 主命令添加到根命令
	rootCmd.AddCommand(loginCmd)

	// 将 115 子命令添加到 login 命令
	loginCmd.AddCommand(login115Cmd)
}
