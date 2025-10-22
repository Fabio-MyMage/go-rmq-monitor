package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "go-rmq-monitor",
	Short: "A RabbitMQ queue monitoring tool",
	Long: `A CLI tool that continuously monitors RabbitMQ queues for stuck messages.
It detects queues where messages are not being processed and logs alerts to a file.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
}
