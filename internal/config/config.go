package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Monitor  MonitorConfig  `mapstructure:"monitor"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// RabbitMQConfig contains RabbitMQ connection details
type RabbitMQConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	VHost    string `mapstructure:"vhost"`
	UseTLS   bool   `mapstructure:"use_tls"`
}

// MonitorConfig contains monitoring behavior settings
type MonitorConfig struct {
	Interval  time.Duration   `mapstructure:"interval"`
	Detection DetectionConfig `mapstructure:"detection"`
	Queues    []QueueConfig   `mapstructure:"queues"`
}

// QueueConfig represents a queue to monitor with optional overrides
type QueueConfig struct {
	Name            string         `mapstructure:"name"`
	CheckInterval   *time.Duration `mapstructure:"check_interval,omitempty"`
	ThresholdChecks *int           `mapstructure:"threshold_checks,omitempty"`
	MinMessageCount *int           `mapstructure:"min_message_count,omitempty"`
	MinConsumeRate  *float64       `mapstructure:"min_consume_rate,omitempty"`
}

// DetectionConfig contains stuck queue detection parameters
type DetectionConfig struct {
	ThresholdChecks int     `mapstructure:"threshold_checks"`
	MinMessageCount int     `mapstructure:"min_message_count"`
	MinConsumeRate  float64 `mapstructure:"min_consume_rate"`
}

// GetDetectionConfig returns the effective detection config for a queue
// Applies queue-specific overrides on top of global defaults
func (q *QueueConfig) GetDetectionConfig(globalDefaults DetectionConfig) DetectionConfig {
	config := globalDefaults

	// Apply overrides if specified
	if q.ThresholdChecks != nil {
		config.ThresholdChecks = *q.ThresholdChecks
	}
	if q.MinMessageCount != nil {
		config.MinMessageCount = *q.MinMessageCount
	}
	if q.MinConsumeRate != nil {
		config.MinConsumeRate = *q.MinConsumeRate
	}

	return config
}

// GetCheckInterval returns the effective check interval for a queue
// Uses queue-specific interval or falls back to global default
func (q *QueueConfig) GetCheckInterval(globalDefault time.Duration) time.Duration {
	if q.CheckInterval != nil {
		return *q.CheckInterval
	}
	return globalDefault
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	FilePath string `mapstructure:"file_path"`
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
}

// Load reads and parses the configuration file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file path
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	v.SetDefault("rabbitmq.host", "localhost")
	v.SetDefault("rabbitmq.port", 15672)
	v.SetDefault("rabbitmq.username", "guest")
	v.SetDefault("rabbitmq.password", "guest")
	v.SetDefault("rabbitmq.vhost", "/")
	v.SetDefault("rabbitmq.use_tls", false)

	v.SetDefault("monitor.interval", "60s")
	v.SetDefault("monitor.detection.threshold_checks", 3)
	v.SetDefault("monitor.detection.min_message_count", 10)
	v.SetDefault("monitor.detection.min_consume_rate", 0.1)

	v.SetDefault("logging.file_path", "/var/log/rabbitmq-monitor/stuck-queues.log")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}

// validate performs basic validation on the configuration
func validate(cfg *Config) error {
	if cfg.RabbitMQ.Host == "" {
		return fmt.Errorf("rabbitmq.host is required")
	}
	if cfg.RabbitMQ.Port <= 0 || cfg.RabbitMQ.Port > 65535 {
		return fmt.Errorf("rabbitmq.port must be between 1 and 65535")
	}
	if cfg.Monitor.Interval <= 0 {
		return fmt.Errorf("monitor.interval must be positive")
	}
	if cfg.Monitor.Detection.ThresholdChecks < 1 {
		return fmt.Errorf("monitor.detection.threshold_checks must be at least 1")
	}
	if cfg.Logging.FilePath == "" {
		return fmt.Errorf("logging.file_path is required")
	}

	return nil
}

// GetRabbitMQURL returns the RabbitMQ management API URL
func (c *RabbitMQConfig) GetRabbitMQURL() string {
	scheme := "http"
	if c.UseTLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, c.Host, c.Port)
}
