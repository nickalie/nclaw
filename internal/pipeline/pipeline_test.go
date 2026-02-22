package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nickalie/nclaw/internal/claude"
)

// mockExecutor implements BlockExecutor for testing.
type mockExecutor struct {
	called   bool
	lastText string
	msg      string // status message to return
}

func (m *mockExecutor) ExecuteBlocks(text string, _ int64, _ int) string {
	m.called = true
	m.lastText = text
	return m.msg
}

// mockSend records all sent messages.
type mockSend struct {
	calls []sendCall
	err   error // if set, returned for all calls
}

type sendCall struct {
	chatID    int64
	threadID  int
	text      string
	parseMode string
}

func (m *mockSend) fn() SendFunc {
	return func(_ context.Context, chatID int64, threadID int, text, parseMode string) error {
		m.calls = append(m.calls, sendCall{chatID, threadID, text, parseMode})
		return m.err
	}
}

// mockSendDoc records sendfile calls.
type mockSendDoc struct {
	called bool
}

func (m *mockSendDoc) fn() func(context.Context, int64, int, string, []byte, string) error {
	return func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
		m.called = true
		return nil
	}
}

func TestProcess_SuccessPath(t *testing.T) {
	exec := &mockExecutor{}
	ms := &mockSend{}
	p := New(ms.fn(), nil, true, exec)

	result := &claude.Result{
		Text:     "Hello world",
		FullText: "Hello world",
	}
	p.Process(context.Background(), result, nil, 100, 0, "/tmp")

	assert.True(t, exec.called)
	assert.Equal(t, "Hello world", exec.lastText)
	require.Len(t, ms.calls, 1)
	assert.Equal(t, int64(100), ms.calls[0].chatID)
	assert.Equal(t, "Hello world", ms.calls[0].text)
	assert.Equal(t, "HTML", ms.calls[0].parseMode)
}

func TestProcess_ErrorPath_SkipsExecution(t *testing.T) {
	exec := &mockExecutor{}
	ms := &mockSend{}
	p := New(ms.fn(), nil, true, exec)

	result := &claude.Result{
		Text:     "error: something went wrong",
		FullText: "error: something went wrong",
	}
	p.Process(context.Background(), result, errors.New("claude failed"), 100, 0, "/tmp")

	assert.False(t, exec.called, "executors should not run on error")
	require.Len(t, ms.calls, 1)
	assert.Equal(t, "error: something went wrong", ms.calls[0].text)
}

func TestProcess_NilWebhookExecutor_Filtered(t *testing.T) {
	exec := &mockExecutor{msg: "scheduled ok"}
	ms := &mockSend{}
	// Pass a nil executor alongside a real one — nil should be filtered.
	p := New(ms.fn(), nil, true, exec, nil)

	result := &claude.Result{Text: "reply", FullText: "reply"}
	p.Process(context.Background(), result, nil, 100, 0, "/tmp")

	assert.True(t, exec.called)
	assert.Len(t, p.executors, 1, "nil executors should be filtered out")
}

func TestProcess_StatusAppending(t *testing.T) {
	exec1 := &mockExecutor{msg: "[Schedule error: oops]"}
	exec2 := &mockExecutor{msg: "[Webhook created: https://example.com/webhooks/abc]"}
	ms := &mockSend{}
	p := New(ms.fn(), nil, true, exec1, exec2)

	result := &claude.Result{Text: "Done", FullText: "Done"}
	p.Process(context.Background(), result, nil, 100, 0, "/tmp")

	require.Len(t, ms.calls, 1)
	assert.Contains(t, ms.calls[0].text, "Done")
	assert.Contains(t, ms.calls[0].text, "[Schedule error: oops]")
	assert.Contains(t, ms.calls[0].text, "[Webhook created: https://example.com/webhooks/abc]")
}

func TestProcess_EmptyText_NoSend(t *testing.T) {
	ms := &mockSend{}
	p := New(ms.fn(), nil, true)

	result := &claude.Result{Text: "", FullText: ""}
	p.Process(context.Background(), result, nil, 100, 0, "/tmp")

	assert.Empty(t, ms.calls, "should not send empty text")
}

