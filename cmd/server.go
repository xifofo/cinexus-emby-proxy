package cmd

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cinexus/internal/config"
	"cinexus/internal/logger"
	"cinexus/internal/server"

	"github.com/spf13/cobra"
)

// serverCmd 表示 server 命令
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动 Web 服务器",
	Long: `使用配置的设置启动 Web 服务器。
服务器将运行直到收到关闭信号。`,
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

func runServer() {
	// 初始化配置
	cfg := config.Load()

	// 初始化日志记录器
	log := logger.New(cfg.Log)

	// 创建服务器
	srv := server.New(cfg, log)

	// 在协程中启动服务器
	go func() {
		log.Infof("在端口 %s 启动服务器", cfg.Server.Port)
		if err := srv.Start(":" + cfg.Server.Port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动服务器失败: %v", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("收到关闭信号，正在关闭服务器...")

	// 上下文用于通知服务器它有 10 秒时间完成当前正在处理的请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Info("正在停止HTTP服务器...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("服务器关闭过程中发生错误: %v", err)
		log.Info("强制关闭服务器...")
		os.Exit(1)
	}

	log.Info("服务器已成功退出")
}
