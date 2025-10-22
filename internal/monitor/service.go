package monitor

import (
	"fmt"
	"sync"
	"time"

	"go-rmq-monitor/internal/analyzer"
	"go-rmq-monitor/internal/config"
	"go-rmq-monitor/internal/logger"
	"go-rmq-monitor/internal/rabbitmq"
)

// Service manages the monitoring process
type Service struct {
	config         *config.Config
	logger         *logger.Logger
	client         *rabbitmq.Client
	analyzer       *analyzer.Analyzer
	queueIntervals map[string]time.Duration // Per-queue check intervals
	lastCheckTimes map[string]time.Time     // Track last check time per queue
	startTime      time.Time                 // Service start time for synchronized checks
	verbosity      int                       // Verbosity level (1=info, 2=+healthy, 3=+each check)
	stopChan       chan struct{}
	wg             sync.WaitGroup
	running        bool
	mu             sync.Mutex
}

// New creates a new monitor service
func New(cfg *config.Config, log *logger.Logger, verbosity int) (*Service, error) {
	// Create RabbitMQ client
	client, err := rabbitmq.NewClient(&cfg.RabbitMQ)
	if err != nil {
		return nil, fmt.Errorf("failed to create RabbitMQ client: %w", err)
	}

	// Create analyzer with global defaults
	analyzer := analyzer.New(&cfg.Monitor.Detection)

	// Configure per-queue settings and intervals
	queueIntervals := make(map[string]time.Duration)
	lastCheckTimes := make(map[string]time.Time)
	
	// Log monitored queues at startup if verbosity >= 2
	if verbosity >= 2 {
		log.Info("Configured queue monitoring", map[string]interface{}{
			"total_queues": len(cfg.Monitor.Queues),
		})
	}
	
	for _, queueCfg := range cfg.Monitor.Queues {
		detectionCfg := queueCfg.GetDetectionConfig(cfg.Monitor.Detection)
		analyzer.SetQueueConfig(queueCfg.Name, detectionCfg)
		
		checkInterval := queueCfg.GetCheckInterval(cfg.Monitor.Interval)
		queueIntervals[queueCfg.Name] = checkInterval
		
		// Log queue configuration if verbosity >= 2
		if verbosity >= 2 {
			log.Info("Queue configuration", map[string]interface{}{
				"queue":             queueCfg.Name,
				"check_interval":    checkInterval.String(),
				"threshold_checks":  detectionCfg.ThresholdChecks,
				"min_message_count": detectionCfg.MinMessageCount,
				"min_consume_rate":  detectionCfg.MinConsumeRate,
			})
		} else {
			log.Debug("Configured queue monitoring", map[string]interface{}{
				"queue":             queueCfg.Name,
				"check_interval":    checkInterval.String(),
				"threshold_checks":  detectionCfg.ThresholdChecks,
				"min_message_count": detectionCfg.MinMessageCount,
				"min_consume_rate":  detectionCfg.MinConsumeRate,
			})
		}
	}

	return &Service{
		config:         cfg,
		logger:         log,
		client:         client,
		analyzer:       analyzer,
		queueIntervals: queueIntervals,
		lastCheckTimes: lastCheckTimes,
		startTime:      time.Now(), // Record start time for synchronized checks
		verbosity:      verbosity,
		stopChan:       make(chan struct{}),
	}, nil
}

// Start begins the monitoring process
func (s *Service) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("monitor is already running")
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Monitor service started", nil)

	// Determine the shortest check interval (base ticker frequency)
	tickerInterval := s.config.Monitor.Interval
	for _, interval := range s.queueIntervals {
		if interval < tickerInterval {
			tickerInterval = interval
		}
	}
	
	s.logger.Info("Monitoring ticker interval", map[string]interface{}{
		"interval": tickerInterval.String(),
	})

	// Create ticker for periodic checks
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	// Run first check immediately
	if err := s.performCheck(); err != nil {
		s.logger.Error("Initial check failed", err, nil)
	}

	// Main monitoring loop
	for {
		select {
		case <-ticker.C:
			if err := s.performCheck(); err != nil {
				s.logger.Error("Check failed", err, nil)
			}
		case <-s.stopChan:
			s.logger.Info("Stopping monitor service", nil)
			return nil
		}
	}
}

// Stop gracefully stops the monitoring process
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)
	s.wg.Wait()
}

