package rabbitmq

import (
	"fmt"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	"go-rmq-monitor/internal/config"
)

// Client wraps the RabbitMQ management API client
type Client struct {
	client *rabbithole.Client
	vhost  string
}

// QueueInfo contains relevant queue metrics
type QueueInfo struct {
	Name            string
	VHost           string
	MessagesReady   int
	Messages        int
	Consumers       int
	ConsumeRate     float64
	AckRate         float64
	PublishRate     float64
	State           string
}

// NewClient creates a new RabbitMQ API client
func NewClient(cfg *config.RabbitMQConfig) (*Client, error) {
	baseURL := cfg.GetRabbitMQURL()
	
	client, err := rabbithole.NewClient(baseURL, cfg.Username, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create RabbitMQ client: %w", err)
	}

	// Test connection
	if _, err := client.Overview(); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	return &Client{
		client: client,
		vhost:  cfg.VHost,
	}, nil
}

// GetQueues returns information about all queues in the vhost
func (c *Client) GetQueues() ([]QueueInfo, error) {
	// Pass vhost directly - rabbit-hole library handles URL encoding internally
	queues, err := c.client.ListQueuesIn(c.vhost)
	if err != nil {
		return nil, fmt.Errorf("failed to list queues: %w", err)
	}

	result := make([]QueueInfo, 0, len(queues))
	for _, q := range queues {
		info := c.convertQueueInfo(&q)
		result = append(result, info)
	}

	return result, nil
}

// GetQueue returns information about a specific queue
func (c *Client) GetQueue(queueName string) (*QueueInfo, error) {
	// Pass vhost and queue name directly - rabbit-hole library handles URL encoding internally
	queue, err := c.client.GetQueue(c.vhost, queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue %s: %w", queueName, err)
	}

	info := c.convertDetailedQueueInfo(queue)
	return &info, nil
}

// convertQueueInfo converts rabbithole.QueueInfo to our QueueInfo
func (c *Client) convertQueueInfo(q *rabbithole.QueueInfo) QueueInfo {
	info := QueueInfo{
		Name:          q.Name,
		VHost:         q.Vhost,
		MessagesReady: q.MessagesReady,
		Messages:      q.Messages,
		Consumers:     q.Consumers,
		State:         "",
	}

	// Extract rates from message stats
	if q.MessageStats != nil {
		info.ConsumeRate = float64(q.MessageStats.DeliverGetDetails.Rate)
		info.AckRate = float64(q.MessageStats.AckDetails.Rate)
		info.PublishRate = float64(q.MessageStats.PublishDetails.Rate)
	}

	return info
}

// convertDetailedQueueInfo converts rabbithole.DetailedQueueInfo to our QueueInfo
func (c *Client) convertDetailedQueueInfo(q *rabbithole.DetailedQueueInfo) QueueInfo {
	info := QueueInfo{
		Name:          q.Name,
		VHost:         q.Vhost,
		MessagesReady: q.MessagesReady,
		Messages:      q.Messages,
		Consumers:     q.Consumers,
		State:         "", // State field not available in v3
	}

	// Extract rates from message stats
	if q.MessageStats != nil {
		info.ConsumeRate = float64(q.MessageStats.DeliverGetDetails.Rate)
		info.AckRate = float64(q.MessageStats.AckDetails.Rate)
		info.PublishRate = float64(q.MessageStats.PublishDetails.Rate)
	}

	return info
}

// FilterQueues returns only the queues specified in the filter list
// If the filter list is empty, returns all queues
func FilterQueues(allQueues []QueueInfo, filter []config.QueueConfig) []QueueInfo {
	if len(filter) == 0 {
		return allQueues
	}

	filterMap := make(map[string]bool)
	for _, qCfg := range filter {
		filterMap[qCfg.Name] = true
	}

	result := make([]QueueInfo, 0)
	for _, q := range allQueues {
		if filterMap[q.Name] {
			result = append(result, q)
		}
	}

	return result
}
