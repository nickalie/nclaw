package handler

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/nickalie/nclaw/internal/config"
	"github.com/nickalie/nclaw/internal/model"
	"github.com/nickalie/nclaw/internal/telegram"
	"github.com/nickalie/nclaw/internal/webhook"
)

func TestWithReplyContext_NoReply(t *testing.T) {
	msg := &models.Message{Text: "hello"}
	result := withReplyContext(msg, "hello")
	assert.Equal(t, "hello", result)
}

func TestWithReplyContext_WithReplyText(t *testing.T) {
	msg := &models.Message{
		Text:           "my reply",
		ReplyToMessage: &models.Message{Text: "original"},
	}
	result := withReplyContext(msg, "my reply")
	assert.Equal(t, "[Replying to message: original]\n\nmy reply", result)
}

func TestWithReplyContext_WithReplyCaption(t *testing.T) {
	msg := &models.Message{
		Text:           "my reply",
		ReplyToMessage: &models.Message{Caption: "photo caption"},
	}
	result := withReplyContext(msg, "my reply")
	assert.Equal(t, "[Replying to message: photo caption]\n\nmy reply", result)
}

func TestWithReplyContext_EmptyOriginal(t *testing.T) {
	msg := &models.Message{
		Text:           "my reply",
		ReplyToMessage: &models.Message{},
	}
	result := withReplyContext(msg, "my reply")
	assert.Equal(t, "my reply", result)
}

func TestMessageContent_TextOnly(t *testing.T) {
	msg := &models.Message{Text: "hello"}
	text, att := messageContent(msg)
	assert.Equal(t, "hello", text)
	assert.Nil(t, att)
}

func TestMessageContent_DocumentWithCaption(t *testing.T) {
	msg := &models.Message{
		Caption:  "file caption",
		Document: &models.Document{FileID: "f1", FileName: "test.pdf"},
	}
	text, att := messageContent(msg)
	assert.Equal(t, "file caption", text)
	assert.NotNil(t, att)
	assert.Equal(t, "f1", att.fileID)
	assert.Equal(t, "test.pdf", att.filename)
}

func TestMessageContent_PhotoNoCaption(t *testing.T) {
	msg := &models.Message{
		Photo: []models.PhotoSize{
			{FileID: "small", Width: 100, Height: 100},
			{FileID: "large", Width: 800, Height: 800},
		},
	}
	text, att := messageContent(msg)
	assert.Empty(t, text)
	assert.NotNil(t, att)
	assert.Equal(t, "large", att.fileID)
	assert.Equal(t, "photo.jpg", att.filename)
}

