package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var sendFileBlockRe = regexp.MustCompile("(?s)```nclaw:sendfile\n(.*?)\n```")

type sendFileCommand struct {
	Path    string `json:"path"`
	Caption string `json:"caption"`
}

// processSendFiles extracts nclaw:sendfile blocks, sends files via Telegram, and returns cleaned text.
func processSendFiles(ctx context.Context, b *bot.Bot, reply string, chatID int64, threadID int, dir string) string {
	matches := sendFileBlockRe.FindAllStringSubmatch(reply, -1)
	if len(matches) == 0 {
		return reply
	}

	for _, match := range matches {
		sendFile(ctx, b, match[1], chatID, threadID, dir)
	}

	cleaned := sendFileBlockRe.ReplaceAllString(reply, "")
	return strings.TrimSpace(cleaned)
}

func sendFile(ctx context.Context, b *bot.Bot, jsonStr string, chatID int64, threadID int, dir string) {
	var cmd sendFileCommand
	if err := json.Unmarshal([]byte(jsonStr), &cmd); err != nil {
		log.Printf("handler: sendfile invalid JSON: %v", err)
		return
	}

	filePath := cmd.Path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Clean(filepath.Join(dir, filePath))
		if !strings.HasPrefix(filePath, dir+string(filepath.Separator)) && filePath != dir {
			log.Printf("handler: sendfile path %q escapes chat dir, rejected", cmd.Path)
			return
		}
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("handler: sendfile read error for %s: %v", filePath, err)
		return
	}

	log.Printf("handler: sending file %s (%d bytes) to chat=%d thread=%d", cmd.Path, len(data), chatID, threadID)

	_, err = b.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Document:        &models.InputFileUpload{Filename: filepath.Base(cmd.Path), Data: bytes.NewReader(data)},
		Caption:         cmd.Caption,
	})
	if err != nil {
		log.Printf("handler: sendfile send error: %v", err)
	}
}
