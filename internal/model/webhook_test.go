package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookConstants(t *testing.T) {
	assert.Equal(t, "active", WebhookStatusActive)
	assert.Equal(t, "paused", WebhookStatusPaused)
}

func TestGenerateWebhookID_Format(t *testing.T) {
	id := GenerateWebhookID()
	assert.NotEmpty(t, id)
	// UUID v4 format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	assert.Len(t, id, 36)
	assert.Equal(t, byte('-'), id[8])
	assert.Equal(t, byte('-'), id[13])
	assert.Equal(t, byte('-'), id[18])
	assert.Equal(t, byte('-'), id[23])
}

func TestGenerateWebhookID_Unique(t *testing.T) {
	ids := make(map[string]struct{}, 100)
	for range 100 {
		id := GenerateWebhookID()
		_, exists := ids[id]
		assert.False(t, exists, "generated duplicate ID: %s", id)
		ids[id] = struct{}{}
	}
}
