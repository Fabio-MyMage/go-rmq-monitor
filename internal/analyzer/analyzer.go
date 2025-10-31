package analyzer

import (
	"sync"
	"time"

	"go-rmq-monitor/internal/config"
	"go-rmq-monitor/internal/rabbitmq"
)

// QueueState tracks the state of a queue over time
type QueueState struct {
	QueueName        string
	History          []QueueSnapshot
	ConsecutiveStuck int
	LastAlertTime    time.Time
	LastSlackAlert   time.Time     // Track last Slack notification time
	LastKnownState   string        // "healthy" or "stuck"
	StuckSince       time.Time     // When queue became stuck (for recovery duration)
}

// QueueSnapshot represents queue metrics at a point in time
type QueueSnapshot struct {
	Timestamp     time.Time
	MessagesReady int
	ConsumeRate   float64
	AckRate       float64
	Consumers     int
}

// StuckQueueAlert contains information about a stuck queue
type StuckQueueAlert struct {
	QueueName        string
	Timestamp        time.Time
	MessagesReady    int
	Consumers        int
	ConsumeRate      float64
	AckRate          float64
	ConsecutiveStuck int
	Reason           string
	// Detection parameters used
	ThresholdChecks  int
	MinMessageCount  int
	MinConsumeRate   float64
}

// StateTransition represents a queue state change
type StateTransition struct {
	QueueName     string
	FromState     string // "healthy" or "stuck"
	ToState       string // "healthy" or "stuck"
	Timestamp     time.Time
	StuckDuration time.Duration // For stuck→healthy transitions
	QueueInfo     rabbitmq.QueueInfo
	Reason        string // Reason for the transition (for stuck state)
}

// AnalysisResult contains both alerts and state transitions
type AnalysisResult struct {
	StuckAlerts     []StuckQueueAlert
	Transitions     []StateTransition
}

// Analyzer analyzes queue health and detects stuck queues
type Analyzer struct {
	defaultConfig *config.DetectionConfig
	queueConfigs  map[string]config.DetectionConfig // Per-queue configs
	states        map[string]*QueueState
	mu            sync.RWMutex
}

// New creates a new queue analyzer
func New(cfg *config.DetectionConfig) *Analyzer {
	return &Analyzer{
		defaultConfig: cfg,
		queueConfigs:  make(map[string]config.DetectionConfig),
		states:        make(map[string]*QueueState),
	}
}

// SetQueueConfig sets a specific detection config for a queue
func (a *Analyzer) SetQueueConfig(queueName string, cfg config.DetectionConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.queueConfigs[queueName] = cfg
}

// getConfigForQueue returns the detection config for a specific queue
func (a *Analyzer) getConfigForQueue(queueName string) config.DetectionConfig {
	if cfg, exists := a.queueConfigs[queueName]; exists {
		return cfg
	}
	return *a.defaultConfig
}

// Analyze processes queue information and detects stuck queues
func (a *Analyzer) Analyze(queues []rabbitmq.QueueInfo) AnalysisResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	alerts := make([]StuckQueueAlert, 0)
	transitions := make([]StateTransition, 0)
	now := time.Now()

	for _, queue := range queues {
		// Get queue-specific config
		queueConfig := a.getConfigForQueue(queue.Name)
		
		// Get or create state for this queue
		state, exists := a.states[queue.Name]
		if !exists {
			state = &QueueState{
				QueueName: queue.Name,
				History:   make([]QueueSnapshot, 0),
			}
			a.states[queue.Name] = state
		}

		// Add current snapshot
		snapshot := QueueSnapshot{
			Timestamp:     now,
			MessagesReady: queue.MessagesReady,
			ConsumeRate:   queue.ConsumeRate,
			AckRate:       queue.AckRate,
			Consumers:     queue.Consumers,
		}
		state.History = append(state.History, snapshot)

		// Keep only recent history (threshold_checks + 1 to allow comparison)
		maxHistory := queueConfig.ThresholdChecks + 1
		if len(state.History) > maxHistory {
			state.History = state.History[len(state.History)-maxHistory:]
		}

		// Check if queue is stuck (using queue-specific config)
		if isStuck, reason := a.isQueueStuck(state, queueConfig); isStuck {
			state.ConsecutiveStuck++
			
			// Check for state transition: healthy → stuck
			if state.LastKnownState != "stuck" && state.ConsecutiveStuck >= queueConfig.ThresholdChecks {
				// State changed from healthy to stuck
				transition := StateTransition{
					QueueName: queue.Name,
					FromState: "healthy",
					ToState:   "stuck",
					Timestamp: now,
					QueueInfo: queue,
					Reason:    reason,
				}
				transitions = append(transitions, transition)
				state.LastKnownState = "stuck"
				state.StuckSince = now
			}
			
			// Only alert if we've crossed the threshold
			if state.ConsecutiveStuck >= queueConfig.ThresholdChecks {
				// Avoid duplicate alerts within 5 minutes
				if now.Sub(state.LastAlertTime) >= 5*time.Minute {
					alert := StuckQueueAlert{
						QueueName:        queue.Name,
						Timestamp:        now,
						MessagesReady:    queue.MessagesReady,
						Consumers:        queue.Consumers,
						ConsumeRate:      queue.ConsumeRate,
						AckRate:          queue.AckRate,
						ConsecutiveStuck: state.ConsecutiveStuck,
						Reason:           reason,
						// Include detection parameters for context
						ThresholdChecks:  queueConfig.ThresholdChecks,
						MinMessageCount:  queueConfig.MinMessageCount,
						MinConsumeRate:   queueConfig.MinConsumeRate,
					}
					alerts = append(alerts, alert)
					state.LastAlertTime = now
				}
			}
		} else {
			// Queue is healthy
			// Check for state transition: stuck → healthy
			if state.LastKnownState == "stuck" {
				// State changed from stuck to healthy
				stuckDuration := now.Sub(state.StuckSince)
				transition := StateTransition{
					QueueName:     queue.Name,
					FromState:     "stuck",
					ToState:       "healthy",
					Timestamp:     now,
					StuckDuration: stuckDuration,
					QueueInfo:     queue,
				}
				transitions = append(transitions, transition)
				state.LastKnownState = "healthy"
			}
			
			// Reset counter if queue is healthy
			state.ConsecutiveStuck = 0
		}
	}

	return AnalysisResult{
		StuckAlerts: alerts,
		Transitions: transitions,
	}
}