func TestProcess_StripsAllBlockTypes(t *testing.T) {
	ms := &mockSend{}
	p := New(ms.fn(), nil, true)

	text := "Hello\n" +
		"```nclaw:sendfile\n{\"path\":\"test.txt\"}\n```\n" +
		"```nclaw:schedule\n{\"action\":\"create\"}\n```\n" +
		"```nclaw:webhook\n{\"action\":\"list\"}\n```\n" +
		"Goodbye"

	result := &claude.Result{Text: text, FullText: text}
	p.Process(context.Background(), result, nil, 100, 0, "/tmp")

	require.Len(t, ms.calls, 1)
	assert.NotContains(t, ms.calls[0].text, "nclaw:sendfile")
	assert.NotContains(t, ms.calls[0].text, "nclaw:schedule")
	assert.NotContains(t, ms.calls[0].text, "nclaw:webhook")
	assert.Contains(t, ms.calls[0].text, "Hello")
	assert.Contains(t, ms.calls[0].text, "Goodbye")
}

func TestProcess_HTMLFallbackToPlainText(t *testing.T) {
	callCount := 0
	sendFn := func(_ context.Context, chatID int64, threadID int, text, parseMode string) error {
		callCount++
		if parseMode == "HTML" {
			return fmt.Errorf("HTML parse error")
		}
		return nil
	}
	p := New(sendFn, nil, true)

	result := &claude.Result{Text: "Hello", FullText: "Hello"}
	p.Process(context.Background(), result, nil, 100, 0, "/tmp")

	assert.Equal(t, 2, callCount, "should try HTML then plain text")
}

func TestProcess_MultipleExecutors(t *testing.T) {
	exec1 := &mockExecutor{}
	exec2 := &mockExecutor{}
	ms := &mockSend{}
	p := New(ms.fn(), nil, true, exec1, exec2)

	result := &claude.Result{Text: "reply", FullText: "full reply"}
	p.Process(context.Background(), result, nil, 100, 5, "/tmp")

	assert.True(t, exec1.called)
	assert.True(t, exec2.called)
	assert.Equal(t, "full reply", exec1.lastText)
	assert.Equal(t, "full reply", exec2.lastText)
}

func TestNew_NilExecutorsFiltered(t *testing.T) {
	p := New(nil, nil, true, nil, nil)
	assert.Empty(t, p.executors)
}

func TestStripAllBlocks(t *testing.T) {
	text := "before\n```nclaw:schedule\n{}\n```\nmiddle\n```nclaw:webhook\n{}\n```\nafter"
	result := stripAllBlocks(text)
	assert.Equal(t, "before\n\nmiddle\n\nafter", result)
}

func TestAppendStatus_Empty(t *testing.T) {
	assert.Equal(t, "hello", appendStatus("hello", nil))
	assert.Equal(t, "hello", appendStatus("hello", []string{}))
}

func TestAppendStatus_WithMessages(t *testing.T) {
	result := appendStatus("text", []string{"msg1", "msg2"})
	assert.Equal(t, "text\n\nmsg1\n\nmsg2", result)
}

func TestAppendStatus_EmptyBase(t *testing.T) {
	result := appendStatus("", []string{"msg"})
	assert.Equal(t, "msg", result)
}

func TestProcess_WebhooksNotConfigured_WarningAppended(t *testing.T) {
	ms := &mockSend{}
	p := New(ms.fn(), nil, false) // webhooksConfigured=false

	text := "Here you go.\n```nclaw:webhook\n{\"action\":\"create\",\"description\":\"test\"}\n```\nDone!"
	result := &claude.Result{Text: text, FullText: text}
	p.Process(context.Background(), result, nil, 100, 0, "/tmp")

	require.Len(t, ms.calls, 1)
	assert.Contains(t, ms.calls[0].text, "Here you go.")
	assert.Contains(t, ms.calls[0].text, "Done!")
	assert.NotContains(t, ms.calls[0].text, "nclaw:webhook")
	assert.Contains(t, ms.calls[0].text, "[Webhooks are not configured on this instance]")
}

func TestProcess_WebhooksConfigured_NoWarning(t *testing.T) {
	exec := &mockExecutor{}
	ms := &mockSend{}
	p := New(ms.fn(), nil, true, exec) // webhooksConfigured=true

	text := "Done.\n```nclaw:webhook\n{\"action\":\"create\"}\n```"
	result := &claude.Result{Text: text, FullText: text}
	p.Process(context.Background(), result, nil, 100, 0, "/tmp")

	require.Len(t, ms.calls, 1)
	assert.NotContains(t, ms.calls[0].text, "not configured")
}
