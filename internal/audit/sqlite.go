package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// SQLiteLogger implements Logger using SQLite
type SQLiteLogger struct {
	db *sql.DB
}

// NewSQLiteLogger creates a new SQLite audit logger
func NewSQLiteLogger(dbPath string) (*SQLiteLogger, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	logger := &SQLiteLogger{db: db}

	if err := logger.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return logger, nil
}

// createSchema creates the audit events table
func (l *SQLiteLogger) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		session_id TEXT,
		run_id TEXT,
		type TEXT NOT NULL,
		category TEXT NOT NULL,
		plugin_id TEXT,
		action TEXT,
		resource TEXT,
		decision TEXT,
		status TEXT NOT NULL,
		error TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_events(session_id);
	CREATE INDEX IF NOT EXISTS idx_audit_run ON audit_events(run_id);
	CREATE INDEX IF NOT EXISTS idx_audit_type ON audit_events(type);
	CREATE INDEX IF NOT EXISTS idx_audit_category ON audit_events(category);
	CREATE INDEX IF NOT EXISTS idx_audit_status ON audit_events(status);
	`

	_, err := l.db.Exec(schema)
	return err
}

// Log records an audit event
func (l *SQLiteLogger) Log(ctx context.Context, event *Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Serialize metadata to JSON
	var metadataJSON []byte
	var err error
	if event.Metadata != nil && len(event.Metadata) > 0 {
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO audit_events (
			id, timestamp, session_id, run_id, type, category,
			plugin_id, action, resource, decision, status, error, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = l.db.ExecContext(ctx, query,
		event.ID,
		event.Timestamp,
		nullString(event.SessionID),
		nullString(event.RunID),
		event.Type,
		event.Category,
		nullString(event.PluginID),
		nullString(event.Action),
		nullString(event.Resource),
		nullString(string(event.Decision)),
		event.Status,
		nullString(event.Error),
		nullBytes(metadataJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// Query retrieves audit events based on filters
func (l *SQLiteLogger) Query(ctx context.Context, filter QueryFilter) ([]*Event, error) {
	var conditions []string
	var args []interface{}

	if filter.SessionID != "" {
		conditions = append(conditions, "session_id = ?")
		args = append(args, filter.SessionID)
	}

	if filter.RunID != "" {
		conditions = append(conditions, "run_id = ?")
		args = append(args, filter.RunID)
	}

	if filter.PluginID != "" {
		conditions = append(conditions, "plugin_id = ?")
		args = append(args, filter.PluginID)
	}

	if filter.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, filter.Type)
	}

	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, filter.Category)
	}

	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}

	if filter.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *filter.StartTime)
	}

	if filter.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *filter.EndTime)
	}

	query := "SELECT id, timestamp, session_id, run_id, type, category, plugin_id, action, resource, decision, status, error, metadata FROM audit_events"

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		var sessionID, runID, pluginID, action, resource, decision, errorMsg sql.NullString
		var metadataJSON sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.Timestamp,
			&sessionID,
			&runID,
			&event.Type,
			&event.Category,
			&pluginID,
			&action,
			&resource,
			&decision,
			&event.Status,
			&errorMsg,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		event.SessionID = sessionID.String
		event.RunID = runID.String
		event.PluginID = pluginID.String
		event.Action = action.String
		event.Resource = resource.String
		if decision.Valid {
			event.Decision = Decision(decision.String)
		}
		event.Error = errorMsg.String

		// Deserialize metadata
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return events, nil
}

// Close closes the database connection
func (l *SQLiteLogger) Close() error {
	return l.db.Close()
}

// Helper functions for nullable types
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullBytes(b []byte) sql.NullString {
	if len(b) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}
