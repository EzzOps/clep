package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Config holds all configuration
type Config struct {
	Port           int    `env:"PORT,default=8080"`
	NatsURL        string `env:"NATS_URL,default=nats://nats:4222"`
	PlaneSecret    string `env:"PLANE_WEBHOOK_SECRET,required"`
	PlaneAPIURL    string `env:"PLANE_API_URL,required"`
	PlaneAPIKey    string `env:"PLANE_API_KEY,required"`
	DatabaseURL    string `env:"DATABASE_URL,required"`
	HermesAdapter  string `env:"HERMES_ADAPTER_URL,default=http://hermes-adapter:8081"`
	EventStream    string `env:"EVENT_STREAM,default=clep.events"`
	ResponseStream string `env:"RESPONSE_STREAM,default=clep.responses"`
}

// Event represents a normalized CLEP event
type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"event_type"`
	Source    string          `json:"source"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	Status    string          `json:"status"`
}

// PlaneWebhookPayload is the raw webhook from Plane
type PlaneWebhookPayload struct {
	ID          string          `json:"id"`
	Event       string          `json:"event"`
	WorkspaceID string          `json:"workspace_id"`
	Resource    json.RawMessage `json:"resource"`
	Metadata    json.RawMessage `json:"metadata"`
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

// PlaneWorkItem is the work item from Plane
type PlaneWorkItem struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ProjectID   string `json:"project_id"`
	WorkspaceID string `json:"workspace_id"`
}

var (
	cfg  Config
	logger *zap.Logger
	nc   *nats.Conn
	db   *Database
)

func main() {
	cfg = loadConfig()
	logger, _ = zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting CLEP Gateway")
	logger.Info("Config loaded", zap.Int("port", cfg.Port))

	// Connect to NATS
	var err error
	nc, err = nats.Connect(cfg.NatsURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer nc.Close()
	logger.Info("Connected to NATS", zap.String("url", cfg.NatsURL))

	// Connect to database
	db = NewDatabase(cfg.DatabaseURL, logger)
	if err := db.Connect(); err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()
	logger.Info("Connected to database")

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		logger.Fatal("Failed to initialize schema", zap.Error(err))
	}

	// Setup router
	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().UTC()})
	})

	// Plane webhook endpoint
	r.POST("/webhooks/plane", handlePlaneWebhook)

	// Event status endpoint
	r.GET("/events/:id", getEventStatus)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("Starting HTTP server", zap.String("addr", addr))

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}
}

func loadConfig() Config {
	return Config{
		Port:           getEnvInt("PORT", 8080),
		NatsURL:        getEnv("NATS_URL", "nats://nats:4222"),
		PlaneSecret:    getEnv("PLANE_WEBHOOK_SECRET", ""),
		PlaneAPIURL:    getEnv("PLANE_API_URL", ""),
		PlaneAPIKey:    getEnv("PLANE_API_KEY", ""),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		HermesAdapter:  getEnv("HERMES_ADAPTER_URL", "http://hermes-adapter:8081"),
		EventStream:    getEnv("EVENT_STREAM", "clep.events"),
		ResponseStream: getEnv("RESPONSE_STREAM", "clep.responses"),
	}
}

func handlePlaneWebhook(c *gin.Context) {
	// Validate HMAC signature
	signature := c.GetHeader("X-Plane-Signature")
	if signature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Plane-Signature header"})
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	// Verify signature
	if !verifySignature(body, signature, cfg.PlaneSecret) {
		logger.Warn("Invalid webhook signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// Parse webhook payload
	var webhook PlaneWebhookPayload
	if err := json.Unmarshal(body, &webhook); err != nil {
		logger.Error("Failed to parse webhook", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	logger.Info("Received Plane webhook",
		zap.String("event", webhook.Event),
		zap.String("workspace", webhook.WorkspaceID),
	)

	// Handle different event types
	switch webhook.Event {
	case "issue.comment.create":
		handleIssueCommentCreate(c, webhook)
	default:
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "event": webhook.Event})
	}
}

func handleIssueCommentCreate(c *gin.Context, webhook PlaneWebhookPayload) {
	var comment PlaneComment
	if err := json.Unmarshal(webhook.Resource, &comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse comment"})
		return
	}

	// Check if comment mentions @hermes
	if !containsHermesMention(comment.Comment) {
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "reason": "no @hermes mention"})
		return
	}

	// Get previous comments
	previousComments, err := db.GetPreviousComments(comment.WorkItemID, comment.ID)
	if err != nil {
		logger.Error("Failed to get previous comments", zap.Error(err))
		previousComments = []PlaneComment{}
	}

	// Create normalized event
	event := createEvent("plane.comment.created", webhook, comment, previousComments)

	// Store event in database
	if err := db.StoreEvent(event); err != nil {
		logger.Error("Failed to store event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store event"})
		return
	}

	// Publish to NATS
	if err := publishEvent(event); err != nil {
		logger.Error("Failed to publish event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "accepted",
		"event_id":     event.ID,
		"work_item_id": comment.WorkItemID,
	})
}

func createEvent(eventType string, webhook PlaneWebhookPayload, comment PlaneComment, previousComments []PlaneComment) Event {
	payload, _ := json.Marshal(map[string]interface{}{
		"comment":           comment,
		"previous_comments": previousComments,
		"workspace_id":      webhook.WorkspaceID,
		"project_id":        comment.ProjectID,
		"mentioned_by":      comment.CreatedBy,
	})

	return Event{
		ID:        generateEventID(),
		Type:      eventType,
		Source:    "plane",
		Payload:   payload,
		Timestamp: time.Now().UTC(),
		Status:    "created",
	}
}

func containsHermesMention(text string) bool {
	return len(text) >= 7 && text[:7] == "@hermes"
}

func verifySignature(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(expectedMAC, []byte(signature))
}

func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		var n int
		fmt.Sscanf(val, "%d", &n)
		if n != 0 {
			return n
		}
	}
	return defaultVal
}

func publishEvent(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := &nats.Msg{
		Subject: cfg.EventStream,
		Data:    data,
		Header:  make(nats.Header),
	}
	msg.Header.Set("event-type", event.Type)
	msg.Header.Set("source", event.Source)
	msg.Header.Set("event-id", event.ID)
	msg.Header.Set("timestamp", event.Timestamp.Format(time.RFC3339))

	if err := nc.PublishMsg(msg); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

func getEventStatus(c *gin.Context) {
	eventID := c.Param("id")

	event, err := db.GetEvent(eventID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	c.JSON(http.StatusOK, event)
}
