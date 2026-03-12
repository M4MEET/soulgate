package loader

import (
	"context"
	"fmt"
	"sync"

	"github.com/M4MEET/soulgate/internal/brokers/messaging"
)

// PluginRegistry holds registered Go plugins
// This is a temporary solution until full WASM plugin loading is implemented
type PluginRegistry struct {
	mu               sync.RWMutex
	channelFactories map[string]ChannelFactory
}

// ChannelFactory creates a messaging channel
type ChannelFactory func(ctx context.Context, config map[string]interface{}) (messaging.Channel, error)

var globalRegistry = &PluginRegistry{
	channelFactories: make(map[string]ChannelFactory),
}

// RegisterChannelPlugin registers a messaging channel plugin
func RegisterChannelPlugin(name string, factory ChannelFactory) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.channelFactories[name] = factory
}

// GetChannelFactory returns a channel factory by name
func GetChannelFactory(name string) (ChannelFactory, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	factory, ok := globalRegistry.channelFactories[name]
	return factory, ok
}

// ListChannelPlugins returns all registered channel plugin names
func ListChannelPlugins() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	names := make([]string, 0, len(globalRegistry.channelFactories))
	for name := range globalRegistry.channelFactories {
		names = append(names, name)
	}
	return names
}

// CreateChannel creates a channel instance from a plugin
func CreateChannel(ctx context.Context, name string, config map[string]interface{}) (messaging.Channel, error) {
	factory, ok := GetChannelFactory(name)
	if !ok {
		return nil, fmt.Errorf("channel plugin %s not registered", name)
	}
	return factory(ctx, config)
}
