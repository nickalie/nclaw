package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nickalie/nclaw/internal/model"
)

func sampleWebhook(chatID int64, threadID int) *model.WebhookRegistration {
	return &model.WebhookRegistration{
		ID:          model.GenerateWebhookID(),
		ChatID:      chatID,
		ThreadID:    threadID,
		Description: "test webhook",
		Status:      model.WebhookStatusActive,
	}
}

func TestCreateAndGetWebhook(t *testing.T) {
	database := setupTestDB(t)
	wh := sampleWebhook(123, 0)

	require.NoError(t, CreateWebhook(database, wh))

	got, err := GetWebhookByID(database, wh.ID)
	require.NoError(t, err)
	assert.Equal(t, wh.ID, got.ID)
	assert.Equal(t, wh.ChatID, got.ChatID)
	assert.Equal(t, wh.Description, got.Description)
	assert.Equal(t, wh.Status, got.Status)
}

func TestGetWebhookByID_NotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetWebhookByID(database, "nonexistent")
	assert.Error(t, err)
}

func TestListWebhooksByChat(t *testing.T) {
	database := setupTestDB(t)

	wh1 := sampleWebhook(100, 0)
	wh2 := sampleWebhook(100, 0)
	wh3 := sampleWebhook(200, 0) // different chat
	wh4 := sampleWebhook(100, 5) // different thread

	for _, wh := range []*model.WebhookRegistration{wh1, wh2, wh3, wh4} {
		require.NoError(t, CreateWebhook(database, wh))
	}

	webhooks, err := ListWebhooksByChat(database, 100, 0)
	require.NoError(t, err)
	assert.Len(t, webhooks, 2)

	webhooks, err = ListWebhooksByChat(database, 200, 0)
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)

	webhooks, err = ListWebhooksByChat(database, 100, 5)
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)

	webhooks, err = ListWebhooksByChat(database, 999, 0)
	require.NoError(t, err)
	assert.Empty(t, webhooks)
}

func TestDeleteWebhook(t *testing.T) {
	database := setupTestDB(t)
	wh := sampleWebhook(100, 0)
	require.NoError(t, CreateWebhook(database, wh))

	require.NoError(t, DeleteWebhook(database, wh.ID))

	_, err := GetWebhookByID(database, wh.ID)
	assert.Error(t, err)
}

func TestUpdateWebhookStatus(t *testing.T) {
	database := setupTestDB(t)
	wh := sampleWebhook(100, 0)
	require.NoError(t, CreateWebhook(database, wh))

	require.NoError(t, UpdateWebhookStatus(database, wh.ID, model.WebhookStatusPaused))

	got, err := GetWebhookByID(database, wh.ID)
	require.NoError(t, err)
	assert.Equal(t, model.WebhookStatusPaused, got.Status)
}
