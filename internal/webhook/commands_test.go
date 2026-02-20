package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/nickalie/nclaw/internal/model"
	"github.com/nickalie/nclaw/internal/telegram"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.WebhookRegistration{}))

	send := func(_ context.Context, _ int64, _ int, _, _ string) error { return nil }
	return NewManager(database, send, "example.com", t.TempDir())
}

func TestWebhookBlockRegex(t *testing.T) {
	input := "text\n```nclaw:webhook\n{\"action\":\"create\"}\n```\nmore"
	matches := webhookBlockRe.FindAllStringSubmatch(input, -1)
	assert.Len(t, matches, 1)
	assert.Equal(t, "{\"action\":\"create\"}", matches[0][1])
}

func TestWebhookBlockRegex_Multiple(t *testing.T) {
	input := "```nclaw:webhook\n{\"action\":\"create\"}\n```\nmid\n```nclaw:webhook\n{\"action\":\"list\"}\n```"
	matches := webhookBlockRe.FindAllStringSubmatch(input, -1)
	assert.Len(t, matches, 2)
}

func TestWebhookBlockRegex_NoMatch(t *testing.T) {
	input := "just text\n```go\nfmt.Println(\"hello\")\n```"
	matches := webhookBlockRe.FindAllStringSubmatch(input, -1)
	assert.Empty(t, matches)
}

func TestProcessReply_NoBlocks(t *testing.T) {
	m := setupTestManager(t)
	result := m.ProcessReply("plain reply", 100, 0)
	assert.Equal(t, "plain reply", result)
}

func TestProcessReply_CreateWebhook(t *testing.T) {
	m := setupTestManager(t)
	reply := "Setting up.\n```nclaw:webhook\n" +
		`{"action":"create","description":"GitHub push events"}` +
		"\n```\nDone!"
	result := m.ProcessReply(reply, 100, 5)

	assert.Contains(t, result, "Setting up.")
	assert.Contains(t, result, "Done!")
	assert.NotContains(t, result, "nclaw:webhook")
	assert.Contains(t, result, "[Webhook created: https://example.com/webhooks/")

	// Verify webhook was created in DB.
	webhooks, err := m.List(100, 5)
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)
	assert.Equal(t, "GitHub push events", webhooks[0].Description)
	assert.Equal(t, model.WebhookStatusActive, webhooks[0].Status)
}

func TestProcessReply_CreateMissingDescription(t *testing.T) {
	m := setupTestManager(t)
	reply := "```nclaw:webhook\n{\"action\":\"create\"}\n```"
	result := m.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "create requires description")
}

func TestProcessReply_DeleteWebhook(t *testing.T) {
	m := setupTestManager(t)

	wh, err := m.Create("test hook", 100, 0)
	require.NoError(t, err)

	reply := "```nclaw:webhook\n" +
		`{"action":"delete","webhook_id":"` + wh.ID + `"}` +
		"\n```"
	result := m.ProcessReply(reply, 100, 0)

	assert.Contains(t, result, "[Webhook deleted: "+wh.ID+"]")
	assert.NotContains(t, result, "Webhook error")

	webhooks, err := m.List(100, 0)
	require.NoError(t, err)
	assert.Empty(t, webhooks)
}

func TestProcessReply_DeleteMissingID(t *testing.T) {
	m := setupTestManager(t)
	reply := "```nclaw:webhook\n{\"action\":\"delete\"}\n```"
	result := m.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "delete requires webhook_id")
}

func TestProcessReply_ListEmpty(t *testing.T) {
	m := setupTestManager(t)
	reply := "```nclaw:webhook\n{\"action\":\"list\"}\n```"
	result := m.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "[No webhooks registered]")
}

func TestProcessReply_ListWithWebhooks(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.Create("hook one", 100, 0)
	require.NoError(t, err)
	_, err = m.Create("hook two", 100, 0)
	require.NoError(t, err)

	reply := "```nclaw:webhook\n{\"action\":\"list\"}\n```"
	result := m.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "hook one")
	assert.Contains(t, result, "hook two")
	assert.Contains(t, result, "https://example.com/webhooks/")
}

func TestProcessReply_InvalidJSON(t *testing.T) {
	m := setupTestManager(t)
	reply := "```nclaw:webhook\n{bad json}\n```"
	result := m.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "Webhook error")
}

func TestProcessReply_UnknownAction(t *testing.T) {
	m := setupTestManager(t)
	reply := "```nclaw:webhook\n{\"action\":\"explode\"}\n```"
	result := m.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "unknown action")
}

func TestCreate(t *testing.T) {
	m := setupTestManager(t)
	wh, err := m.Create("test webhook", 200, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, wh.ID)
	assert.Equal(t, int64(200), wh.ChatID)
	assert.Equal(t, 10, wh.ThreadID)
	assert.Equal(t, "test webhook", wh.Description)
	assert.Equal(t, model.WebhookStatusActive, wh.Status)
}

func TestDelete(t *testing.T) {
	m := setupTestManager(t)
	wh, err := m.Create("to delete", 100, 0)
	require.NoError(t, err)

	err = m.Delete(wh.ID)
	require.NoError(t, err)

	webhooks, err := m.List(100, 0)
	require.NoError(t, err)
	assert.Empty(t, webhooks)
}

