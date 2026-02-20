package webhook

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/nickalie/nclaw/internal/claude"
	"github.com/nickalie/nclaw/internal/db"
	"github.com/nickalie/nclaw/internal/model"
)

// SendFunc sends a text message to a Telegram chat/thread.
type SendFunc func(ctx context.Context, chatID int64, threadID int, text string) error

// IncomingRequest holds the data from an incoming webhook HTTP request.
type IncomingRequest struct {
	Method  string
	Headers map[string]string
	Query   map[string]string
	Body    string
}

const maxConcurrentWebhooks = 5

// Manager handles webhook registration and incoming webhook processing.
type Manager struct {
	db         *gorm.DB
	send       SendFunc
	baseDomain string
	dataDir    string
	sem        chan struct{}
}

// NewManager creates a new webhook Manager.
func NewManager(database *gorm.DB, send SendFunc, baseDomain, dataDir string) *Manager {
	return &Manager{
		db:         database,
		send:       send,
		baseDomain: baseDomain,
		dataDir:    dataDir,
		sem:        make(chan struct{}, maxConcurrentWebhooks),
	}
}

// Create registers a new webhook and returns it.
func (m *Manager) Create(description string, chatID int64, threadID int) (*model.WebhookRegistration, error) {
	webhook := &model.WebhookRegistration{
		ID:          model.GenerateWebhookID(),
		ChatID:      chatID,
		ThreadID:    threadID,
		Description: description,
		Status:      model.WebhookStatusActive,
	}
	if err := db.CreateWebhook(m.db, webhook); err != nil {
		return nil, fmt.Errorf("webhook: create: %w", err)
	}
	log.Printf("webhook: created %s for chat=%d thread=%d desc=%q", webhook.ID, chatID, threadID, description)
	return webhook, nil
}

// Delete removes a webhook by ID.
func (m *Manager) Delete(webhookID string) error {
	if err := db.DeleteWebhook(m.db, webhookID); err != nil {
		return fmt.Errorf("webhook: delete: %w", err)
	}
	log.Printf("webhook: deleted %s", webhookID)
	return nil
}

// List returns all webhooks for a chat/thread.
func (m *Manager) List(chatID int64, threadID int) ([]model.WebhookRegistration, error) {
	return db.ListWebhooksByChat(m.db, chatID, threadID)
}

// WebhookURL returns the full URL for a webhook.
func (m *Manager) WebhookURL(webhookID string) string {
	return fmt.Sprintf("https://%s/webhooks/%s", m.baseDomain, webhookID)
}

// HandleIncoming looks up a webhook by ID and processes the request asynchronously.
// Returns an error if the webhook is not found or inactive (caller should return 404).
func (m *Manager) HandleIncoming(webhookID string, req IncomingRequest) error {
	webhook, err := db.GetWebhookByID(m.db, webhookID)
	if err != nil {
		return err
	}
	if webhook.Status != model.WebhookStatusActive {
		return errors.New("webhook inactive")
	}

	log.Printf("webhook: incoming request for %s method=%s", webhookID, req.Method)
	select {
	case m.sem <- struct{}{}:
		go func() {
			defer func() { <-m.sem }()
			m.processIncoming(webhook, req)
		}()
	default:
		log.Printf("webhook: concurrency limit reached for %s", webhookID)
		return errors.New("too many concurrent requests")
	}
	return nil
}

func (m *Manager) processIncoming(webhook *model.WebhookRegistration, req IncomingRequest) {
	prompt := buildIncomingPrompt(webhook, req)

	dir := webhookChatDir(m.dataDir, webhook.ChatID, webhook.ThreadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("webhook: mkdir %s: %v", dir, err)
		return
	}

	if err := claude.EnsureValidToken(); err != nil {
		log.Printf("webhook: token refresh warning: %v", err)
	}

	reply, err := claude.New().Dir(dir).SkipPermissions().Continue(prompt)
	if err != nil {
		log.Printf("webhook: claude error for %s: %v", webhook.ID, err)
		if reply == "" {
			reply = "Webhook processing error: " + err.Error()
		}
	}

	if reply == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.send(ctx, webhook.ChatID, webhook.ThreadID, reply); err != nil {
		log.Printf("webhook: send error for %s: %v", webhook.ID, err)
	}
}

func buildIncomingPrompt(webhook *model.WebhookRegistration, req IncomingRequest) string {
	var b strings.Builder
	b.WriteString("[WEBHOOK REQUEST - Incoming HTTP request to your registered webhook]\n\n")
	fmt.Fprintf(&b, "Webhook: %s\n", webhook.Description)
	fmt.Fprintf(&b, "Method: %s\n", req.Method)

	if len(req.Headers) > 0 {
		b.WriteString("Headers:\n")
		for k, v := range req.Headers {
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}

	if len(req.Query) > 0 {
		b.WriteString("Query Parameters:\n")
		for k, v := range req.Query {
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}

	if req.Body != "" {
		fmt.Fprintf(&b, "Body:\n%s\n", req.Body)
	}

	return b.String()
}

func webhookChatDir(dataDir string, chatID int64, threadID int) string {
	dir := filepath.Join(dataDir, fmt.Sprintf("%d", chatID))
	if threadID != 0 {
		dir = filepath.Join(dir, fmt.Sprintf("%d", threadID))
	}
	return dir
}
