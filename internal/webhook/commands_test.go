package webhook

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/nickalie/nclaw/internal/cli"
	"github.com/nickalie/nclaw/internal/model"
	"github.com/nickalie/nclaw/internal/pipeline"
	"github.com/nickalie/nclaw/internal/sendfile"
	"github.com/nickalie/nclaw/internal/telegram"
)

// mockProvider implements cli.Provider for testing.
type mockProvider struct{}

func (m *mockProvider) NewClient() cli.Client    { return &mockClient{} }
func (m *mockProvider) PreInvoke() error         { return nil }
func (m *mockProvider) Version() (string, error) { return "mock-1.0.0", nil }
func (m *mockProvider) Name() string             { return "mock" }

// mockClient implements cli.Client for testing.
type mockClient struct{}

func (m *mockClient) Dir(string) cli.Client                { return m }
func (m *mockClient) SkipPermissions() cli.Client          { return m }
func (m *mockClient) AppendSystemPrompt(string) cli.Client { return m }
func (m *mockClient) Ask(string) (*cli.Result, error) {
	return &cli.Result{Text: "mock response", FullText: "mock response"}, nil
}
func (m *mockClient) Continue(string) (*cli.Result, error) {
	return &cli.Result{Text: "mock response", FullText: "mock response"}, nil
}

func noopSend(_ context.Context, _ int64, _ int, _, _ string) error { return nil }

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.WebhookRegistration{}))

	mgr := NewManager(database, &mockProvider{}, "example.com", t.TempDir(), telegram.NewChatLocker())
	mgr.SetPipeline(pipeline.New(noopSend, sendfile.Senders{}, true))
	return mgr
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

func TestExecuteBlocks_CreateWebhookFull(t *testing.T) {
	m := setupTestManager(t)
	text := "Setting up.\n```nclaw:webhook\n" +
		`{"action":"create","description":"GitHub push events"}` +
		"\n```\nDone!"
	statusMsg := m.ExecuteBlocks(text, 100, 5)
	assert.Contains(t, statusMsg, "[Webhook created: https://example.com/webhooks/")

	display := strings.TrimSpace(webhookBlockRe.ReplaceAllString(text, ""))
	assert.Contains(t, display, "Setting up.")
	assert.Contains(t, display, "Done!")
	assert.NotContains(t, display, "nclaw:webhook")

	// Verify webhook was created in DB.
	webhooks, err := m.List(100, 5)
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)
	assert.Equal(t, "GitHub push events", webhooks[0].Description)
	assert.Equal(t, model.WebhookStatusActive, webhooks[0].Status)
}

func TestExecuteBlocks_CreateMissingDescription(t *testing.T) {
	m := setupTestManager(t)
	text := "```nclaw:webhook\n{\"action\":\"create\"}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "create requires description")
}

func TestExecuteBlocks_DeleteWebhook(t *testing.T) {
	m := setupTestManager(t)

	wh, err := m.Create("test hook", 100, 0)
	require.NoError(t, err)

	text := "```nclaw:webhook\n" +
		`{"action":"delete","webhook_id":"` + wh.ID + `"}` +
		"\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "[Webhook deleted: "+wh.ID+"]")
	assert.NotContains(t, result, "Webhook error")

	webhooks, err := m.List(100, 0)
	require.NoError(t, err)
	assert.Empty(t, webhooks)
}

func TestExecuteBlocks_DeleteMissingID(t *testing.T) {
	m := setupTestManager(t)
	text := "```nclaw:webhook\n{\"action\":\"delete\"}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "delete requires webhook_id")
}

func TestExecuteBlocks_ListEmpty(t *testing.T) {
	m := setupTestManager(t)
	text := "```nclaw:webhook\n{\"action\":\"list\"}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "[No webhooks registered]")
}

func TestExecuteBlocks_ListWithWebhooks(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.Create("hook one", 100, 0)
	require.NoError(t, err)
	_, err = m.Create("hook two", 100, 0)
	require.NoError(t, err)

	text := "```nclaw:webhook\n{\"action\":\"list\"}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "hook one")
	assert.Contains(t, result, "hook two")
	assert.Contains(t, result, "https://example.com/webhooks/")
}

func TestExecuteBlocks_InvalidJSON(t *testing.T) {
	m := setupTestManager(t)
	text := "```nclaw:webhook\n{bad json}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
}

func TestExecuteBlocks_UnknownAction(t *testing.T) {
	m := setupTestManager(t)
	text := "```nclaw:webhook\n{\"action\":\"explode\"}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "unknown action")
}

func TestExecuteBlocks_NoBlocks(t *testing.T) {
	m := setupTestManager(t)
	result := m.ExecuteBlocks("plain text", 100, 0)
	assert.Empty(t, result)
}

