package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
)

type Config struct {
	Port           int    `env:"PORT,default=8080"`
	NatsURL        string `env:"NATS_URL,default=nats://nats:4222"`
	PlaneSecret    string `env:"PLANE_WEBHOOK_SECRET,required"`
	DatabaseURL    string `env:"DATABASE_URL,required"`
	EventStream    string `env:"EVENT_STREAM,default=clep.events"`
}

type Event struct {
	ID      string          `json:"id"`
	Type    string          `json:"event_type"`
	Source  string          `json:"source"`
	Payload json.RawMessage `json:"payload"`
	Status  string          `json:"status"`
}

var (
	cfg    Config
	nc     *nats.Conn
	db     *sql.DB
	logger *log.Logger
)

func main() {
	logger = log.New(os.Stdout, "[CLEP] ", log.LstdFlags|log.Lmicroseconds)
	logger.Println("Starting CLEP Gateway...")

	cfg = loadConfig()
	logger.Printf("Config: port=%d, nats=%s, db=%s", cfg.Port, cfg.NatsURL, "postgres://***:***@...")

	// Connect to NATS with timeout
	logger.Println("Connecting to NATS...")
	var natsErr error
	nc, natsErr = nats.Connect(cfg.NatsURL,
		nats.Timeout(3*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if natsErr != nil {
		logger.Printf("WARNING: NATS connection failed: %v (will retry)", natsErr)
	} else {
		logger.Println("NATS connected")
		defer nc.Close()
	}

	// Connect to Database with timeout
	logger.Println("Connecting to PostgreSQL...")
	var dbErr error
	db, dbErr = sql.Open("postgres", cfg.DatabaseURL)
	if dbErr != nil {
		logger.Printf("ERROR: Failed to open database: %v", dbErr)
		os.Exit(1)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Ping with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if dbErr = db.PingContext(ctx); dbErr != nil {
		logger.Printf("ERROR: Database ping failed: %v", dbErr)
		os.Exit(1)
	}
	logger.Println("Database connected")

	// Initialize schema
	logger.Println("Initializing schema...")
	if err := initSchema(); err != nil {
		logger.Printf("ERROR: Schema init failed: %v", err)
		os.Exit(1)
	}
	logger.Println("Schema initialized")

	// Setup HTTP server
	logger.Println("Setting up HTTP server...")
	r := gin.New()
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[CLEP] %s | %s | %d | %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.ClientIP,
			param.StatusCode,
			param.Latency,
		)
	}))
	r.Use(gin.Recovery())

	r.GET("/health", healthHandler)
	r.POST("/webhooks/plane", handleWebhook)
	r.GET("/events/:id", getEvent)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Printf("Starting HTTP server on %s", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("ERROR: HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("Shutting down...")

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("ERROR: Server forced shutdown: %v", err)
	}
	logger.Println("Server stopped")
}

func loadConfig() Config {
	return Config{
		Port:          getEnvInt("PORT", 8080),
		NatsURL:       getEnv("NATS_URL", "nats://nats:4222"),
		PlaneSecret:   getEnv("PLANE_WEBHOOK_SECRET", ""),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		EventStream:   getEnv("EVENT_STREAM", "clep.events"),
	}
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"nats":      statusText(nc != nil),
		"db":        statusText(db != nil),
	})
}

func statusText(ok bool) string {
	if ok {
		return "connected"
	}
	return "disconnected"
}

func handleWebhook(c *gin.Context) {
	logger.Println("Received webhook")

	sig := c.GetHeader("X-Plane-Signature")
	body, err := c.GetRawData()
	if err != nil {
		logger.Printf("ERROR: Failed to read body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	if !verifySig(body, sig) {
		logger.Println("ERROR: Invalid signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}
	logger.Println("Signature valid")

	var wh map[string]interface{}
	if err := json.Unmarshal(body, &wh); err != nil {
		logger.Printf("ERROR: Invalid JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	eventType, _ := wh["event"].(string)
	if eventType != "issue.comment.create" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	resource, _ := wh["resource"].(map[string]interface{})
	comment, _ := resource["comment"].(string)

	if len(comment) < 7 || comment[:7] != "@hermes" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	event := Event{
		ID:      fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:    "plane.comment.created",
		Source:  "plane",
		Payload: body,
		Status:  "created",
	}

	if err := storeEvent(event); err != nil {
		logger.Printf("ERROR: Failed to store event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store event"})
		return
	}
	logger.Printf("Event stored: %s", event.ID)

	if nc != nil {
		if err := nc.Publish(cfg.EventStream, body); err != nil {
			logger.Printf("WARNING: NATS publish failed: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "accepted", "event_id": event.ID})
}

func verifySig(body []byte, sig string) bool {
	mac := hmac.New(sha256.New, []byte(cfg.PlaneSecret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), []byte(sig))
}

func storeEvent(event Event) error {
	_, err := db.Exec(
		"INSERT INTO events (id, event_type, source, payload, status) VALUES ($1, $2, $3, $4, $5)",
		event.ID, event.Type, event.Source, event.Payload, event.Status,
	)
	return err
}

func getEvent(c *gin.Context) {
	var event Event
	err := db.QueryRow(
		"SELECT id, event_type, source, payload, status FROM events WHERE id = $1",
		c.Param("id"),
	).Scan(&event.ID, &event.Type, &event.Source, &event.Payload, &event.Status)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}
	c.JSON(http.StatusOK, event)
}

func initSchema() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL,
			source TEXT NOT NULL,
			payload JSONB NOT NULL,
			status TEXT NOT NULL DEFAULT 'created',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func getEnv(k, d string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return d
}

func getEnvInt(k string, d int) int {
	if v, ok := os.LookupEnv(k); ok {
		var n int
		fmt.Sscanf(v, "%d", &n)
		return n
	}
	return d
}
