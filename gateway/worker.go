package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func main() {
	cfg := loadConfig()
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting CLEP Hermes Adapter")

	// Connect to NATS
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer nc.Close()

	// Connect to database
	db := NewDatabase(cfg.DatabaseURL, logger)
	if err := db.Connect(); err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		logger.Fatal("Failed to initialize schema", zap.Error(err))
	}

	// Create Plane API client
	planeAPI := NewPlaneAPI(cfg.PlaneAPIURL, cfg.PlaneAPIKey)

	// Subscribe to events
	subject := cfg.EventStream
	logger.Info("Subscribing to NATS subject", zap.String("subject", subject))

	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		handleEvent(msg, db, planeAPI, logger)
	})
	if err != nil {
		logger.Fatal("Failed to subscribe to NATS", zap.Error(err))
	}

	// Setup router
	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().UTC()})
	})

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("Starting Hermes Adapter", zap.String("addr", addr))

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
	logger.Info("Shutting down Hermes Adapter...")
}

func handleEvent(msg *nats.Msg, db *Database, planeAPI *PlaneAPI, logger *zap.Logger) {
	var event Event
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		logger.Error("Failed to unmarshal event", zap.Error(err))
		return
	}

	logger.Info("Processing event",
		zap.String("event_id", event.ID),
		zap.String("type", event.Type),
	)

	// Update event status
	if err := db.UpdateEventStatus(event.ID, "processing"); err != nil {
		logger.Error("Failed to update event status", zap.Error(err))
		return
	}

	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		logger.Error("Failed to parse payload", zap.Error(err))
		return
	}

	// Extract work item ID from payload
	workItemID, _ := payload["work_item_id"].(string)
	if workItemID == "" {
		// Try nested structure
		if workItem, ok := payload["work_item"].(map[string]interface{}); ok {
			if id, ok := workItem["id"].(string); ok {
				workItemID = id
			}
		}
	}

	if workItemID == "" {
		logger.Warn("No work item ID found in event payload")
		if err := db.UpdateEventStatus(event.ID, "failed"); err != nil {
			logger.Error("Failed to update event status", zap.Error(err))
		}
		return
	}

	// Create agent session
	sessionID, err := db.CreateAgentSession(event.ID, workItemID, "")
	if err != nil {
		logger.Error("Failed to create agent session", zap.Error(err))
		return
	}

	// Build context for Hermes
	context := buildContext(payload, logger)

	// Create Hermes comment in Plane
	hermesComment, err := planeAPI.CreateComment(workItemID, "🤖 Hermes is thinking...")
	if err != nil {
		logger.Error("Failed to create Hermes comment", zap.Error(err))
		if err := db.UpdateAgentSessionStatus(sessionID, "failed"); err != nil {
			logger.Error("Failed to update session status", zap.Error(err))
		}
		return
	}

	// Update event status
	if err := db.UpdateEventStatus(event.ID, "streaming"); err != nil {
		logger.Error("Failed to update event status", zap.Error(err))
		return
	}

	// Stream response from Hermes
	var fullResponse string
	err = streamHermesResponse(context, sessionID, func(chunk string) {
		fullResponse += chunk

		// Update Plane comment incrementally
		if len(fullResponse)%500 == 0 || len(fullResponse) > 2000 {
			if updateErr := planeAPI.UpdateComment(hermesComment.ID, formatHermesResponse(fullResponse)); updateErr != nil {
				logger.Error("Failed to update comment", zap.Error(updateErr))
			}
		}
	})

	if err != nil {
		logger.Error("Failed to stream Hermes response", zap.Error(err))
		if err := db.UpdateAgentSessionStatus(sessionID, "failed"); err != nil {
			logger.Error("Failed to update session status", zap.Error(err))
		}
		if err := planeAPI.UpdateComment(hermesComment.ID, "❌ Hermes encountered an error. Please try again."); err != nil {
			logger.Error("Failed to update error comment", zap.Error(err))
		}
		return
	}

	// Final update
	if err := planeAPI.UpdateComment(hermesComment.ID, formatHermesResponse(fullResponse)); err != nil {
		logger.Error("Failed to update final comment", zap.Error(err))
	}

	// Update session status
	if err := db.UpdateAgentSessionStatus(sessionID, "completed"); err != nil {
		logger.Error("Failed to update session status", zap.Error(err))
	}

	// Update event status
	if err := db.UpdateEventStatus(event.ID, "completed"); err != nil {
		logger.Error("Failed to update event status", zap.Error(err))
	}

	// Store final message
	if err := db.StoreAgentMessage(sessionID, "assistant", fullResponse); err != nil {
		logger.Error("Failed to store agent message", zap.Error(err))
	}

	logger.Info("Event processed successfully",
		zap.String("event_id", event.ID),
		zap.String("session_id", sessionID),
		zap.String("comment_id", hermesComment.ID),
	)
}

func buildContext(payload map[string]interface{}, logger *zap.Logger) map[string]interface{} {
	context := make(map[string]interface{})

	// Extract comment info
	if comment, ok := payload["comment"].(map[string]interface{}); ok {
		context["comment"] = comment
	}

	// Extract previous comments
	if prevComments, ok := payload["previous_comments"].([]interface{}); ok {
		context["previous_comments"] = prevComments
	}

	// Extract workspace info
	if workspaceID, ok := payload["workspace_id"].(string); ok {
		context["workspace_id"] = workspaceID
	}

	// Extract project info
	if projectID, ok := payload["project_id"].(string); ok {
		context["project_id"] = projectID
	}

	logger.Info("Context built", zap.Any("context_keys", getKeys(context)))
	return context
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func streamHermesResponse(context map[string]interface{}, sessionID string, callback func(string)) error {
	adapter := NewHermesAdapter(cfg.HermesAdapter)

	// Get the query from the comment
	comment, ok := context["comment"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no comment in context")
	}

	query, ok := comment["comment"].(string)
	if !ok {
		return fmt.Errorf("no comment text in context")
	}

	return adapter.QueryStream(query, context, sessionID, callback)
}

func formatHermesResponse(response string) string {
	return "🤖 **Hermes**\n\n" + response
}
