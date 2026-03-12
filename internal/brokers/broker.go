package brokers

// Broker is the base interface for all resource brokers
type Broker interface {
	// Name returns the broker name
	Name() string

	// Close closes the broker and releases resources
	Close() error
}

// BrokerContext contains common context for broker operations
type BrokerContext struct {
	WorkspaceRoot string
	PluginID      string
	RunID         string
	SessionID     string
}
