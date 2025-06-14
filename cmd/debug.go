package cmd

import (
	"cinexus/internal/config"
	"cinexus/internal/logger"
	"cinexus/internal/server/routes"

	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "调试命令",
	Long:  `这是一个用于调试的命令`,
	Run: func(cmd *cobra.Command, args []string) {
		// 加载配置
		cfg := config.Load()

		// 初始化日志
		log := logger.New(cfg.Log)

		// 记录调试信息
		log.WithField("config", cfg).Debug("当前配置信息")

		// 执行调试代码
		routes.GetNextMediaInfo("1255", cfg, log)
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
