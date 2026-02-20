package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/nickalie/nclaw/internal/claude"
	"github.com/nickalie/nclaw/internal/config"
	"github.com/nickalie/nclaw/internal/scheduler"
	"github.com/nickalie/nclaw/internal/webhook"
)

const telegramPrompt = `IMPORTANT: Your output will be displayed in Telegram.
Format all responses using Telegram HTML. Supported tags:
<b>bold</b>, <i>italic</i>, <u>underline</u>, <s>strikethrough</s>,
<code>inline code</code>, <pre>code block</pre>, <pre><code class="language-go">code with language</code></pre>,
<a href="URL">link</a>, <blockquote>quote</blockquote>, <tg-spoiler>spoiler</tg-spoiler>

Rules:
- Do NOT use Markdown syntax (no #headers, no **bold**, no backticks for code)
- Use ONLY the HTML tags listed above. No other HTML tags are supported.
- Escape &, < and > in regular text as &amp; &lt; &gt; (but not inside tags themselves)
- Do NOT use <p>, <br>, <div>, <h1>-<h6>, <ul>, <li>, <ol>, <table>, or any other HTML tags
- For lists, use plain text with bullet characters or numbers
- For section titles, use <b>bold text</b> on its own line
- Keep formatting minimal and clean`

// Handler processes incoming Telegram messages.
type Handler struct {
	Scheduler      *scheduler.Scheduler
	WebhookManager *webhook.Manager
}

// Default handles incoming messages by forwarding them to Claude Code.
func (h *Handler) Default(parentCtx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message

	if !isChatAllowed(msg.Chat.ID) {
		log.Printf("handler: ignoring message from non-whitelisted chat=%d", msg.Chat.ID)
		return
	}

	text, att := resolveContent(msg)
	if text == "" && att == nil {
		log.Printf("handler: skipping update (no text or attachment)")
		return
	}

	go h.processMessage(parentCtx, b, msg, text, att)
}

func (h *Handler) processMessage(ctx context.Context, b *bot.Bot, msg *models.Message, text string, att *attachment) {
	text = withReplyContext(msg, text)

	chatID := msg.Chat.ID
	threadID := msg.MessageThreadID
	dir := chatDir(chatID, threadID)
	ensureDir(dir)

	typingCtx, stopTyping := context.WithCancel(ctx)
	defer stopTyping()

	go sendTyping(typingCtx, b, chatID, threadID)

	prompt := buildPrompt(ctx, b, text, att, dir)
	log.Printf("handler: received message from chat=%d thread=%d text=%q hasFile=%v", chatID, threadID, text, att != nil)

	reply := h.callClaude(dir, prompt, chatID, threadID)
	stopTyping()

	reply = processSendFiles(ctx, b, reply, chatID, threadID, dir)

	if reply != "" {
		sendReply(ctx, b, chatID, threadID, reply)
	}
}

// resolveContent extracts text and attachment from a message, falling back to reply attachment.
func resolveContent(msg *models.Message) (string, *attachment) {
	text, att := messageContent(msg)
	if att == nil && msg.ReplyToMessage != nil {
		att = extractAttachment(msg.ReplyToMessage)
	}
	return text, att
}

// withReplyContext prepends the original message text when the user replies to a message.
func withReplyContext(msg *models.Message, text string) string {
	if msg.ReplyToMessage == nil {
		return text
	}

	original := msg.ReplyToMessage.Text
	if original == "" {
		original = msg.ReplyToMessage.Caption
	}

	if original == "" {
		return text
	}

	return fmt.Sprintf("[Replying to message: %s]\n\n%s", original, text)
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
	systemPrompt := telegramPrompt + "\n\n" + taskPrompt

	if err := claude.EnsureValidToken(); err != nil {
		log.Printf("handler: token refresh warning: %v", err)
	}

	log.Printf("handler: calling claude.Continue in dir=%s", dir)
	reply, err := claude.New().Dir(dir).SkipPermissions().AppendSystemPrompt(systemPrompt).Continue(prompt)

	if err != nil {
		log.Printf("handler: claude error: %v", err)
		if reply == "" {
			reply = "error: " + err.Error()
		}
	}

	reply = h.Scheduler.ProcessReply(reply, chatID, threadID)
	if h.WebhookManager != nil {
		reply = h.WebhookManager.ProcessReply(reply, chatID, threadID)
	} else {
		reply = webhook.StripBlocks(reply)
	}
	return reply
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

func isChatAllowed(chatID int64) bool {
	return slices.Contains(config.WhitelistChatIDs(), chatID)
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

const maxMessageLen = 4096

func sendReply(ctx context.Context, b *bot.Bot, chatID int64, threadID int, text string) {
	log.Printf("handler: sending reply len=%d", len(text))

	for _, chunk := range splitMessage(text, maxMessageLen) {
		sendChunk(ctx, b, chatID, threadID, chunk)
	}
}

func sendChunk(ctx context.Context, b *bot.Bot, chatID int64, threadID int, text string) {
	modes := []models.ParseMode{models.ParseModeHTML, ""}
	for _, mode := range modes {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            text,
			ParseMode:       mode,
		})
		if err == nil {
			return
		}
		log.Printf("handler: SendMessage parseMode=%q error: %v", mode, err)
	}
}

// splitMessage splits text into chunks of at most maxLen characters, breaking at newlines.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string

	for text != "" {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		cut := strings.LastIndex(text[:maxLen], "\n")
		if cut <= 0 {
			cut = maxLen
		}

		chunks = append(chunks, text[:cut])
		text = strings.TrimLeft(text[cut:], "\n")
	}

	return chunks
}
