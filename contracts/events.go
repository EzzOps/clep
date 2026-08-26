// Event schemas for CLEP system

package contracts

import (
	"time"
)

// PlaneCommentEvent represents a comment created in Plane
type PlaneCommentEvent struct {
	ID          string    `json:"id"`
	Comment     string    `json:"comment"`
	WorkItemID  string    `json:"work_item_id"`
	WorkspaceID string    `json:"workspace_id"`
	ProjectID   string    `json:"project_id"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// WorkItem represents a Plane work item
type WorkItem struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ProjectID   string `json:"project_id"`
	WorkspaceID string `json:"workspace_id"`
}

// CLEPEvent is the normalized event format
type CLEPEvent struct {
	ID        string      `json:"id"`
	Type      string      `json:"event_type"`
	Source    string      `json:"source"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
	Status    string      `json:"status"`
}

// AgentSession represents a Hermes agent session
type AgentSession struct {
	ID         string    `json:"id"`
	EventID    string    `json:"event_id"`
	WorkItemID string    `json:"work_item_id"`
	WorkspaceID string   `json:"workspace_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// AgentMessage represents a message in an agent session
type AgentMessage struct {
	ID        int       `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// EventStatus represents the status of an event
type EventStatus string

const (
	EventStatusCreated   EventStatus = "created"
	EventStatusProcessing EventStatus = "processing"
	EventStatusStreaming EventStatus = "streaming"
	EventStatusCompleted EventStatus = "completed"
	EventStatusFailed    EventStatus = "failed"
)

// SessionStatus represents the status of an agent session
type SessionStatus string

const (
	SessionStatusRunning  SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed   SessionStatus = "failed"
)
