package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/nickalie/nclaw/internal/claude"
	"github.com/nickalie/nclaw/internal/config"
	"github.com/nickalie/nclaw/internal/scheduler"
)

// Handler processes incoming Telegram messages.
type Handler struct {
	Scheduler *scheduler.Scheduler
}

// Default handles incoming messages by forwarding them to Claude Code.
func (h *Handler) Default(parentCtx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	text, att := messageContent(msg)

	if text == "" && att == nil {
		log.Printf("handler: skipping update (no text or attachment)")
		return
	}

	chatID := msg.Chat.ID
	threadID := msg.MessageThreadID
	dir := chatDir(chatID, threadID)
	ensureDir(dir)

	typingCtx, stopTyping := context.WithCancel(parentCtx)
	defer stopTyping()

	go sendTyping(typingCtx, b, chatID, threadID)

	prompt := buildPrompt(parentCtx, b, text, att, dir)
	log.Printf("handler: received message from chat=%d thread=%d text=%q hasFile=%v", chatID, threadID, text, att != nil)

	reply := h.callClaude(dir, prompt, chatID, threadID)
	stopTyping()

	reply = processSendFiles(parentCtx, b, reply, chatID, threadID, dir)

	if reply != "" {
		log.Printf("handler: sending reply len=%d", len(reply))
		_, sendErr := b.SendMessage(parentCtx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            reply,
		})

		if sendErr != nil {
			log.Printf("handler: SendMessage error: %v", sendErr)
		}
	}
}

// messageContent extracts the user text and optional attachment from a message.
func messageContent(msg *models.Message) (string, *attachment) {
	att := extractAttachment(msg)

	text := msg.Text
	if att != nil && text == "" {
		text = msg.Caption
	}

	return text, att
}

// buildPrompt constructs the prompt for Claude, downloading any attachment first.
func buildPrompt(ctx context.Context, b *bot.Bot, text string, att *attachment, dir string) string {
	if att == nil {
		return text
	}

	localPath, err := downloadAttachment(ctx, b, att, dir)
	if err != nil {
		log.Printf("handler: download error: %v", err)
		return text + "\n\n(file attachment failed to download: " + err.Error() + ")"
	}

	prompt := fmt.Sprintf("I'm sending you a file: %s (saved at %s). Please read it.\n\n", att.filename, localPath)
	if text != "" {
		prompt += text
	}

	return prompt
}

func (h *Handler) callClaude(dir, prompt string, chatID int64, threadID int) string {
	taskPrompt := h.Scheduler.FormatTaskList(chatID, threadID)

	log.Printf("handler: calling claude.Continue in dir=%s", dir)
	reply, err := claude.New().Dir(dir).SkipPermissions().AppendSystemPrompt(taskPrompt).Continue(prompt)

	if err != nil {
		log.Printf("handler: claude error: %v", err)
		reply = "error: " + err.Error()
	}

	return h.Scheduler.ProcessReply(reply, chatID, threadID)
}

func sendTyping(ctx context.Context, b *bot.Bot, chatID int64, threadID int) {
	params := &bot.SendChatActionParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Action:          models.ChatActionTyping,
	}

	for {
		b.SendChatAction(ctx, params)

		select {
		case <-ctx.Done():
			return
		case <-time.After(4 * time.Second):
		}
	}
}

func chatDir(chatID int64, threadID int) string {
	base := filepath.Join(config.DataDir(), fmt.Sprintf("%d", chatID))
	if threadID != 0 {
		return filepath.Join(base, fmt.Sprintf("%d", threadID))
	}
	return base
}

func ensureDir(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("failed to create dir %s: %v", dir, err)
	}
}
