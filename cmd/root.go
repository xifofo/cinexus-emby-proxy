package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd 表示没有任何子命令时调用的基础命令
var rootCmd = &cobra.Command{
	Use:   "cinexus",
	Short: "Cinexus 是一个现代化的 Web 应用框架",
	Long: `Cinexus 是一个基于 Echo、Cobra 和 Viper 构建的现代化 Web 应用框架。
它为构建可扩展的 Web 应用提供了坚实的基础，包含自动日志记录、配置管理等功能。`,
}

// Execute 将所有子命令添加到根命令，并适当设置标志。
// 这由 main.main() 调用。只需要对 rootCmd 执行一次。
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// 在这里定义你的标志和配置设置。
	// Cobra 支持持久标志，如果在这里定义，将对应用程序全局生效。

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件 (默认是 $HOME/.cinexus.yaml)")

	// Cobra 也支持本地标志，只有在直接调用此操作时才会运行
	rootCmd.Flags().BoolP("toggle", "t", false, "切换选项的帮助信息")
}

// initConfig 读取配置文件和环境变量（如果设置）
func initConfig() {
	if cfgFile != "" {
		// 使用标志中的配置文件
		viper.SetConfigFile(cfgFile)
	} else {
		// 查找主目录
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// 在主目录中搜索名为 ".cinexus" 的配置（不包含扩展名）
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // 读取匹配的环境变量

	// 如果找到配置文件，读取它
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "使用配置文件:", viper.ConfigFileUsed())
	}
}