// isQueueStuck determines if a queue is stuck based on its history
func (a *Analyzer) isQueueStuck(state *QueueState, cfg config.DetectionConfig) (bool, string) {
	// Need enough history to make a determination
	if len(state.History) < cfg.ThresholdChecks {
		return false, ""
	}

	latest := state.History[len(state.History)-1]

	// Ignore queues with few messages (or empty queues)
	if latest.MessagesReady <= cfg.MinMessageCount {
		return false, ""
	}

	// Check 1: Low or zero consume/ack rate (check this FIRST)
	// This handles both dedicated workers and cron-based consumption
	// Note: If min_consume_rate < 0, rate checking is disabled (only checks message count trends)
	hasActivity := cfg.MinConsumeRate < 0 || latest.ConsumeRate >= cfg.MinConsumeRate || latest.AckRate >= cfg.MinConsumeRate
	
	if !hasActivity {
		// No consumption activity - check if messages are decreasing
		if a.isMessageCountStagnant(state, cfg) {
			// No activity AND messages not decreasing
			if latest.Consumers == 0 {
				return true, "no active consumers and messages not being processed"
			}
			return true, "consume rate below threshold and messages not decreasing"
		}
		// Messages ARE decreasing despite low rate - queue is healthy (e.g., cron-based)
		return false, ""
	}

	// Check 2: Messages not decreasing over time despite activity
	// This catches cases where consumers exist but aren't actually processing
	if a.isMessageCountStagnant(state, cfg) {
		return true, "messages not decreasing despite consumer activity"
	}

	return false, ""
}

// isMessageCountStagnant checks if message count is stable or increasing
func (a *Analyzer) isMessageCountStagnant(state *QueueState, cfg config.DetectionConfig) bool {
	if len(state.History) < 2 {
		return false
	}

	// Get the last N snapshots
	recentHistory := state.History
	if len(recentHistory) > cfg.ThresholdChecks {
		recentHistory = recentHistory[len(recentHistory)-cfg.ThresholdChecks:]
	}

	// Check if messages are consistently high
	firstCount := recentHistory[0].MessagesReady
	lastCount := recentHistory[len(recentHistory)-1].MessagesReady

	// If both are at or below min threshold, queue is healthy (empty or nearly empty)
	// This prevents false positives when a queue stays at 0 messages
	if firstCount <= 0 && lastCount <= 0 {
		return false
	}

	// Consider it stagnant only if:
	// 1. Message count increased, OR
	// 2. Message count stayed exactly the same (and above 0), OR
	// 3. Message count decreased by less than 1 message per check on average
	//
	// This prevents false positives for slow-processing queues that ARE making progress
	if lastCount > firstCount {
		// Messages increased - definitely stuck
		return true
	}
	
	if lastCount == firstCount {
		// No change at all - stuck (we already filtered out the 0==0 case above)
		return true
	}
	
	// Calculate minimum expected decrease (at least 1 message per check interval)
	checksSpanned := len(recentHistory) - 1
	minExpectedDecrease := checksSpanned // At least 1 message per check
	actualDecrease := firstCount - lastCount
	
	// If we haven't seen at least 1 message processed per check, consider it stagnant
	if actualDecrease < minExpectedDecrease {
		return true
	}

	return false
}

// Reset clears all tracked state (useful for testing)
func (a *Analyzer) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.states = make(map[string]*QueueState)
}

// GetState returns the current state for a queue (for debugging/testing)
func (a *Analyzer) GetState(queueName string) (*QueueState, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	state, exists := a.states[queueName]
	return state, exists
}

// GetQueueState returns the current state for a queue (for Slack notifications)
func (a *Analyzer) GetQueueState(queueName string) *QueueState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.states[queueName]
}
