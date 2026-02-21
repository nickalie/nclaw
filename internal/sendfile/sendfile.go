package sendfile

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SendDocFunc sends a document to a Telegram chat/thread.
type SendDocFunc func(ctx context.Context, chatID int64, threadID int, filename string, data []byte, caption string) error

var blockRe = regexp.MustCompile("(?s)```nclaw:sendfile\n(.*?)\n```")

type command struct {
	Path    string `json:"path"`
	Caption string `json:"caption"`
}

// StripBlocks removes nclaw:sendfile code blocks from text without processing them.
func StripBlocks(text string) string {
	return strings.TrimSpace(blockRe.ReplaceAllString(text, ""))
}

// ProcessReply extracts nclaw:sendfile blocks, sends files via sendDoc, and returns cleaned text.
func ProcessReply(ctx context.Context, sendDoc SendDocFunc, reply string, chatID int64, threadID int, dir string) string {
	matches := blockRe.FindAllStringSubmatch(reply, -1)
	if len(matches) == 0 {
		return reply
	}

	for _, match := range matches {
		send(ctx, sendDoc, match[1], chatID, threadID, dir)
	}

	cleaned := blockRe.ReplaceAllString(reply, "")
	return strings.TrimSpace(cleaned)
}

func send(ctx context.Context, sendDoc SendDocFunc, jsonStr string, chatID int64, threadID int, dir string) {
	if sendDoc == nil {
		log.Printf("sendfile: sendDoc callback is nil, skipping")
		return
	}

	var cmd command
	if err := json.Unmarshal([]byte(jsonStr), &cmd); err != nil {
		log.Printf("sendfile: invalid JSON: %v", err)
		return
	}

	filePath := cmd.Path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(dir, filePath)
	}
	filePath = filepath.Clean(filePath)
	resolved, err := filepath.EvalSymlinks(filePath)
	if err != nil {
		log.Printf("sendfile: resolve error for %s: %v", filePath, err)
		return
	}
	if !isAllowedPath(resolved, dir) {
		log.Printf("sendfile: path %q escapes allowed directories, rejected", cmd.Path)
		return
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		log.Printf("sendfile: read error for %s: %v", filePath, err)
		return
	}

	log.Printf("sendfile: sending %s (%d bytes) to chat=%d thread=%d", cmd.Path, len(data), chatID, threadID)

	if err := sendDoc(ctx, chatID, threadID, filepath.Base(cmd.Path), data, cmd.Caption); err != nil {
		log.Printf("sendfile: send error: %v", err)
	}
}

// isAllowedPath checks whether resolved path is inside the chat dir or the OS temp dir.
func isAllowedPath(resolved, chatDir string) bool {
	for _, allowed := range []string{chatDir, os.TempDir()} {
		allowedResolved, err := filepath.EvalSymlinks(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(resolved, allowedResolved+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
