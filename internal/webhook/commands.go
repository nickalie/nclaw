package webhook

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/nickalie/nclaw/internal/db"
)

var webhookBlockRe = regexp.MustCompile("(?s)```nclaw:webhook\n(.*?)\n```")

type webhookCommand struct {
	Action      string `json:"action"`
	Description string `json:"description"`
	WebhookID   string `json:"webhook_id"`
}

// ProcessReply extracts nclaw:webhook code blocks from a reply, executes them, and returns cleaned text.
func (m *Manager) ProcessReply(reply string, chatID int64, threadID int) string {
	matches := webhookBlockRe.FindAllStringSubmatchIndex(reply, -1)
	if len(matches) == 0 {
		return reply
	}

	var results []string
	var errs []string

	for _, match := range matches {
		jsonStr := reply[match[2]:match[3]]
		result, err := m.executeCommand(jsonStr, chatID, threadID)
		if err != nil {
			log.Printf("webhook: command error: %v", err)
			errs = append(errs, err.Error())
		} else if result != "" {
			results = append(results, result)
		}
	}

	cleaned := webhookBlockRe.ReplaceAllString(reply, "")
	cleaned = strings.TrimSpace(cleaned)

	if len(results) > 0 {
		cleaned += "\n\n" + strings.Join(results, "\n")
	}
	if len(errs) > 0 {
		cleaned += "\n\n[Webhook error: " + strings.Join(errs, "; ") + "]"
	}

	return cleaned
}

func (m *Manager) executeCommand(jsonStr string, chatID int64, threadID int) (string, error) {
	var cmd webhookCommand
	if err := json.Unmarshal([]byte(jsonStr), &cmd); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	log.Printf("webhook: processing command action=%s webhook_id=%s", cmd.Action, cmd.WebhookID)

	switch cmd.Action {
	case "create":
		return m.createFromCommand(cmd.Description, chatID, threadID)
	case "delete":
		return m.deleteFromCommand(cmd.WebhookID, chatID, threadID)
	case "list":
		return m.listFromCommand(chatID, threadID)
	default:
		return "", fmt.Errorf("unknown action %q", cmd.Action)
	}
}

func (m *Manager) createFromCommand(description string, chatID int64, threadID int) (string, error) {
	if description == "" {
		return "", fmt.Errorf("create requires description")
	}
	webhook, err := m.Create(description, chatID, threadID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[Webhook created: %s]", m.WebhookURL(webhook.ID)), nil
}

func (m *Manager) deleteFromCommand(webhookID string, chatID int64, threadID int) (string, error) {
	if webhookID == "" {
		return "", fmt.Errorf("delete requires webhook_id")
	}
	wh, err := db.GetWebhookByID(m.db, webhookID)
	if err != nil {
		return "", fmt.Errorf("webhook not found: %w", err)
	}
	if wh.ChatID != chatID || wh.ThreadID != threadID {
		return "", fmt.Errorf("webhook %s does not belong to this chat", webhookID)
	}
	if err := m.Delete(webhookID); err != nil {
		return "", err
	}
	return fmt.Sprintf("[Webhook deleted: %s]", webhookID), nil
}

func (m *Manager) listFromCommand(chatID int64, threadID int) (string, error) {
	webhooks, err := m.List(chatID, threadID)
	if err != nil {
		return "", err
	}
	if len(webhooks) == 0 {
		return "[No webhooks registered]", nil
	}

	var b strings.Builder
	b.WriteString("[Registered webhooks:\n")
	for _, wh := range webhooks {
		fmt.Fprintf(&b, "- %s: %s (status: %s, url: %s)\n", wh.ID, wh.Description, wh.Status, m.WebhookURL(wh.ID))
	}
	b.WriteString("]")
	return b.String(), nil
}
