package scanner

import (
	"log"
	"sync"
	"time"
)

// StructuredLog represents a JSON-marshallable log entry
type StructuredLog struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	TaskID    uint                   `json:"task_id"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// LoggerService manages structured logging
type LoggerService struct {
	buffer chan *StructuredLog
	done   chan bool
	mu     sync.Mutex
}

// NewLoggerService creates a new logger service
func NewLoggerService(bufferSize int) *LoggerService {
	ls := &LoggerService{
		buffer: make(chan *StructuredLog, bufferSize),
		done:   make(chan bool),
	}
	return ls
}

// LogError logs an error message
func LogError(taskID uint, message string, context map[string]interface{}) {
	log.Printf("[ERROR] Task %d: %s | Context: %v\n", taskID, message, context)
}

// LogWarning logs a warning message
func LogWarning(taskID uint, message string) {
	log.Printf("[WARN] Task %d: %s\n", taskID, message)
}

// LogInfo logs an info message
func LogInfo(taskID uint, message string) {
	log.Printf("[INFO] Task %d: %s\n", taskID, message)
}

// AgentHealthMonitor monitors agent connectivity
type AgentHealthMonitor struct {
	agents map[string]*AgentState
	mu     sync.RWMutex
	ticker *time.Ticker
}

// AgentState tracks agent status
type AgentState struct {
	AgentID         string
	LastHeartbeat   time.Time
	Connected       bool
	AssociatedTasks map[uint]bool
}

// NewAgentHealthMonitor creates a new health monitor
func NewAgentHealthMonitor(checkInterval time.Duration) *AgentHealthMonitor {
	return &AgentHealthMonitor{
		agents: make(map[string]*AgentState),
		ticker: time.NewTicker(checkInterval),
	}
}

// RegisterAgent registers a new agent
func (ahm *AgentHealthMonitor) RegisterAgent(agentID string) {
	ahm.mu.Lock()
	defer ahm.mu.Unlock()

	if _, exists := ahm.agents[agentID]; !exists {
		ahm.agents[agentID] = &AgentState{
			AgentID:         agentID,
			LastHeartbeat:   time.Now(),
			Connected:       true,
			AssociatedTasks: make(map[uint]bool),
		}
		log.Printf("[AGENT_REGISTERED] Agent %s registered\n", agentID)
	}
}

// Heartbeat records agent heartbeat
func (ahm *AgentHealthMonitor) Heartbeat(agentID string) {
	ahm.mu.Lock()
	defer ahm.mu.Unlock()

	if agent, exists := ahm.agents[agentID]; exists {
		agent.LastHeartbeat = time.Now()
		agent.Connected = true
	}
}

// GetDisconnectedAgents returns agents that haven't heartbeat recently
func (ahm *AgentHealthMonitor) GetDisconnectedAgents(timeout time.Duration) []string {
	ahm.mu.RLock()
	defer ahm.mu.RUnlock()

	var disconnected []string
	now := time.Now()

	for agentID, agent := range ahm.agents {
		if agent.Connected && now.Sub(agent.LastHeartbeat) > timeout {
			disconnected = append(disconnected, agentID)
		}
	}

	return disconnected
}

// Stop gracefully shuts down the monitor
func (ahm *AgentHealthMonitor) Stop() {
	if ahm.ticker != nil {
		ahm.ticker.Stop()
	}
}
