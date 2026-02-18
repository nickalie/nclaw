package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTaskID_Format(t *testing.T) {
	id := GenerateTaskID()
	assert.True(t, strings.HasPrefix(id, "task-"), "should start with 'task-'")
	parts := strings.SplitN(id, "-", 3)
	require.Len(t, parts, 3, "should have format task-<timestamp>-<hex>")
	assert.NotEmpty(t, parts[1], "timestamp part should not be empty")
	assert.NotEmpty(t, parts[2], "hex part should not be empty")
}

func TestGenerateTaskID_Unique(t *testing.T) {
	ids := make(map[string]struct{}, 100)
	for range 100 {
		id := GenerateTaskID()
		_, exists := ids[id]
		assert.False(t, exists, "generated duplicate ID: %s", id)
		ids[id] = struct{}{}
	}
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "cron", ScheduleCron)
	assert.Equal(t, "interval", ScheduleInterval)
	assert.Equal(t, "once", ScheduleOnce)

	assert.Equal(t, "active", StatusActive)
	assert.Equal(t, "paused", StatusPaused)
	assert.Equal(t, "completed", StatusCompleted)
	assert.Equal(t, "failed", StatusFailed)

	assert.Equal(t, "group", ContextGroup)
	assert.Equal(t, "isolated", ContextIsolated)
}
