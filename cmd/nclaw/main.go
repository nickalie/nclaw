package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/gorm"

	"github.com/nickalie/nclaw/internal/config"
	"github.com/nickalie/nclaw/internal/db"
	"github.com/nickalie/nclaw/internal/handler"
	"github.com/nickalie/nclaw/internal/scheduler"
	"github.com/nickalie/nclaw/internal/sendfile"
	"github.com/nickalie/nclaw/internal/telegram"
	"github.com/nickalie/nclaw/internal/version"
	"github.com/nickalie/nclaw/internal/webhook"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(config.DBPath())
	if err != nil {
		log.Fatal("db open: ", err)
	}

	if err := db.Init(database); err != nil {
		log.Fatal("db init: ", err)
	}

	chatLocker := telegram.NewChatLocker()
	h := &handler.Handler{ChatLocker: chatLocker}

	b, err := bot.New(config.TelegramBotToken(),
		bot.WithDefaultHandler(h.Default),
	)
	if err != nil {
		log.Fatal(err)
	}

	sendDoc := newSendDocFunc(b)
	h.SendDoc = sendDoc
	sched, err := scheduler.New(database, newSendFunc(b), sendDoc, config.Timezone(), config.DataDir(), chatLocker)
	if err != nil {
		log.Fatal("scheduler: ", err)
	}
	sched.LoadTasks()
	sched.Start()

	h.Scheduler = sched

	webhookSrv, webhookMgr := startWebhooks(database, b, chatLocker)
	h.WebhookManager = webhookMgr

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	defer sched.Shutdown()
	defer shutdownWebhook(webhookSrv, webhookMgr)

	log.Printf("nclaw bot started (%s)", version.String())
	sendStartupNotifications(b)
	b.Start(ctx)
}

func sendStartupNotifications(b *bot.Bot) {
	chatIDs := config.WhitelistChatIDs()
	if len(chatIDs) == 0 {
		return
	}

	text := "nclaw bot started\n" + version.String()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, chatID := range chatIDs {
		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   text,
		}); err != nil {
			log.Printf("startup notify chat %d: %v", chatID, err)
		}
	}
}

func newSendDocFunc(b *bot.Bot) sendfile.SendDocFunc {
	return func(ctx context.Context, chatID int64, threadID int, filename string, data []byte, caption string) error {
		_, err := b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Document:        &models.InputFileUpload{Filename: filename, Data: bytes.NewReader(data)},
			Caption:         caption,
		})
		return err
	}
}

func newSendFunc(b *bot.Bot) scheduler.SendFunc {
	return func(ctx context.Context, chatID int64, threadID int, text string) error {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            text,
		})
		return err
	}
}

func newWebhookSendFunc(b *bot.Bot) webhook.SendFunc {
	return func(ctx context.Context, chatID int64, threadID int, text, parseMode string) error {
		params := &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            text,
		}
		if parseMode != "" {
			params.ParseMode = models.ParseMode(parseMode)
		}
		_, err := b.SendMessage(ctx, params)
		return err
	}
}

func startWebhooks(
	database *gorm.DB, b *bot.Bot, chatLocker *telegram.ChatLocker,
) (*webhook.Server, *webhook.Manager) {
	domain := config.WebhookBaseDomain()
	if domain == "" {
		return nil, nil
	}

	mgr := webhook.NewManager(database, newWebhookSendFunc(b), domain, config.DataDir(), chatLocker)
	srv := webhook.NewServer(mgr)

	listenErr := make(chan error, 1)
	go func() {
		if err := srv.Listen(config.WebhookPort()); err != nil {
			listenErr <- err
		}
	}()

	// Give the listener a moment to fail on bind errors.
	select {
	case err := <-listenErr:
		log.Fatalf("webhook server failed to start: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// Monitor for runtime listener failures.
	go func() {
		if err, ok := <-listenErr; ok {
			log.Fatalf("webhook server error: %v", err)
		}
	}()

	return srv, mgr
}

func shutdownWebhook(srv *webhook.Server, mgr *webhook.Manager) {
	if srv != nil {
		if err := srv.Shutdown(); err != nil {
			log.Printf("webhook shutdown: %v", err)
		}
	}
	if mgr != nil {
		mgr.Wait()
	}
}
