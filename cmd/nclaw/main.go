package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/nickalie/nclaw/internal/config"
	"github.com/nickalie/nclaw/internal/db"
	"github.com/nickalie/nclaw/internal/handler"
	"github.com/nickalie/nclaw/internal/scheduler"
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

	h := &handler.Handler{}

	b, err := bot.New(config.TelegramBotToken(),
		bot.WithDefaultHandler(h.Default),
	)
	if err != nil {
		log.Fatal(err)
	}

	sched, err := scheduler.New(database, newSendFunc(b), config.Timezone(), config.DataDir())
	if err != nil {
		log.Fatal("scheduler: ", err)
	}
	sched.LoadTasks()
	sched.Start()

	h.Scheduler = sched

	var webhookSrv *webhook.Server
	if domain := config.WebhookBaseDomain(); domain != "" {
		mgr := webhook.NewManager(database, newWebhookSendFunc(b), domain, config.DataDir())
		h.WebhookManager = mgr

		webhookSrv = webhook.NewServer(mgr)
		go func() {
			if err := webhookSrv.Listen(config.WebhookPort()); err != nil {
				log.Fatalf("webhook server: %v", err)
			}
		}()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	defer sched.Shutdown()
	defer shutdownWebhook(webhookSrv)

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

func shutdownWebhook(srv *webhook.Server) {
	if srv == nil {
		return
	}
	if err := srv.Shutdown(); err != nil {
		log.Printf("webhook shutdown: %v", err)
	}
}
