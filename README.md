# RabbitMQ Queue Monitor

A Go CLI application that continuously monitors RabbitMQ queues to detect stuck queues (queues where messages are not being processed) and logs alerts to a file.

## Features

- 🔍 **Continuous Monitoring** - Periodically checks queue status at configurable intervals
- 📊 **Smart Detection** - Uses multiple metrics to identify stuck queues:
  - Message count trends
  - Consumer count
  - Consume/acknowledgment rates
  - Threshold-based
- ⚙️ **Flexible Configuration** - YAML-based configuration for all settings
- 📝 **Structured Logging** - JSON or text format logging to file and stdout
- 🎯 **Selective Monitoring** - Monitor all queues or specific queues only

## Installation

### Download Pre-built Binary

Download the latest release for your platform from the [Releases page](https://github.com/Fabio-MyMage/go-rmq-monitor/releases).

```bash
# Example for Linux amd64
wget https://github.com/Fabio-MyMage/go-rmq-monitor/releases/download/v0.0.1/go-rmq-monitor_0.0.1_Linux_x86_64.tar.gz
tar -xzf go-rmq-monitor_0.0.1_Linux_x86_64.tar.gz
sudo mv go-rmq-monitor /usr/local/bin/
```

### Prerequisites

- Go 1.21 or higher (for building from source)
- RabbitMQ with Management Plugin enabled

### Build from Source

```bash
# Clone the repository
git clone https://github.com/Fabio-MyMage/go-rmq-monitor.git
cd go-rmq-monitor

# Download dependencies
go mod download

# Build the application
go build -o go-rmq-monitor

# Optional: Install to system path
sudo mv go-rmq-monitor /usr/local/bin/
```

## Configuration

Create a `config.yaml` file with your RabbitMQ connection details and monitoring preferences:

```yaml
rabbitmq:
  host: "localhost"
  port: 15672
  username: "guest"
  password: "guest"
  vhost: "/"
  use_tls: false

monitor:
  # How often to check queue status
  interval: 60s
  
  detection:
    # Number of consecutive checks before flagging as stuck
    threshold_checks: 3
    # Minimum message count to consider (ignore nearly-empty queues)
    min_message_count: 10
    # Minimum consume rate (messages/second) - below this is considered stuck
    min_consume_rate: 0.1
  
  # Optional: specific queues to monitor
  # If empty or omitted, all queues in the vhost will be monitored
  queues:
    - "order-processing"
    - "notification-queue"

logging:
  file_path: "./logs/stuck-queues.log"
  level: "info"  # debug, info, warn, error
  format: "json"  # json or text
```

### Configuration Options

#### RabbitMQ Settings

- `host` - RabbitMQ server hostname
- `port` - Management API port (default: 15672)
- `username` - RabbitMQ username
- `password` - RabbitMQ password
- `vhost` - Virtual host to monitor
- `use_tls` - Enable TLS/SSL for API connection

#### Monitor Settings

- `interval` - How often to check queues (e.g., `60s`, `5m`, `1h`)
- `detection.threshold_checks` - Consecutive checks before alerting (reduces false positives)
- `detection.min_message_count` - Ignore queues with fewer messages
- `detection.min_consume_rate` - Minimum messages/second consumption rate
- `queues` - List of specific queue names to monitor (empty = monitor all)

#### Logging Settings

- `file_path` - Path to log file (directory will be created if needed)
- `level` - Log level: `debug`, `info`, `warn`, `error`
- `format` - Log format: `json` or `text`

## Usage

```bash
# Use default config.yaml in current directory
./go-rmq-monitor monitor

# Use custom config file
./go-rmq-monitor monitor --config /path/to/config.yaml
```

## Deployment

### Systemd Service

Create `/etc/systemd/system/rabbitmq-monitor.service`:

```ini
[Unit]
Description=RabbitMQ Queue Monitor
After=network.target

[Service]
Type=simple
User=rabbitmq-monitor
WorkingDirectory=/opt/rabbitmq-monitor
ExecStart=/opt/rabbitmq-monitor/go-rmq-monitor monitor --config /etc/rabbitmq-monitor/config.yaml
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable rabbitmq-monitor
sudo systemctl start rabbitmq-monitor
sudo systemctl status rabbitmq-monitor
```

### Docker

Create a `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o go-rmq-monitor

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/go-rmq-monitor .
COPY config.yaml .
CMD ["./go-rmq-monitor", "monitor"]
```

Build and run:
```bash
docker build -t rabbitmq-monitor .
docker run -v $(pwd)/config.yaml:/root/config.yaml rabbitmq-monitor
```
