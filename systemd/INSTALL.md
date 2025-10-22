# Installation Instructions for Systemd Service

## Prerequisites

1. Build the application:
```bash
make build
```

2. Create a dedicated user:
```bash
sudo useradd -r -s /bin/false rabbitmq-monitor
```

3. Create necessary directories:
```bash
sudo mkdir -p /opt/rabbitmq-monitor
sudo mkdir -p /etc/rabbitmq-monitor
sudo mkdir -p /var/log/rabbitmq-monitor
```

## Installation Steps

1. Copy the binary:
```bash
sudo cp go-rmq-monitor /opt/rabbitmq-monitor/
sudo chown rabbitmq-monitor:rabbitmq-monitor /opt/rabbitmq-monitor/go-rmq-monitor
sudo chmod +x /opt/rabbitmq-monitor/go-rmq-monitor
```

2. Copy the configuration:
```bash
sudo cp config.yaml /etc/rabbitmq-monitor/config.yaml
sudo chown rabbitmq-monitor:rabbitmq-monitor /etc/rabbitmq-monitor/config.yaml
sudo chmod 600 /etc/rabbitmq-monitor/config.yaml
```

3. Update the config file with your RabbitMQ credentials:
```bash
sudo nano /etc/rabbitmq-monitor/config.yaml
```

4. Set proper permissions on log directory:
```bash
sudo chown rabbitmq-monitor:rabbitmq-monitor /var/log/rabbitmq-monitor
sudo chmod 755 /var/log/rabbitmq-monitor
```

5. Install the systemd service:
```bash
sudo cp systemd/rabbitmq-monitor.service /etc/systemd/system/
sudo systemctl daemon-reload
```

6. Enable and start the service:
```bash
sudo systemctl enable rabbitmq-monitor
sudo systemctl start rabbitmq-monitor
```

## Managing the Service

Check status:
```bash
sudo systemctl status rabbitmq-monitor
```

View logs:
```bash
sudo journalctl -u rabbitmq-monitor -f
```

Restart the service:
```bash
sudo systemctl restart rabbitmq-monitor
```

Stop the service:
```bash
sudo systemctl stop rabbitmq-monitor
```

Disable the service:
```bash
sudo systemctl disable rabbitmq-monitor
```

## Log Rotation

Create `/etc/logrotate.d/rabbitmq-monitor`:

```
/var/log/rabbitmq-monitor/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    missingok
    create 0644 rabbitmq-monitor rabbitmq-monitor
    postrotate
        systemctl reload rabbitmq-monitor > /dev/null 2>&1 || true
    endscript
}
```

## Troubleshooting

1. Check service status:
```bash
sudo systemctl status rabbitmq-monitor
```

2. Check logs:
```bash
sudo journalctl -u rabbitmq-monitor --no-pager
```

3. Test configuration:
```bash
sudo -u rabbitmq-monitor /opt/rabbitmq-monitor/go-rmq-monitor monitor --config /etc/rabbitmq-monitor/config.yaml
```

4. Verify RabbitMQ connection:
```bash
curl -u guest:guest http://localhost:15672/api/overview
```
