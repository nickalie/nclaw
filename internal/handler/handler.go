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
	if update.Message == nil || update.Message.Text == "" {
		log.Printf("handler: skipping update (no message or empty text)")
		return
	}

	msg := update.Message
	chatID := msg.Chat.ID
	threadID := msg.MessageThreadID
	log.Printf("handler: received message from chat=%d thread=%d text=%q", chatID, threadID, msg.Text)

	dir := chatDir(chatID, threadID)
	ensureDir(dir)

	typingCtx, stopTyping := context.WithCancel(parentCtx)
	defer stopTyping()

	go sendTyping(typingCtx, b, chatID, threadID)

	taskPrompt := h.Scheduler.FormatTaskList(chatID, threadID)

	log.Printf("handler: calling claude.Continue in dir=%s", dir)
	reply, err := claude.New().Dir(dir).AppendSystemPrompt(taskPrompt).Continue(msg.Text)
	stopTyping()

	if err != nil {
		log.Printf("handler: claude error: %v", err)
		reply = "error: " + err.Error()
	}

	reply = h.Scheduler.ProcessReply(reply, chatID, threadID)

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
