package cmd

import (
	"fmt"
	"os"
	"time"

	"cinexus/internal/storage"

	"github.com/spf13/cobra"
)

// tokenCmd 表示 token 命令
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "管理 115 tokens",
	Long: `管理 115 tokens 的命令。
可以用来设置、更新或查看当前的 refresh_token 和 access_token。

锁行为选项:
  --lock-timeout: 设置获取文件锁的超时时间 (默认: 30s)
  --no-wait: 不等待锁，如果锁被占用立即返回错误`,
}

// setTokenCmd 表示 set 子命令
var setTokenCmd = &cobra.Command{
	Use:   "set",
	Short: "设置 115 tokens",
	Long: `设置 115 tokens 的 refresh_token 和 access_token。
可以同时设置两个 token，也可以只设置其中一个。`,
	Run: func(cmd *cobra.Command, args []string) {
		refreshToken, _ := cmd.Flags().GetString("refresh-token")
		accessToken, _ := cmd.Flags().GetString("access-token")

		if refreshToken == "" && accessToken == "" {
			fmt.Fprintf(os.Stderr, "错误: 必须提供至少一个 token (--refresh-token 或 --access-token)\n")
			os.Exit(1)
		}

		// 设置锁行为
		if err := configureLockBehavior(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}

		if err := storage.UpdateTokens(refreshToken, accessToken); err != nil {
			fmt.Fprintf(os.Stderr, "错误: 更新 tokens 失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Tokens 更新成功!")
		if refreshToken != "" {
			fmt.Printf("   Refresh Token: %s\n", maskToken(refreshToken))
		}
		if accessToken != "" {
			fmt.Printf("   Access Token: %s\n", maskToken(accessToken))
		}
	},
}

// writeTokenCmd 表示 write 子命令（完全重写）
var writeTokenCmd = &cobra.Command{
	Use:   "write",
	Short: "写入 115 tokens（完全重写）",
	Long: `完全重写 115 tokens 文件。
必须同时提供 refresh_token 和 access_token。`,
	Run: func(cmd *cobra.Command, args []string) {
		refreshToken, _ := cmd.Flags().GetString("refresh-token")
		accessToken, _ := cmd.Flags().GetString("access-token")

		if refreshToken == "" || accessToken == "" {
			fmt.Fprintf(os.Stderr, "错误: 必须同时提供 --refresh-token 和 --access-token\n")
			os.Exit(1)
		}

		// 设置锁行为
		if err := configureLockBehavior(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}

		if err := storage.WriteTokens(refreshToken, accessToken); err != nil {
			fmt.Fprintf(os.Stderr, "错误: 写入 tokens 失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Tokens 写入成功!")
		fmt.Printf("   Refresh Token: %s\n", maskToken(refreshToken))
		fmt.Printf("   Access Token: %s\n", maskToken(accessToken))
	},
}

// showTokenCmd 表示 show 子命令
var showTokenCmd = &cobra.Command{
	Use:   "show",
	Short: "查看当前的 115 tokens",
	Long:  `显示当前存储的 115 tokens 信息。`,
	Run: func(cmd *cobra.Command, args []string) {
		refreshToken, accessToken, updatedAt, err := storage.GetTokens()
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 读取 tokens 失败: %v\n", err)
			os.Exit(1)
		}

		if refreshToken == "" && accessToken == "" {
			fmt.Println("📝 未找到任何 tokens")
			return
		}

		fmt.Println("📋 当前的 115 Tokens:")
		if refreshToken != "" {
			fmt.Printf("   Refresh Token: %s\n", maskToken(refreshToken))
		} else {
			fmt.Println("   Refresh Token: (未设置)")
		}

		if accessToken != "" {
			fmt.Printf("   Access Token: %s\n", maskToken(accessToken))
		} else {
			fmt.Println("   Access Token: (未设置)")
		}

		if !updatedAt.IsZero() {
			fmt.Printf("   更新时间: %s\n", updatedAt.Format("2006-01-02 15:04:05"))
		}
	},
}

// configureLockBehavior 根据命令行参数配置锁行为
func configureLockBehavior(cmd *cobra.Command) error {
	// 检查是否设置了超时时间
	if cmd.Flags().Changed("lock-timeout") {
		timeoutStr, _ := cmd.Flags().GetString("lock-timeout")
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("无效的超时时间格式: %v (例如: 30s, 1m, 5m30s)", err)
		}
		storage.FileLockTimeout = timeout
		fmt.Printf("🕐 文件锁超时设置为: %v\n", timeout)
	}

	// 检查是否设置了非阻塞模式
	noWait, _ := cmd.Flags().GetBool("no-wait")
	if noWait {
		// 设置超时为0，使用非阻塞模式
		storage.FileLockTimeout = 0
		fmt.Println("⚡ 使用非阻塞模式，如果锁被占用将立即返回")
	}

	return nil
}

// maskToken 掩码显示 token，只显示前后几位
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

func init() {
	// 将 token 命令添加到根命令
	rootCmd.AddCommand(tokenCmd)

	// 将子命令添加到 token 命令
	tokenCmd.AddCommand(setTokenCmd)
	tokenCmd.AddCommand(writeTokenCmd)
	tokenCmd.AddCommand(showTokenCmd)

	// 为所有需要写入的命令添加锁行为标志
	for _, cmd := range []*cobra.Command{setTokenCmd, writeTokenCmd} {
		cmd.Flags().StringP("refresh-token", "r", "", "设置 refresh token")
		cmd.Flags().StringP("access-token", "a", "", "设置 access token")
		cmd.Flags().String("lock-timeout", "30s", "文件锁超时时间 (例如: 30s, 1m, 5m)")
		cmd.Flags().Bool("no-wait", false, "不等待锁，如果被占用立即返回错误")
	}

	// 为 write 命令的帮助信息更新
	writeTokenCmd.Flags().Lookup("refresh-token").Usage = "设置 refresh token (必需)"
	writeTokenCmd.Flags().Lookup("access-token").Usage = "设置 access token (必需)"
}