func TestExtractAttachment_Document(t *testing.T) {
	msg := &models.Message{
		Document: &models.Document{FileID: "doc1", FileName: "report.pdf"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "doc1", att.fileID)
	assert.Equal(t, "report.pdf", att.filename)
}

func TestExtractAttachment_Audio(t *testing.T) {
	msg := &models.Message{
		Audio: &models.Audio{FileID: "a1", FileName: "song.mp3"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "a1", att.fileID)
	assert.Equal(t, "song.mp3", att.filename)
}

func TestExtractAttachment_AudioFallbackName(t *testing.T) {
	msg := &models.Message{
		Audio: &models.Audio{FileID: "a1"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "audio.ogg", att.filename)
}

func TestExtractAttachment_Voice(t *testing.T) {
	msg := &models.Message{
		Voice: &models.Voice{FileID: "v1"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "v1", att.fileID)
	assert.Equal(t, "voice.ogg", att.filename)
}

func TestExtractAttachment_Video(t *testing.T) {
	msg := &models.Message{
		Video: &models.Video{FileID: "vid1", FileName: "clip.mp4"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "vid1", att.fileID)
	assert.Equal(t, "clip.mp4", att.filename)
}

func TestExtractAttachment_VideoFallback(t *testing.T) {
	msg := &models.Message{
		Video: &models.Video{FileID: "vid1"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "video.mp4", att.filename)
}

func TestExtractAttachment_VideoNote(t *testing.T) {
	msg := &models.Message{
		VideoNote: &models.VideoNote{FileID: "vn1"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "vn1", att.fileID)
	assert.Equal(t, "video_note.mp4", att.filename)
}

func TestExtractAttachment_Animation(t *testing.T) {
	msg := &models.Message{
		Animation: &models.Animation{FileID: "an1", FileName: "funny.gif"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "an1", att.fileID)
	assert.Equal(t, "funny.gif", att.filename)
}

func TestExtractAttachment_AnimationFallback(t *testing.T) {
	msg := &models.Message{
		Animation: &models.Animation{FileID: "an1"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "animation.mp4", att.filename)
}

func TestExtractAttachment_Sticker(t *testing.T) {
	msg := &models.Message{
		Sticker: &models.Sticker{FileID: "st1"},
	}
	att := extractAttachment(msg)
	assert.NotNil(t, att)
	assert.Equal(t, "st1", att.fileID)
	assert.Equal(t, "sticker.webp", att.filename)
}

func TestExtractAttachment_None(t *testing.T) {
	msg := &models.Message{Text: "just text"}
	att := extractAttachment(msg)
	assert.Nil(t, att)
}

func TestNameOr(t *testing.T) {
	assert.Equal(t, "given.mp3", nameOr("given.mp3", "fallback.ogg"))
	assert.Equal(t, "fallback.ogg", nameOr("", "fallback.ogg"))
}

func TestIsChatAllowed(t *testing.T) {
	viper.Set("telegram.whitelist_chat_ids", "100,200,300")
	defer viper.Reset()

	assert.True(t, isChatAllowed(100))
	assert.True(t, isChatAllowed(200))
	assert.True(t, isChatAllowed(300))
	assert.False(t, isChatAllowed(999))
}

func TestChatDir_NoThread(t *testing.T) {
	viper.Set("data_dir", "/data")
	defer viper.Reset()

	dir := telegram.ChatDir(config.DataDir(), 12345, 0)
	assert.Equal(t, "/data/12345", dir)
}

func TestChatDir_WithThread(t *testing.T) {
	viper.Set("data_dir", "/data")
	defer viper.Reset()

	dir := telegram.ChatDir(config.DataDir(), 12345, 99)
	assert.Equal(t, "/data/12345/99", dir)
}

func TestBuildPrompt_NoAttachment(t *testing.T) {
	result := buildPrompt(context.TODO(), nil, "just text", nil, "/tmp")
	assert.Equal(t, "just text", result)
}

func TestProcessSendFiles_NoMatch(t *testing.T) {
	result := processSendFiles(context.TODO(), nil, "plain reply", 0, 0, "")
	assert.Equal(t, "plain reply", result)
}

func TestProcessSendFiles_StripsBlocks(t *testing.T) {
	reply := "before\n```nclaw:sendfile\n{\"path\":\"/nonexistent\",\"caption\":\"test\"}\n```\nafter"
	// sendFile will fail (no bot, file doesn't exist) but the block should still be stripped
	result := processSendFiles(context.TODO(), nil, reply, 0, 0, "")
	assert.Equal(t, "before\n\nafter", result)
}

func TestSendFileBlockRegex(t *testing.T) {
	input := "text\n```nclaw:sendfile\n{\"path\":\"file.txt\"}\n```\nmore"
	matches := sendFileBlockRe.FindAllStringSubmatch(input, -1)
	assert.Len(t, matches, 1)
	assert.Equal(t, "{\"path\":\"file.txt\"}", matches[0][1])
}

func TestSendFileBlockRegex_Multiple(t *testing.T) {
	input := "```nclaw:sendfile\n{\"path\":\"a.txt\"}\n```\nmiddle\n```nclaw:sendfile\n{\"path\":\"b.txt\"}\n```"
	matches := sendFileBlockRe.FindAllStringSubmatch(input, -1)
	assert.Len(t, matches, 2)
	assert.Equal(t, "{\"path\":\"a.txt\"}", matches[0][1])
	assert.Equal(t, "{\"path\":\"b.txt\"}", matches[1][1])
}

func setupTestWebhookManager(t *testing.T) *webhook.Manager {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.WebhookRegistration{}))

	send := func(_ context.Context, _ int64, _ int, _, _ string) error { return nil }
	return webhook.NewManager(database, send, "example.com", t.TempDir(), telegram.NewChatLocker())
}

func TestHandlerWebhookManager_NilStripsBlocks(t *testing.T) {
	h := &Handler{}
	// Simulating what callClaude does after getting a reply from Claude.
	// With nil WebhookManager, webhook blocks are stripped (not executed).
	reply := "text\n```nclaw:webhook\n{\"action\":\"list\"}\n```\nmore"
	if h.WebhookManager != nil {
		reply = h.WebhookManager.ProcessReply(reply, 100, 0)
	} else {
		reply = webhook.StripBlocks(reply)
	}
	assert.NotContains(t, reply, "nclaw:webhook")
	assert.Contains(t, reply, "text")
	assert.Contains(t, reply, "more")
}

func TestHandlerWebhookManager_ProcessesBlocks(t *testing.T) {
	mgr := setupTestWebhookManager(t)
	h := &Handler{WebhookManager: mgr}

	reply := "Here you go.\n```nclaw:webhook\n" +
		`{"action":"create","description":"test hook"}` +
		"\n```\nDone!"

	// Simulate the webhook processing step from callClaude.
	if h.WebhookManager != nil {
		reply = h.WebhookManager.ProcessReply(reply, 100, 0)
	}

	assert.NotContains(t, reply, "nclaw:webhook")
	assert.Contains(t, reply, "Here you go.")
	assert.Contains(t, reply, "Done!")
	assert.Contains(t, reply, "[Webhook created: https://example.com/webhooks/")
}

func TestHandlerWebhookManager_ListEmpty(t *testing.T) {
	mgr := setupTestWebhookManager(t)
	h := &Handler{WebhookManager: mgr}

	reply := "```nclaw:webhook\n{\"action\":\"list\"}\n```"
	if h.WebhookManager != nil {
		reply = h.WebhookManager.ProcessReply(reply, 100, 0)
	}

	assert.Contains(t, reply, "[No webhooks registered]")
	assert.NotContains(t, reply, "nclaw:webhook")
}
