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
	"github.com/nickalie/nclaw/internal/pipeline"
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

	fileSenders := sendfile.Senders{
		Doc:   newSendDocFunc(b),
		Audio: newSendAudioFunc(b),
	}
	sched, err := scheduler.New(database, config.Timezone(), config.DataDir(), chatLocker)
	if err != nil {
		log.Fatal("scheduler: ", err)
	}

	h.Scheduler = sched

	webhookMgr := createWebhookManager(database, chatLocker)

	// Build pipeline executors; avoid Go nil interface trap for webhookMgr.
	executors := []pipeline.BlockExecutor{sched}
	if webhookMgr != nil {
		executors = append(executors, webhookMgr)
	}
	fileSenders.MediaGroup = newSendMediaGroupFunc(b)
	p := pipeline.New(newPipelineSendFunc(b), fileSenders, webhookMgr != nil, executors...)
	h.Pipeline = p
	sched.SetPipeline(p)
	if webhookMgr != nil {
		webhookMgr.SetPipeline(p)
	}

	// Load tasks and start scheduler before webhook server to avoid a race where
	// an incoming webhook creates a task that LoadTasks then re-registers as a duplicate job.
	sched.LoadTasks()
	sched.Start()

	// Start webhook HTTP server after pipeline is wired and scheduler is loaded.
	webhookSrv := startWebhookServer(webhookMgr)

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

func newSendAudioFunc(b *bot.Bot) sendfile.SendAudioFunc {
	return func(ctx context.Context, chatID int64, threadID int, filename string, data []byte, caption string) error {
		_, err := b.SendAudio(ctx, &bot.SendAudioParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Audio:           &models.InputFileUpload{Filename: filename, Data: bytes.NewReader(data)},
			Caption:         caption,
		})
		return err
	}
}

func newSendMediaGroupFunc(b *bot.Bot) sendfile.SendMediaGroupFunc {
	return func(ctx context.Context, chatID int64, threadID int, files []sendfile.File) error {
		media := make([]models.InputMedia, len(files))
		for i, f := range files {
			media[i] = buildInputMedia(f)
		}
		_, err := b.SendMediaGroup(ctx, &bot.SendMediaGroupParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Media:           media,
		})
		return err
	}
}

func buildInputMedia(f sendfile.File) models.InputMedia {
	attach := "attach://" + f.Filename
	reader := bytes.NewReader(f.Data)
	switch f.MediaType {
	case sendfile.MediaAudio:
		return &models.InputMediaAudio{
			Media: attach, Caption: f.Caption, MediaAttachment: reader,
		}
	case sendfile.MediaPhoto:
		return &models.InputMediaPhoto{
			Media: attach, Caption: f.Caption, MediaAttachment: reader,
		}
	case sendfile.MediaVideo:
		return &models.InputMediaVideo{
			Media: attach, Caption: f.Caption, MediaAttachment: reader,
		}
	default:
		return &models.InputMediaDocument{
			Media: attach, Caption: f.Caption, MediaAttachment: reader,
		}
	}
}

func newPipelineSendFunc(b *bot.Bot) pipeline.SendFunc {
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

func createWebhookManager(database *gorm.DB, chatLocker *telegram.ChatLocker) *webhook.Manager {
	domain := config.WebhookBaseDomain()
	if domain == "" {
		return nil
	}
	return webhook.NewManager(database, domain, config.DataDir(), chatLocker)
}

func startWebhookServer(mgr *webhook.Manager) *webhook.Server {
	if mgr == nil {
		return nil
	}

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

	return srv
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
