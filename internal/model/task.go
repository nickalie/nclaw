package model

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Schedule types.
const (
	ScheduleCron     = "cron"
	ScheduleInterval = "interval"
	ScheduleOnce     = "once"
)

// Task statuses.
const (
	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Context modes.
const (
	ContextGroup    = "group"
	ContextIsolated = "isolated"
)

// ScheduledTask represents a scheduled task persisted in the database.
type ScheduledTask struct {
	ID            string     `gorm:"primaryKey" json:"id"`
	ChatID        int64      `gorm:"not null;index" json:"chat_id"`
	ThreadID      int        `gorm:"not null;default:0" json:"thread_id"`
	Prompt        string     `gorm:"not null" json:"prompt"`
	ScheduleType  string     `gorm:"not null" json:"schedule_type"`
	ScheduleValue string     `gorm:"not null" json:"schedule_value"`
	ContextMode   string     `gorm:"not null;default:group" json:"context_mode"`
	Status        string     `gorm:"not null;default:active;index" json:"status"`
	NextRun       *time.Time `gorm:"index" json:"next_run"`
	LastRun       *time.Time `json:"last_run"`
	LastResult    *string    `json:"last_result"`
	CreatedAt     time.Time  `json:"created_at"`
}

// TaskRunLog records the result of a single task execution.
type TaskRunLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TaskID     string    `gorm:"not null;index" json:"task_id"`
	RunAt      time.Time `gorm:"not null" json:"run_at"`
	DurationMs int64     `gorm:"not null" json:"duration_ms"`
	Status     string    `gorm:"not null" json:"status"`
	Result     *string   `json:"result"`
	Error      *string   `json:"error"`
}

// GenerateTaskID creates a unique task ID.
func GenerateTaskID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("task-%d-%x", time.Now().UnixMilli(), b)
}
