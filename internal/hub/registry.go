package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Registry tracks installed hub items
type Registry struct {
	path      string
	installed map[string]InstalledItem
	mu        sync.RWMutex
}

// NewRegistry creates a new registry
func NewRegistry(configDir string) (*Registry, error) {
	path := filepath.Join(configDir, "hub", "installed.json")

	r := &Registry{
		path:      path,
		installed: make(map[string]InstalledItem),
	}

	// Load existing registry
	if err := r.load(); err != nil {
		// If file doesn't exist, that's okay
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load registry: %w", err)
		}
	}

	return r, nil
}

// Add adds an installed item
func (r *Registry) Add(item InstalledItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s/%s", item.Type, item.Name)
	item.InstalledAt = time.Now()
	item.UpdatedAt = time.Now()

	r.installed[key] = item

	return r.save()
}

// Remove removes an installed item
func (r *Registry) Remove(itemType, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s/%s", itemType, name)
	delete(r.installed, key)

	return r.save()
}

// Update updates an installed item
func (r *Registry) Update(itemType, name, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s/%s", itemType, name)
	if item, exists := r.installed[key]; exists {
		item.Version = version
		item.UpdatedAt = time.Now()
		r.installed[key] = item
		return r.save()
	}

	return fmt.Errorf("item not found: %s", key)
}

// Get gets an installed item
func (r *Registry) Get(itemType, name string) (*InstalledItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", itemType, name)
	if item, exists := r.installed[key]; exists {
		return &item, nil
	}

	return nil, fmt.Errorf("item not found: %s", key)
}

// List lists all installed items
func (r *Registry) List() []InstalledItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]InstalledItem, 0, len(r.installed))
	for _, item := range r.installed {
		items = append(items, item)
	}

	return items
}

// ListByType lists installed items by type
func (r *Registry) ListByType(itemType string) []InstalledItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]InstalledItem, 0)
	for _, item := range r.installed {
		if item.Type == itemType {
			items = append(items, item)
		}
	}

	return items
}

// IsInstalled checks if an item is installed
func (r *Registry) IsInstalled(itemType, name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", itemType, name)
	_, exists := r.installed[key]
	return exists
}

// load loads the registry from disk
func (r *Registry) load() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}

	var items []InstalledItem
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("failed to unmarshal registry: %w", err)
	}

	// Convert to map
	for _, item := range items {
		key := fmt.Sprintf("%s/%s", item.Type, item.Name)
		r.installed[key] = item
	}

	return nil
}

// save saves the registry to disk
func (r *Registry) save() error {
	// Convert map to slice
	items := make([]InstalledItem, 0, len(r.installed))
	for _, item := range r.installed {
		items = append(items, item)
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	// Create directory if needed
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(r.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry: %w", err)
	}

	return nil
}

// Count returns the number of installed items
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.installed)
}

// CountByType returns the number of installed items by type
func (r *Registry) CountByType(itemType string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, item := range r.installed {
		if item.Type == itemType {
			count++
		}
	}
	return count
}
