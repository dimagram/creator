package main

import (
	"log"
	"os"

	"dimagram/creator/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "dimagram",
	Short: "Dimagram backend tool",
	Long:  `Dimagram backend application with various commands.`,
}

func initConfig() {
	viper.SetEnvPrefix("dimagram")
	viper.AutomaticEnv()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(cmd.GetServerCmd())
	rootCmd.AddCommand(cmd.GetPublishCmd())
	rootCmd.AddCommand(cmd.GetUnpublishCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
