package model

import (
	"time"

	"github.com/google/uuid"
)

// Webhook statuses.
const (
	WebhookStatusActive = "active"
	WebhookStatusPaused = "paused"
)

// WebhookRegistration represents a registered webhook persisted in the database.
type WebhookRegistration struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	ChatID      int64     `gorm:"not null;index" json:"chat_id"`
	ThreadID    int       `gorm:"not null;default:0" json:"thread_id"`
	Description string    `gorm:"not null" json:"description"`
	Status      string    `gorm:"not null;default:active;index" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GenerateWebhookID creates a new UUID for a webhook.
func GenerateWebhookID() string {
	return uuid.New().String()
}
