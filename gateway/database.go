package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// Event represents a normalized CLEP event
type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"event_type"`
	Source    string          `json:"source"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	Status    string          `json:"status"`
}

// PlaneComment is the comment resource from Plane webhook
type PlaneComment struct {
	ID          string `json:"id"`
	Comment     string `json:"comment"`
	WorkItemID  string `json:"work_item_id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
}

// Database wraps PostgreSQL connection
type Database struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewDatabase creates a new Database instance
func NewDatabase(dsn string, logger *zap.Logger) *Database {
	return &Database{
		logger: logger,
	}
}

// Connect establishes database connection
func (d *Database) Connect() error {
	var err error
	d.db, err = sql.Open("postgres", d.logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := d.db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	d.logger.Info("Database connection established")
	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// StoreEvent persists an event to the database
func (d *Database) StoreEvent(event Event) error {
	query := `
		INSERT INTO events (id, event_type, source, payload, created_at, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`

	_, err := d.db.Exec(query,
		event.ID,
		event.Type,
		event.Source,
		event.Payload,
		event.Timestamp,
		event.Status,
	)

	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	d.logger.Info("Event stored", zap.String("event_id", event.ID))
	return nil
}

// GetEvent retrieves an event by ID
func (d *Database) GetEvent(id string) (Event, error) {
	var event Event
	query := `
		SELECT id, event_type, source, payload, created_at, status
		FROM events
		WHERE id = $1
	`

	err := d.db.QueryRow(query, id).Scan(
		&event.ID,
		&event.Type,
		&event.Source,
		&event.Payload,
		&event.Timestamp,
		&event.Status,
	)

	if err != nil {
		return event, fmt.Errorf("failed to get event: %w", err)
	}

	return event, nil
}

// UpdateEventStatus updates the status of an event
func (d *Database) UpdateEventStatus(id, status string) error {
	query := `UPDATE events SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := d.db.Exec(query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update event status: %w", err)
	}
	return nil
}

// CreateAgentSession creates a new agent session
func (d *Database) CreateAgentSession(eventID, workItemID, workspaceID string) (string, error) {
	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())

	query := `
		INSERT INTO agent_sessions (id, event_id, work_item_id, workspace_id, status, created_at)
		VALUES ($1, $2, $3, $4, 'running', NOW())
	`

	_, err := d.db.Exec(query, sessionID, eventID, workItemID, workspaceID)
	if err != nil {
		return "", fmt.Errorf("failed to create agent session: %w", err)
	}

	return sessionID, nil
}

// UpdateAgentSessionStatus updates the status of an agent session
func (d *Database) UpdateAgentSessionStatus(sessionID, status string) error {
	query := `UPDATE agent_sessions SET status = $2, completed_at = NOW() WHERE id = $1`
	_, err := d.db.Exec(query, sessionID, status)
	if err != nil {
		return fmt.Errorf("failed to update agent session status: %w", err)
	}
	return nil
}

// StoreAgentMessage stores a message from an agent
func (d *Database) StoreAgentMessage(sessionID, role, content string) error {
	query := `
		INSERT INTO agent_messages (session_id, role, content, created_at)
		VALUES ($1, $2, $3, NOW())
	`

	_, err := d.db.Exec(query, sessionID, role, content)
	if err != nil {
		return fmt.Errorf("failed to store agent message: %w", err)
	}
	return nil
}

// GetPreviousComments retrieves previous comments for a work item
func (d *Database) GetPreviousComments(workItemID, excludeCommentID string) ([]PlaneComment, error) {
	query := `
		SELECT id, comment, work_item_id, workspace_id, project_id, created_by, created_at
		FROM plane_comments
		WHERE work_item_id = $1 AND id != $2
		ORDER BY created_at ASC
		LIMIT 20
	`

	rows, err := d.db.Query(query, workItemID, excludeCommentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get previous comments: %w", err)
	}
	defer rows.Close()

	var comments []PlaneComment
	for rows.Next() {
		var c PlaneComment
		if err := rows.Scan(&c.ID, &c.Comment, &c.WorkItemID, &c.WorkspaceID, &c.ProjectID, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}
		comments = append(comments, c)
	}

	return comments, nil
}

// InitSchema creates the necessary tables
func (d *Database) InitSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL,
			source TEXT NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ,
			status TEXT NOT NULL DEFAULT 'created'
		);

		CREATE TABLE IF NOT EXISTS agent_sessions (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'running',
			created_at TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ
		);

		CREATE TABLE IF NOT EXISTS agent_messages (
			id SERIAL PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);

		CREATE TABLE IF NOT EXISTS plane_comments (
			id TEXT PRIMARY KEY,
			comment TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
		CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
		CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
		CREATE INDEX IF NOT EXISTS idx_agent_sessions_event ON agent_sessions(event_id);
		CREATE INDEX IF NOT EXISTS idx_agent_sessions_status ON agent_sessions(status);
		CREATE INDEX IF NOT EXISTS idx_plane_comments_work_item ON plane_comments(work_item_id);
	`

	_, err := d.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to init schema: %w", err)
	}

	d.logger.Info("Database schema initialized")
	return nil
}
