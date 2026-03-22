package a2a

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Client connects to a remote A2A agent.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates a client for the given agent URL.
func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// WithAPIKey sets a Bearer token for authentication.
func (c *Client) WithAPIKey(key string) *Client {
	c.apiKey = key
	return c
}

// Discover fetches the remote agent's card.
func (c *Client) Discover() (*AgentCard, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/.well-known/agent.json", nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a: discover failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("a2a: discover returned HTTP %d", resp.StatusCode)
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("a2a: invalid agent card: %w", err)
	}
	return &card, nil
}

// SendMessage sends a message to the remote agent and waits for completion.
func (c *Client) SendMessage(msg Message) (*Task, error) {
	params := SendMessageParams{Message: msg}
	body, _ := json.Marshal(params)

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/a2a/message/send", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a: send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("a2a: send returned HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Task *Task `json:"task"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("a2a: invalid response: %w", err)
	}
	return result.Task, nil
}

// SendStreamingMessage sends a message and streams events back via callback.
func (c *Client) SendStreamingMessage(msg Message, onEvent func(StreamResponse)) (*Task, error) {
	params := SendMessageParams{Message: msg}
	body, _ := json.Marshal(params)

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/a2a/message/stream", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a: stream failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("a2a: stream returned HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var lastTask *Task
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var evt StreamResponse
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}
		if evt.Task != nil {
			lastTask = evt.Task
		}
		if onEvent != nil {
			onEvent(evt)
		}
	}
	return lastTask, nil
}

// GetTask retrieves a task by ID.
func (c *Client) GetTask(taskID string) (*Task, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/a2a/tasks/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a: get task failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Task *Task `json:"task"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Task, nil
}

// CancelTask cancels a running task.
func (c *Client) CancelTask(taskID string) error {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/a2a/tasks/"+taskID+"/cancel", nil)
	if err != nil {
		return err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("a2a: cancel failed: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("a2a: cancel returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ListTasks lists tasks on the remote agent.
func (c *Client) ListTasks(contextID string) ([]*Task, error) {
	url := c.baseURL + "/a2a/tasks"
	if contextID != "" {
		url += "?contextId=" + contextID
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Tasks []*Task `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Tasks, nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// --------------------------------------------------------------------------
// Agent Registry (persists known remote agents)
// --------------------------------------------------------------------------

// AgentRegistry tracks discovered remote A2A agents.
type AgentRegistry struct {
	mu      sync.RWMutex
	agents  map[string]*RemoteAgent // URL → agent
	dataDir string
}

// NewAgentRegistry creates a persistent agent registry.
func NewAgentRegistry(dataDir string) *AgentRegistry {
	r := &AgentRegistry{
		agents:  make(map[string]*RemoteAgent),
		dataDir: dataDir,
	}
	r.load()
	return r
}

// Add discovers and registers a remote agent by URL.
func (r *AgentRegistry) Add(url string) (*RemoteAgent, error) {
	client := NewClient(url)
	card, err := client.Discover()
	if err != nil {
		return nil, err
	}

	agent := &RemoteAgent{
		URL:      url,
		Card:     *card,
		AddedAt:  time.Now().UTC(),
		LastSeen: time.Now().UTC(),
		Status:   "online",
	}

	r.mu.Lock()
	r.agents[url] = agent
	r.save()
	r.mu.Unlock()

	return agent, nil
}

// Remove unregisters a remote agent.
func (r *AgentRegistry) Remove(url string) {
	r.mu.Lock()
	delete(r.agents, url)
	r.save()
	r.mu.Unlock()
}

// List returns all registered remote agents.
func (r *AgentRegistry) List() []*RemoteAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*RemoteAgent, 0, len(r.agents))
	for _, a := range r.agents {
		cp := *a
		out = append(out, &cp)
	}
	return out
}

// Get returns a remote agent by URL.
func (r *AgentRegistry) Get(url string) (*RemoteAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[url]
	if !ok {
		return nil, false
	}
	cp := *a
	return &cp, true
}

// Refresh re-discovers all registered agents and updates their status.
func (r *AgentRegistry) Refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for url, agent := range r.agents {
		client := NewClient(url)
		card, err := client.Discover()
		if err != nil {
			agent.Status = "offline"
		} else {
			agent.Card = *card
			agent.LastSeen = time.Now().UTC()
			agent.Status = "online"
		}
	}
	r.save()
}

const registryFile = "state/a2a_agents.json"

func (r *AgentRegistry) save() {
	if r.dataDir == "" {
		return
	}
	path := filepath.Join(r.dataDir, registryFile)
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	data, _ := json.MarshalIndent(r.agents, "", "  ")
	_ = os.WriteFile(path, data, 0600)
}

func (r *AgentRegistry) load() {
	if r.dataDir == "" {
		return
	}
	path := filepath.Join(r.dataDir, registryFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var agents map[string]*RemoteAgent
	if json.Unmarshal(data, &agents) == nil && agents != nil {
		r.agents = agents
	}
}
