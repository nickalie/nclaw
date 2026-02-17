package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"

	"github.com/nickalie/nclaw/internal/config"
	"github.com/nickalie/nclaw/internal/db"
	"github.com/nickalie/nclaw/internal/handler"
	"github.com/nickalie/nclaw/internal/scheduler"
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	defer sched.Shutdown()

	fmt.Println("nclaw bot started")
	b.Start(ctx)
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
