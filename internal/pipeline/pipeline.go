package pipeline

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/nickalie/nclaw/internal/claude"
	"github.com/nickalie/nclaw/internal/sendfile"
	"github.com/nickalie/nclaw/internal/telegram"
)

// BlockExecutor processes command blocks extracted from Claude's response.
// ExecuteBlocks scans text for command blocks, executes them, and returns
// any status/error messages to append to the display text.
type BlockExecutor interface {
	ExecuteBlocks(text string, chatID int64, threadID int) string
}

// SendFunc sends a text message to a Telegram chat/thread with an optional parse mode.
type SendFunc func(ctx context.Context, chatID int64, threadID int, text, parseMode string) error

// Own copies of block regexes to avoid import cycles with scheduler/webhook packages.
var (
	scheduleBlockRe = regexp.MustCompile("(?s)```nclaw:schedule\n(.*?)\n```")
	webhookBlockRe  = regexp.MustCompile("(?s)```nclaw:webhook\n(.*?)\n```")
)

// Pipeline orchestrates post-Claude response processing: block execution,
// stripping, status appending, file sending, and reply delivery.
type Pipeline struct {
	executors          []BlockExecutor
	sendDoc            sendfile.SendDocFunc
	sendMediaGroup     sendfile.SendMediaGroupFunc
	send               SendFunc
	webhooksConfigured bool
}

// New creates a Pipeline. Nil executors are silently filtered out.
// webhooksConfigured indicates whether a webhook executor is present, used to
// warn users when webhook blocks appear but webhooks are not enabled.
func New(
	send SendFunc, sendDoc sendfile.SendDocFunc, sendMediaGroup sendfile.SendMediaGroupFunc,
	webhooksConfigured bool, executors ...BlockExecutor,
) *Pipeline {
	var filtered []BlockExecutor
	for _, e := range executors {
		if e != nil {
			filtered = append(filtered, e)
		}
	}
	return &Pipeline{
		executors:          filtered,
		sendDoc:            sendDoc,
		sendMediaGroup:     sendMediaGroup,
		send:               send,
		webhooksConfigured: webhooksConfigured,
	}
}

// Process handles the full post-Claude response workflow:
//  1. Execute command blocks on FullText (only when claudeErr is nil)
//  2. Strip all command block syntax from Text
//  3. Append execution status messages
//  4. Send the reply with HTML-then-plain-text fallback
func (p *Pipeline) Process(
	ctx context.Context, result *claude.Result, claudeErr error,
	chatID int64, threadID int, dir string,
) {
	var statusMsgs []string

	// Phase 1: Execute command blocks (only on success).
	if claudeErr == nil {
		for _, exec := range p.executors {
			if msg := exec.ExecuteBlocks(result.FullText, chatID, threadID); msg != "" {
				statusMsgs = append(statusMsgs, msg)
			}
		}
		sendfile.ExecuteBlocks(ctx, p.sendDoc, p.sendMediaGroup, result.FullText, chatID, threadID, dir)
	}

	// Phase 2: Strip all command block syntax from display text.
	text := stripAllBlocks(result.Text)

	// Warn if webhook blocks were found but webhooks aren't configured.
	if !p.webhooksConfigured && webhookBlockRe.MatchString(result.Text) {
		statusMsgs = append(statusMsgs, "[Webhooks are not configured on this instance]")
	}

	// Phase 3: Append status messages from block execution.
	text = appendStatus(text, statusMsgs)

	// Phase 4: Send reply.
	if text != "" {
		p.sendReply(ctx, chatID, threadID, text)
	}
}

// stripAllBlocks removes all known command block types from text.
func stripAllBlocks(text string) string {
	text = sendfile.StripBlocks(text)
	text = scheduleBlockRe.ReplaceAllString(text, "")
	text = webhookBlockRe.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func appendStatus(text string, msgs []string) string {
	for _, msg := range msgs {
		if msg != "" {
			text = strings.TrimSpace(text) + "\n\n" + msg
		}
	}
	return strings.TrimSpace(text)
}

func (p *Pipeline) sendReply(ctx context.Context, chatID int64, threadID int, text string) {
	log.Printf("pipeline: sending reply len=%d", len(text))
	for _, chunk := range telegram.SplitMessage(text, telegram.MaxMessageLen) {
		p.sendChunk(ctx, chatID, threadID, chunk)
	}
}

func (p *Pipeline) sendChunk(ctx context.Context, chatID int64, threadID int, text string) {
	for _, mode := range []string{"HTML", ""} {
		if err := p.send(ctx, chatID, threadID, text, mode); err == nil {
			return
		} else {
			log.Printf("pipeline: send parseMode=%q error: %v", mode, err)
		}
	}
	log.Printf("pipeline: failed to send message to chat=%d thread=%d (all modes failed)", chatID, threadID)
}
