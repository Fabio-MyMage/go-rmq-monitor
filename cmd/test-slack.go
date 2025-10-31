package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go-rmq-monitor/internal/slack"
)

// TestSlackData represents the JSON input for testing RMQ Slack alerts
type TestSlackData struct {
	QueueName        string  `json:"queue_name"`
	VHost            string  `json:"vhost"`
	MessagesReady    int     `json:"messages_ready"`
	Consumers        int     `json:"consumers"`
	ConsumeRate      float64 `json:"consume_rate"`
	AckRate          float64 `json:"ack_rate"`
	PublishRate      float64 `json:"publish_rate"`
	ConsecutiveStuck int     `json:"consecutive_stuck"`
	Reason           string  `json:"reason"`
}

var testSlackCmd = &cobra.Command{
	Use:   "test-slack <webhook-url> <alert-json>",
	Short: "Test Slack notifications with custom alert data",
	Long: `Test Slack notifications by sending a sample alert or recovery message.

Examples:
  # Test alerting notification
  go-rmq-monitor test-slack "https://hooks.slack.com/..." '{"queue_name":"orders","vhost":"/","messages_ready":1000,"consumers":0,"consume_rate":0.0,"ack_rate":0.0,"publish_rate":15.5,"consecutive_stuck":5,"reason":"no active consumers and messages not being processed"}'

  # Test recovery notification
  go-rmq-monitor test-slack "https://hooks.slack.com/..." '{"queue_name":"orders","vhost":"/","messages_ready":50,"consumers":2,"consume_rate":12.5,"ack_rate":12.3,"publish_rate":15.5,"consecutive_stuck":0,"reason":"Queue recovered"}' --recovery`,
	Args: cobra.ExactArgs(2),
	RunE: runTestSlack,
}

var recoveryFlag bool

func init() {
	rootCmd.AddCommand(testSlackCmd)
	testSlackCmd.Flags().BoolVar(&recoveryFlag, "recovery", false, "Send a recovery notification instead of alerting")
}

func runTestSlack(cmd *cobra.Command, args []string) error {
	webhookURL := args[0]
	alertJSON := args[1]

	// Parse the JSON input
	var testData TestSlackData
	if err := json.Unmarshal([]byte(alertJSON), &testData); err != nil {
		return fmt.Errorf("failed to parse alert JSON: %w", err)
	}

	// Create the alert
	alertType := slack.AlertTypeAlerting
	stuckDuration := time.Duration(0)
	
	if recoveryFlag {
		alertType = slack.AlertTypeNotAlerting
		// For recovery, use a default stuck duration for demo
		stuckDuration = 15 * time.Minute
	}

	alert := slack.QueueAlert{
		Type:             alertType,
		QueueName:        testData.QueueName,
		VHost:            testData.VHost,
		MessagesReady:    testData.MessagesReady,
		Consumers:        testData.Consumers,
		ConsumeRate:      testData.ConsumeRate,
		AckRate:          testData.AckRate,
		PublishRate:      testData.PublishRate,
		ConsecutiveStuck: testData.ConsecutiveStuck,
		Reason:           testData.Reason,
		Timestamp:        time.Now(),
		StuckDuration:    stuckDuration,
	}

	// Create Slack client and send alert
	config := slack.Config{
		Enabled:     true,
		WebhookURLs: []string{webhookURL},
		Timeout:     10 * time.Second,
	}
	client := slack.New(config)
	
	fmt.Printf("Sending %s notification to Slack...\n", alertType)
	fmt.Printf("Webhook URL: %s\n", webhookURL)
	fmt.Printf("Alert data: %+v\n\n", alert)
	
	if err := client.SendAlert(alert); err != nil {
		return fmt.Errorf("failed to send Slack alert: %w", err)
	}

	fmt.Printf("✅ Successfully sent %s notification to Slack!\n", alertType)
	return nil
}