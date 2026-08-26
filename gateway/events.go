package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// publishEvent publishes an event to NATS
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

// getWorkItem fetches work item from Plane API
func getWorkItem(workItemID string) (PlaneWorkItem, error) {
	api := NewPlaneAPI(cfg.PlaneAPIURL, cfg.PlaneAPIKey)
	return api.GetWorkItem(workItemID)
}

// getPreviousComments fetches previous comments from database
func getPreviousComments(workItemID, excludeCommentID string) ([]PlaneComment, error) {
	return db.GetPreviousComments(workItemID, excludeCommentID)
}

// getEventStatus returns the status of an event
func getEventStatus(c *gin.Context) {
	eventID := c.Param("id")
	
	event, err := db.GetEvent(eventID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	c.JSON(http.StatusOK, event)
}