func TestExecuteBlocks_CreateWebhook(t *testing.T) {
	m := setupTestManager(t)
	text := "text\n```nclaw:webhook\n" +
		`{"action":"create","description":"test hook"}` +
		"\n```\nmore"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "[Webhook created: https://example.com/webhooks/")

	webhooks, err := m.List(100, 0)
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)
}

func TestExecuteBlocks_Error(t *testing.T) {
	m := setupTestManager(t)
	text := "```nclaw:webhook\n{bad json}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "[Webhook error:")
}

func TestExecuteBlocks_MixedSuccessAndError(t *testing.T) {
	m := setupTestManager(t)
	text := "```nclaw:webhook\n" +
		`{"action":"create","description":"ok"}` +
		"\n```\n```nclaw:webhook\n{bad}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "[Webhook created:")
	assert.Contains(t, result, "[Webhook error:")
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

func TestHandleIncoming_NonWhitelistedChat(t *testing.T) {
	viper.Set("telegram.whitelist_chat_ids", "200")
	defer viper.Reset()
	m := setupTestManager(t)
	wh, err := m.Create("stale hook", 100, 0)
	require.NoError(t, err)

	err = m.HandleIncoming(wh.ID, IncomingRequest{Method: "POST"})
	assert.ErrorIs(t, err, ErrWebhookNotFound)
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

func TestBuildIncomingPrompt_FiltersPatternHeaders(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "pattern test"}
	req := IncomingRequest{
		Method: "POST",
		Headers: map[string]string{
			"Content-Type":      "application/json",
			"Stripe-Signature":  "t=123,v1=abc",
			"X-Slack-Signature": "v0=hash",
			"X-Gitlab-Token":    "glpat-xxx",
			"X-Request-Id":      "req-123",
		},
	}

	prompt := buildIncomingPrompt(wh, req)
	assert.Contains(t, prompt, "Content-Type: application/json")
	assert.Contains(t, prompt, "X-Request-Id: req-123")
	assert.NotContains(t, prompt, "t=123,v1=abc")
	assert.NotContains(t, prompt, "v0=hash")
	assert.NotContains(t, prompt, "glpat-xxx")
}

func TestIsSensitiveHeader(t *testing.T) {
	sensitive := []string{
		"Authorization", "Cookie", "Set-Cookie", "Proxy-Authorization",
		"X-Api-Key", "X-Api-Token", "X-Auth-Token", "X-Secret",
		"X-Hub-Signature", "X-Hub-Signature-256", "X-Webhook-Secret",
		"Stripe-Signature", "X-Slack-Signature", "X-Gitlab-Token",
	}
	for _, h := range sensitive {
		assert.True(t, isSensitiveHeader(h), "expected %q to be sensitive", h)
	}

	safe := []string{"Content-Type", "Accept", "X-Request-Id", "User-Agent", "Host"}
	for _, h := range safe {
		assert.False(t, isSensitiveHeader(h), "expected %q to be safe", h)
	}
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
	defer m.Wait()
	wh, err := m.Create("active hook", 100, 0)
	require.NoError(t, err)

	err = m.HandleIncoming(wh.ID, IncomingRequest{Method: "POST", Body: "test"})
	assert.NoError(t, err)
}

func TestHandleIncoming_NotFound_SentinelError(t *testing.T) {
	m := setupTestManager(t)
	err := m.HandleIncoming("nonexistent", IncomingRequest{Method: "GET"})
	assert.ErrorIs(t, err, ErrWebhookNotFound)
}

func TestHandleIncoming_Busy(t *testing.T) {
	m := setupTestManager(t)
	wh, err := m.Create("busy hook", 100, 0)
	require.NoError(t, err)

	// Fill the semaphore to capacity.
	for range maxConcurrentWebhooks {
		m.sem <- struct{}{}
	}

	err = m.HandleIncoming(wh.ID, IncomingRequest{Method: "POST"})
	assert.ErrorIs(t, err, ErrWebhookBusy)

	// Drain semaphore.
	for range maxConcurrentWebhooks {
		<-m.sem
	}
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

func TestExecuteBlocks_PauseTask(t *testing.T) {
	m := setupTestManager(t)

	// Create a webhook, then try to pause it via a schedule pause command.
	// The webhook manager only handles webhook actions; pause is not a supported action.
	// So we test that pause on the webhook manager returns an unknown action error.
	text := "```nclaw:webhook\n" +
		`{"action":"pause","webhook_id":"some-id"}` +
		"\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "unknown action")
}

func TestExecuteBlocks_ResumeTask(t *testing.T) {
	m := setupTestManager(t)

	// Resume is not a supported webhook action.
	text := "```nclaw:webhook\n" +
		`{"action":"resume","webhook_id":"some-id"}` +
		"\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "unknown action")
}

func TestExecuteBlocks_CancelTask(t *testing.T) {
	m := setupTestManager(t)

	// Cancel is not a supported webhook action.
	text := "```nclaw:webhook\n" +
		`{"action":"cancel","webhook_id":"some-id"}` +
		"\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "unknown action")
}

func TestExecuteBlocks_DeleteWebhook_NotFound(t *testing.T) {
	m := setupTestManager(t)

	text := "```nclaw:webhook\n" +
		`{"action":"delete","webhook_id":"nonexistent-id"}` +
		"\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "webhook not found")
}

func TestExecuteBlocks_DeleteWebhook_WrongChat(t *testing.T) {
	m := setupTestManager(t)

	// Create webhook in chat 200.
	wh, err := m.Create("cross-chat hook", 200, 0)
	require.NoError(t, err)

	// Try to delete from chat 100 — should fail.
	text := "```nclaw:webhook\n" +
		`{"action":"delete","webhook_id":"` + wh.ID + `"}` +
		"\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "Webhook error")
	assert.Contains(t, result, "webhook not found")

	// Verify it still exists.
	webhooks, err := m.List(200, 0)
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)
}

func TestExecuteBlocks_ListWebhooks_MultipleChats(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.Create("hook A", 100, 0)
	require.NoError(t, err)
	_, err = m.Create("hook B", 100, 0)
	require.NoError(t, err)
	_, err = m.Create("hook C", 200, 0)
	require.NoError(t, err)

	text := "```nclaw:webhook\n{\"action\":\"list\"}\n```"
	result := m.ExecuteBlocks(text, 100, 0)
	assert.Contains(t, result, "hook A")
	assert.Contains(t, result, "hook B")
	assert.NotContains(t, result, "hook C")
}

func TestBuildIncomingPrompt_Full(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "full test"}
	req := IncomingRequest{
		Method:  "PUT",
		Headers: map[string]string{"Content-Type": "text/plain", "Accept": "application/json"},
		Query:   map[string]string{"page": "1", "limit": "10"},
		Body:    "request body content",
	}

	prompt := buildIncomingPrompt(wh, req)
	assert.Contains(t, prompt, "WEBHOOK REQUEST")
	assert.Contains(t, prompt, "full test")
	assert.Contains(t, prompt, "PUT")
	assert.Contains(t, prompt, "Headers:")
	assert.Contains(t, prompt, "Content-Type: text/plain")
	assert.Contains(t, prompt, "Accept: application/json")
	assert.Contains(t, prompt, "Query Parameters:")
	assert.Contains(t, prompt, "page: 1")
	assert.Contains(t, prompt, "limit: 10")
	assert.Contains(t, prompt, "Body:")
	assert.Contains(t, prompt, "request body content")
}

func TestBuildIncomingPrompt_MethodOnly(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "minimal"}
	req := IncomingRequest{Method: "DELETE"}

	prompt := buildIncomingPrompt(wh, req)
	assert.Contains(t, prompt, "minimal")
	assert.Contains(t, prompt, "DELETE")
	assert.NotContains(t, prompt, "Headers:")
	assert.NotContains(t, prompt, "Query Parameters:")
	assert.NotContains(t, prompt, "Body:")
}

func TestBuildIncomingPrompt_SensitiveHeadersFiltered(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "sensitive test"}
	req := IncomingRequest{
		Method: "POST",
		Headers: map[string]string{
			"Content-Type":        "application/json",
			"Authorization":       "Bearer super-secret",
			"Cookie":              "session=private",
			"Set-Cookie":          "id=hidden",
			"Proxy-Authorization": "Basic creds",
			"X-Api-Key":           "key-123",
			"X-Hub-Signature-256": "sha256=abc",
			"X-Custom-Header":     "visible",
		},
	}

	prompt := buildIncomingPrompt(wh, req)
	// Safe headers should be present.
	assert.Contains(t, prompt, "Content-Type: application/json")
	assert.Contains(t, prompt, "X-Custom-Header: visible")
	// Sensitive headers should be filtered out.
	assert.NotContains(t, prompt, "super-secret")
	assert.NotContains(t, prompt, "session=private")
	assert.NotContains(t, prompt, "id=hidden")
	assert.NotContains(t, prompt, "Basic creds")
	assert.NotContains(t, prompt, "key-123")
	assert.NotContains(t, prompt, "sha256=abc")
}

func TestBuildIncomingPrompt_EmptyHeaders(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "empty headers"}
	req := IncomingRequest{
		Method:  "GET",
		Headers: map[string]string{},
	}

	prompt := buildIncomingPrompt(wh, req)
	assert.NotContains(t, prompt, "Headers:")
}

func TestBuildIncomingPrompt_EmptyQuery(t *testing.T) {
	wh := &model.WebhookRegistration{Description: "empty query"}
	req := IncomingRequest{
		Method: "GET",
		Query:  map[string]string{},
	}

	prompt := buildIncomingPrompt(wh, req)
	assert.NotContains(t, prompt, "Query Parameters:")
}
