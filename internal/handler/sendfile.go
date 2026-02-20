package handler

import (
	"bytes"
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/nickalie/nclaw/internal/sendfile"
)

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
