package cmd

import (
	"fmt"

	"go-rmq-monitor/internal/config"
	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test RabbitMQ connection",
	Long:  `Test connection to RabbitMQ Management API and display basic information.`,
	RunE:  runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	configPath := cfgFile
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("🔗 Connecting to: %s\n", cfg.RabbitMQ.GetRabbitMQURL())
	fmt.Printf("👤 Username: %s\n", cfg.RabbitMQ.Username)
	fmt.Printf("🔒 TLS: %v\n\n", cfg.RabbitMQ.UseTLS)

	client, err := rabbithole.NewClient(
		cfg.RabbitMQ.GetRabbitMQURL(),
		cfg.RabbitMQ.Username,
		cfg.RabbitMQ.Password,
	)
	if err != nil {
		return fmt.Errorf("❌ Failed to create client: %w", err)
	}

	// Test overview
	fmt.Println("✓ Testing API connection...")
	overview, err := client.Overview()
	if err != nil {
		return fmt.Errorf("❌ Failed to get overview: %w", err)
	}
	fmt.Printf("✓ Connected! RabbitMQ version: %s\n\n", overview.RabbitMQVersion)

	// List vhosts
	fmt.Println("📋 Available vhosts:")
	vhosts, err := client.ListVhosts()
	if err != nil {
		return fmt.Errorf("❌ Failed to list vhosts: %w", err)
	}

	for _, vh := range vhosts {
		marker := " "
		if vh.Name == cfg.RabbitMQ.VHost {
			marker = "→"
		}
		fmt.Printf("  %s %s\n", marker, vh.Name)
	}
	fmt.Println()

	// Try to list queues
	fmt.Printf("📊 Queues in vhost '%s':\n", cfg.RabbitMQ.VHost)
	queues, err := client.ListQueuesIn(cfg.RabbitMQ.VHost)
	if err != nil {
		fmt.Printf("❌ Failed to list queues: %v\n\n", err)
		fmt.Println("💡 Tip: Make sure the vhost name matches one from the list above")
		return nil
	}

	if len(queues) == 0 {
		fmt.Println("  (no queues found)")
	} else {
		for _, q := range queues {
			fmt.Printf("  • %s (messages: %d, consumers: %d)\n", q.Name, q.Messages, q.Consumers)
		}
	}
	fmt.Println()
	fmt.Println("✅ All checks passed!")
	return nil
}
