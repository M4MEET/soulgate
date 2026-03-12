package gateway

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// RoutingStrategy defines how agents are selected
type RoutingStrategy string

const (
	StrategyRoundRobin  RoutingStrategy = "round_robin"
	StrategyLeastLoaded RoutingStrategy = "least_loaded"
	StrategyAffinity    RoutingStrategy = "affinity"
	StrategySmart       RoutingStrategy = "smart"
)

// Router handles intelligent agent selection
type Router struct {
	strategy      RoutingStrategy
	roundRobinIdx int
	mu            sync.Mutex

	// Agent load tracking
	agentLoads map[string]int // agentID -> active session count
	loadMux    sync.RWMutex
}

// NewRouter creates a new router
func NewRouter(strategy RoutingStrategy) *Router {
	if strategy == "" {
		strategy = StrategySmart
	}

	return &Router{
		strategy:   strategy,
		agentLoads: make(map[string]int),
	}
}

// SelectAgent selects an agent for a session
func (r *Router) SelectAgent(session *Session, agents map[string]*Client) (*Client, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agents available")
	}

	switch r.strategy {
	case StrategyRoundRobin:
		return r.selectRoundRobin(agents), nil

	case StrategyLeastLoaded:
		return r.selectLeastLoaded(agents), nil

	case StrategyAffinity:
		return r.selectWithAffinity(session, agents), nil

	case StrategySmart:
		return r.selectSmart(session, agents), nil

	default:
		return r.selectSmart(session, agents), nil
	}
}

// selectRoundRobin selects agents in round-robin fashion
func (r *Router) selectRoundRobin(agents map[string]*Client) *Client {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Convert map to slice for consistent ordering
	agentList := make([]*Client, 0, len(agents))
	for _, agent := range agents {
		agentList = append(agentList, agent)
	}

	if len(agentList) == 0 {
		return nil
	}

	// Select next agent
	agent := agentList[r.roundRobinIdx%len(agentList)]
	r.roundRobinIdx++

	return agent
}

// selectLeastLoaded selects the agent with the fewest active sessions
func (r *Router) selectLeastLoaded(agents map[string]*Client) *Client {
	r.loadMux.RLock()
	defer r.loadMux.RUnlock()

	var selectedAgent *Client
	minLoad := int(^uint(0) >> 1) // Max int

	for agentID, agent := range agents {
		load := r.agentLoads[agentID]
		if load < minLoad {
			minLoad = load
			selectedAgent = agent
		}
	}

	return selectedAgent
}

// selectWithAffinity prefers the agent that handled the session before
func (r *Router) selectWithAffinity(session *Session, agents map[string]*Client) *Client {
	// If session has affinity enabled and an assigned agent
	if session.AgentAffinityEnabled {
		assignedAgentID := session.GetAssignedAgent()
		if assignedAgentID != "" {
			// Check if assigned agent is still available
			if agent, exists := agents[assignedAgentID]; exists {
				return agent
			}
		}
	}

	// Fall back to least loaded
	return r.selectLeastLoaded(agents)
}

// selectSmart uses a combination of strategies for optimal routing
func (r *Router) selectSmart(session *Session, agents map[string]*Client) *Client {
	// 1. Check for affinity first (keep conversations with same agent)
	if session.AgentAffinityEnabled {
		assignedAgentID := session.GetAssignedAgent()
		if assignedAgentID != "" {
			if agent, exists := agents[assignedAgentID]; exists {
				// Check if agent is not overloaded
				r.loadMux.RLock()
				load := r.agentLoads[assignedAgentID]
				r.loadMux.RUnlock()

				// Use assigned agent if load is reasonable (<10 sessions)
				if load < 10 {
					return agent
				}
			}
		}
	}

	// 2. Select least loaded agent
	return r.selectLeastLoaded(agents)
}

// IncrementLoad increments the load counter for an agent
func (r *Router) IncrementLoad(agentID string) {
	r.loadMux.Lock()
	defer r.loadMux.Unlock()

	r.agentLoads[agentID]++
}

// DecrementLoad decrements the load counter for an agent
func (r *Router) DecrementLoad(agentID string) {
	r.loadMux.Lock()
	defer r.loadMux.Unlock()

	if r.agentLoads[agentID] > 0 {
		r.agentLoads[agentID]--
	}
}

// GetLoad returns the current load for an agent
func (r *Router) GetLoad(agentID string) int {
	r.loadMux.RLock()
	defer r.loadMux.RUnlock()

	return r.agentLoads[agentID]
}

// GetAllLoads returns load for all agents
func (r *Router) GetAllLoads() map[string]int {
	r.loadMux.RLock()
	defer r.loadMux.RUnlock()

	loads := make(map[string]int)
	for agentID, load := range r.agentLoads {
		loads[agentID] = load
	}

	return loads
}

// ResetLoad resets the load counter for an agent
func (r *Router) ResetLoad(agentID string) {
	r.loadMux.Lock()
	defer r.loadMux.Unlock()

	delete(r.agentLoads, agentID)
}

// RandomAgent selects a random agent (for testing/fallback)
func (r *Router) RandomAgent(agents map[string]*Client) *Client {
	if len(agents) == 0 {
		return nil
	}

	// Convert to slice
	agentList := make([]*Client, 0, len(agents))
	for _, agent := range agents {
		agentList = append(agentList, agent)
	}

	// Random selection
	rand.Seed(time.Now().UnixNano())
	return agentList[rand.Intn(len(agentList))]
}
