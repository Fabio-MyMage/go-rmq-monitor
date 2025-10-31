package slack

import "time"

// Message represents a Slack message with blocks
type Message struct {
	Text    string  `json:"text"`
	Blocks  []Block `json:"blocks,omitempty"`
	Channel string  `json:"channel,omitempty"`
}

// Block represents a Slack block
type Block struct {
	Type     string        `json:"type"`
	Text     *TextObject   `json:"text,omitempty"`
	Fields   []TextObject  `json:"fields,omitempty"`
	Elements []TextObject  `json:"elements,omitempty"`
}

// TextObject represents a Slack text object
type TextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeAlerting    AlertType = "alerting"
	AlertTypeNotAlerting AlertType = "not_alerting"
)

// QueueAlert contains information for Slack notifications
type QueueAlert struct {
	Type             AlertType
	QueueName        string
	VHost            string
	MessagesReady    int
	Consumers        int
	ConsumeRate      float64
	AckRate          float64
	PublishRate      float64
	ConsecutiveStuck int
	Reason           string
	Timestamp        time.Time
	StuckDuration    time.Duration // For recovery alerts
}
