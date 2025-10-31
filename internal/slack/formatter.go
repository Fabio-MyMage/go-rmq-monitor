package slack

import (
	"fmt"
	"time"
)

// FormatAlert formats a queue alert for Slack
func FormatAlert(alert QueueAlert) Message {
	if alert.Type == AlertTypeRecovered {
		return formatRecoveryMessage(alert)
	}
	return formatStuckMessage(alert)
}

// formatStuckMessage creates a Slack message for a stuck queue
func formatStuckMessage(alert QueueAlert) Message {
	timestamp := alert.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC")

	return Message{
		Text: fmt.Sprintf("🚨 Queue `%s` is stuck!", alert.QueueName),
		Blocks: []Block{
			{
				Type: "header",
				Text: &TextObject{
					Type: "plain_text",
					Text: "🚨 Stuck Queue Alert",
				},
			},
			{
				Type: "section",
				Fields: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("*Queue:*\n`%s`", alert.QueueName)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*VHost:*\n`%s`", alert.VHost)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Messages:*\n%s 📊", formatNumber(alert.MessagesReady))},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Consumers:*\n%d 👷", alert.Consumers)},
				},
			},
			{
				Type: "section",
				Fields: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("*Consume Rate:*\n%.2f msg/s", alert.ConsumeRate)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Ack Rate:*\n%.2f msg/s", alert.AckRate)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Publish Rate:*\n%.2f msg/s", alert.PublishRate)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Consecutive Stuck:*\n%d checks", alert.ConsecutiveStuck)},
				},
			},
			{
				Type: "section",
				Text: &TextObject{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Problem:* %s", alert.Reason),
				},
			},
			{
				Type: "context",
				Elements: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("🕒 Alerted at: %s", timestamp)},
				},
			},
		},
	}
}

// formatRecoveryMessage creates a Slack message for a recovered queue
func formatRecoveryMessage(alert QueueAlert) Message {
	timestamp := alert.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC")
	duration := formatDuration(alert.StuckDuration)

	return Message{
		Text: fmt.Sprintf("✅ Queue `%s` has recovered!", alert.QueueName),
		Blocks: []Block{
			{
				Type: "header",
				Text: &TextObject{
					Type: "plain_text",
					Text: "✅ Queue Recovered",
				},
			},
			{
				Type: "section",
				Fields: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("*Queue:*\n`%s`", alert.QueueName)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*VHost:*\n`%s`", alert.VHost)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Was Stuck For:*\n%s ⏱️", duration)},
					{Type: "mrkdwn", Text: "*Status:*\n🟢 Healthy"},
				},
			},
			{
				Type: "section",
				Fields: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("*Current Messages:*\n%s", formatNumber(alert.MessagesReady))},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Consumers:*\n%d", alert.Consumers)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Consume Rate:*\n%.2f msg/s", alert.ConsumeRate)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Ack Rate:*\n%.2f msg/s", alert.AckRate)},
				},
			},
			{
				Type: "section",
				Fields: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("*Publish Rate:*\n%.2f msg/s", alert.PublishRate)},
					{Type: "mrkdwn", Text: "*Status:*\n🟢 Healthy"},
				},
			},
			{
				Type: "context",
				Elements: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("🕒 Recovered at: %s", timestamp)},
				},
			},
		},
	}
}

// formatNumber formats a number with commas
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}

// formatDuration formats a duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds == 0 {
			return fmt.Sprintf("%d minutes", minutes)
		}
		return fmt.Sprintf("%d minutes %d seconds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d hours %d minutes", hours, minutes)
}