// performCheck performs a single monitoring check
func (s *Service) performCheck() error {
	now := time.Now()

	// Fetch queue information
	allQueues, err := s.client.GetQueues()
	if err != nil {
		return fmt.Errorf("failed to fetch queues: %w", err)
	}

	s.logger.Debug("Fetched queues", map[string]interface{}{
		"count": len(allQueues),
	})

	// Filter queues if specific queues are configured
	allQueuesToMonitor := rabbitmq.FilterQueues(allQueues, s.config.Monitor.Queues)

	// Filter based on per-queue check intervals
	queuesToCheck := make([]rabbitmq.QueueInfo, 0)
	for _, queue := range allQueuesToMonitor {
		// Get the check interval for this queue (or use global default)
		checkInterval, exists := s.queueIntervals[queue.Name]
		if !exists {
			checkInterval = s.config.Monitor.Interval
		}

		// Check if this queue is due for checking
		// Option B: Synchronized checking - check if elapsed time from start is a multiple of interval
		timeSinceStart := now.Sub(s.startTime)
		intervalsSinceStart := int(timeSinceStart / checkInterval)
		nextCheckTime := s.startTime.Add(time.Duration(intervalsSinceStart) * checkInterval)
		
		// Also check if we haven't checked since the last expected check time
		lastCheck, hasBeenChecked := s.lastCheckTimes[queue.Name]
		shouldCheck := false
		
		if !hasBeenChecked {
			// First check - always check
			shouldCheck = true
		} else {
			// Check if we've passed the next scheduled check time and haven't checked since
			shouldCheck = now.Sub(nextCheckTime) >= 0 && lastCheck.Before(nextCheckTime)
		}
		
		if shouldCheck {
			queuesToCheck = append(queuesToCheck, queue)
			s.lastCheckTimes[queue.Name] = now
			
			// Log each check run if verbosity >= 3
			if s.verbosity >= 3 {
				timeSinceLastCheck := "first check"
				if hasBeenChecked {
					timeSinceLastCheck = now.Sub(lastCheck).String()
				}
				s.logger.Info("Checking queue", map[string]interface{}{
					"queue":          queue.Name,
					"messages_ready": queue.MessagesReady,
					"consumers":      queue.Consumers,
					"consume_rate":   queue.ConsumeRate,
					"check_interval": checkInterval.String(),
					"since_last":     timeSinceLastCheck,
				})
			} else {
				s.logger.Debug("Checking queue", map[string]interface{}{
					"queue":          queue.Name,
					"check_interval": checkInterval.String(),
				})
			}
		}
	}
	if len(queuesToCheck) == 0 {
		s.logger.Debug("No queues due for checking", nil)
		return nil
	}

	s.logger.Debug("Monitoring queues", map[string]interface{}{
		"count": len(queuesToCheck),
	})

	// Analyze queues for stuck status
	alerts := s.analyzer.Analyze(queuesToCheck)

	// Log any stuck queue alerts
	for _, alert := range alerts {
		s.logStuckQueue(alert)
	}

	// Log results based on verbosity
	if len(alerts) > 0 {
		s.logger.Info("Stuck queues detected", map[string]interface{}{
			"count": len(alerts),
		})
	} else {
		// Log healthy queues if verbosity >= 2
		if s.verbosity >= 2 {
			healthyQueues := make([]string, 0, len(queuesToCheck))
			for _, q := range queuesToCheck {
				healthyQueues = append(healthyQueues, q.Name)
			}
			s.logger.Info("All checked queues healthy", map[string]interface{}{
				"queues": healthyQueues,
				"count":  len(queuesToCheck),
			})
		} else {
			s.logger.Debug("All queues healthy", nil)
		}
	}

	return nil
}

// logStuckQueue logs a stuck queue alert
func (s *Service) logStuckQueue(alert analyzer.StuckQueueAlert) {
	s.logger.Warn("STUCK QUEUE DETECTED", map[string]interface{}{
		"queue":            alert.QueueName,
		"messages_ready":   alert.MessagesReady,
		"consumers":        alert.Consumers,
		"consume_rate":     alert.ConsumeRate,
		"ack_rate":         alert.AckRate,
		"consecutive_stuck": alert.ConsecutiveStuck,
		"reason":           alert.Reason,
		"timestamp":        alert.Timestamp.Format(time.RFC3339),
	})
}
