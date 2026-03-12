package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store manages integration credentials and configuration
type Store struct {
	mu           sync.RWMutex
	configPath   string
	integrations map[string]map[string]string // integration name -> config map
}

// NewStore creates a new integration store
func NewStore(configDir string) (*Store, error) {
	configPath := filepath.Join(configDir, "integrations.json")

	store := &Store{
		configPath:   configPath,
		integrations: make(map[string]map[string]string),
	}

	// Load existing config if it exists
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load integrations config: %w", err)
	}

	return store, nil
}

// Save saves an integration's configuration
func (s *Store) Save(name string, config map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.integrations[name] = config

	return s.persist()
}

// Get retrieves an integration's configuration
func (s *Store) Get(name string) (map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.integrations[name]
	return config, exists
}

// Delete removes an integration's configuration
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.integrations, name)

	return s.persist()
}

// List returns all configured integration names
func (s *Store) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.integrations))
	for name := range s.integrations {
		names = append(names, name)
	}

	return names
}

// load reads the config from disk
func (s *Store) load() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.integrations)
}

// persist writes the config to disk
func (s *Store) persist() error {
	data, err := json.Marshal(s.integrations)
	if err != nil {
		return fmt.Errorf("failed to marshal integrations: %w", err)
	}

	// Write with restrictive permissions (only owner can read/write)
	if err := os.WriteFile(s.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write integrations config: %w", err)
	}

	return nil
}

// IntegrationConfig represents a stored integration configuration
type IntegrationConfig struct {
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}

// Export exports all integrations (for backup)
func (s *Store) Export() []IntegrationConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make([]IntegrationConfig, 0, len(s.integrations))
	for name, config := range s.integrations {
		configs = append(configs, IntegrationConfig{
			Name:   name,
			Config: config,
		})
	}

	return configs
}