func TestList(t *testing.T) {
	m := setupTestManager(t)
	_, err := m.Create("one", 100, 0)
	require.NoError(t, err)
	_, err = m.Create("two", 100, 0)
	require.NoError(t, err)
	_, err = m.Create("other chat", 999, 0)
	require.NoError(t, err)

	webhooks, err := m.List(100, 0)
	require.NoError(t, err)
	assert.Len(t, webhooks, 2)
}

func TestWebhookURL(t *testing.T) {
	m := setupTestManager(t)
	url := m.WebhookURL("abc-123")
	assert.Equal(t, "https://example.com/webhooks/abc-123", url)
}

func TestHandleIncoming_NotFound(t *testing.T) {
	m := setupTestManager(t)
	err := m.HandleIncoming("nonexistent-uuid", IncomingRequest{Method: "POST"})
	assert.Error(t, err)
}

func TestHandleIncoming_Inactive(t *testing.T) {
	m := setupTestManager(t)
	wh, err := m.Create("paused hook", 100, 0)
	require.NoError(t, err)

	// Set status to paused directly.
	m.db.Model(&model.WebhookRegistration{}).Where("id = ?", wh.ID).Update("status", model.WebhookStatusPaused)

	err = m.HandleIncoming(wh.ID, IncomingRequest{Method: "POST"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrWebhookInactive)
}

func TestBuildIncomingPrompt(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "CI notifications"}
	req := IncomingRequest{
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Query:   map[string]string{"token": "abc"},
		Body:    `{"event":"push"}`,
	}

	prompt := buildIncomingPrompt(wh, req)
	assert.Contains(t, prompt, "WEBHOOK REQUEST")
	assert.Contains(t, prompt, "CI notifications")
	assert.Contains(t, prompt, "POST")
	assert.Contains(t, prompt, "Content-Type: application/json")
	assert.Contains(t, prompt, "token: abc")
	assert.Contains(t, prompt, `{"event":"push"}`)
}

func TestBuildIncomingPrompt_FiltersSensitiveHeaders(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "auth test"}
	req := IncomingRequest{
		Method: "POST",
		Headers: map[string]string{
			"Content-Type":        "application/json",
			"Authorization":       "Bearer secret-token",
			"Cookie":              "session=abc",
			"Set-Cookie":          "id=xyz",
			"Proxy-Authorization": "Basic creds",
			"X-Custom":            "safe-value",
		},
	}

	prompt := buildIncomingPrompt(wh, req)
	assert.Contains(t, prompt, "Content-Type: application/json")
	assert.Contains(t, prompt, "X-Custom: safe-value")
	assert.NotContains(t, prompt, "secret-token")
	assert.NotContains(t, prompt, "session=abc")
	assert.NotContains(t, prompt, "id=xyz")
	assert.NotContains(t, prompt, "Basic creds")
}

func TestBuildIncomingPrompt_Minimal(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "simple"}
	req := IncomingRequest{Method: "GET"}

	prompt := buildIncomingPrompt(wh, req)
	assert.Contains(t, prompt, "simple")
	assert.Contains(t, prompt, "GET")
	assert.NotContains(t, prompt, "Headers:")
	assert.NotContains(t, prompt, "Query Parameters:")
	assert.NotContains(t, prompt, "Body:")
}

func TestChatDir_NoThread(t *testing.T) {
	dir := telegram.ChatDir("/data", 123, 0)
	assert.Equal(t, "/data/123", dir)
}

func TestChatDir_WithThread(t *testing.T) {
	dir := telegram.ChatDir("/data", 123, 456)
	assert.Equal(t, "/data/123/456", dir)
}

func TestHandleIncoming_ActiveWebhook(t *testing.T) {
	m := setupTestManager(t)
	wh, err := m.Create("active hook", 100, 0)
	require.NoError(t, err)

	// HandleIncoming should succeed for active webhook (spawns goroutine but we don't wait for it).
	err = m.HandleIncoming(wh.ID, IncomingRequest{Method: "POST", Body: "test"})
	assert.NoError(t, err)
}

func TestHandleIncoming_NotFound_SentinelError(t *testing.T) {
	m := setupTestManager(t)
	err := m.HandleIncoming("nonexistent", IncomingRequest{Method: "GET"})
	assert.ErrorIs(t, err, ErrWebhookNotFound)
}

func TestSplitMessage_Short(t *testing.T) {
	chunks := telegram.SplitMessage("hello", 100)
	assert.Equal(t, []string{"hello"}, chunks)
}

func TestSplitMessage_ExactLimit(t *testing.T) {
	text := "aaaaaaaaaa" // 10 chars
	chunks := telegram.SplitMessage(text, 10)
	assert.Equal(t, []string{text}, chunks)
}

func TestSplitMessage_SplitsAtNewline(t *testing.T) {
	text := "aaaaa\nbbbbb" // 5 + newline + 5
	chunks := telegram.SplitMessage(text, 6)
	assert.Len(t, chunks, 2)
	assert.Equal(t, "aaaaa", chunks[0])
	assert.Equal(t, "bbbbb", chunks[1])
}

func TestSplitMessage_NoNewline(t *testing.T) {
	text := "aaaaaaaaaabbbbbbbbbb" // 20 chars
	chunks := telegram.SplitMessage(text, 10)
	assert.Len(t, chunks, 2)
	assert.Equal(t, "aaaaaaaaaa", chunks[0])
	assert.Equal(t, "bbbbbbbbbb", chunks[1])
}
