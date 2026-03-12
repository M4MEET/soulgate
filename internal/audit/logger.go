package audit

import (
	"context"
	"time"
)

// Logger defines the interface for audit logging
type Logger interface {
	// Log records an audit event
	Log(ctx context.Context, event *Event) error

	// Query retrieves audit events based on filters
	Query(ctx context.Context, filter QueryFilter) ([]*Event, error)

	// Close closes the logger and releases resources
	Close() error
}

// QueryFilter defines filters for querying audit events
type QueryFilter struct {
	SessionID string
	RunID     string
	PluginID  string
	Type      EventType
	Category  EventCategory
	Status    EventStatus
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

// DefaultQueryFilter returns a default query filter
func DefaultQueryFilter() QueryFilter {
	return QueryFilter{
		Limit: 100,
	}
}
