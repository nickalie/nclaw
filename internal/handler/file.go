package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// attachment holds normalized file info extracted from a Telegram message.
type attachment struct {
	fileID   string
	filename string
}

// extractAttachment returns file info if the message contains a file, or nil otherwise.
func extractAttachment(msg *models.Message) *attachment {
	if att := extractMedia(msg); att != nil {
		return att
	}

	return extractExtra(msg)
}

func extractMedia(msg *models.Message) *attachment {
	switch {
	case msg.Document != nil:
		return &attachment{fileID: msg.Document.FileID, filename: msg.Document.FileName}
	case len(msg.Photo) > 0:
		best := msg.Photo[len(msg.Photo)-1]
		return &attachment{fileID: best.FileID, filename: "photo.jpg"}
	case msg.Audio != nil:
		return &attachment{fileID: msg.Audio.FileID, filename: nameOr(msg.Audio.FileName, "audio.ogg")}
	case msg.Voice != nil:
		return &attachment{fileID: msg.Voice.FileID, filename: "voice.ogg"}
	default:
		return nil
	}
}

func extractExtra(msg *models.Message) *attachment {
	switch {
	case msg.Video != nil:
		return &attachment{fileID: msg.Video.FileID, filename: nameOr(msg.Video.FileName, "video.mp4")}
	case msg.VideoNote != nil:
		return &attachment{fileID: msg.VideoNote.FileID, filename: "video_note.mp4"}
	case msg.Animation != nil:
		return &attachment{fileID: msg.Animation.FileID, filename: nameOr(msg.Animation.FileName, "animation.mp4")}
	case msg.Sticker != nil:
		return &attachment{fileID: msg.Sticker.FileID, filename: "sticker.webp"}
	default:
		return nil
	}
}

func nameOr(name, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}

// downloadAttachment fetches a file from Telegram and saves it into dir. Returns the local path.
func downloadAttachment(ctx context.Context, b *bot.Bot, att *attachment, dir string) (string, error) {
	f, err := b.GetFile(ctx, &bot.GetFileParams{FileID: att.fileID})
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}

	link := b.FileDownloadLink(f)
	log.Printf("handler: downloading file %s from %s", att.filename, link)

	localPath := filepath.Join(dir, att.filename)

	if err := fetchToFile(ctx, link, localPath); err != nil {
		return "", fmt.Errorf("download %s: %w", att.filename, err)
	}

	return localPath, nil
}

func fetchToFile(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
