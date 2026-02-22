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

// SendDocFunc sends a single document to a Telegram chat/thread.
type SendDocFunc func(ctx context.Context, chatID int64, threadID int, filename string, data []byte, caption string) error

// File holds resolved file data ready to send.
type File struct {
	Filename string
	Data     []byte
	Caption  string
}

// SendMediaGroupFunc sends a group of files as a single Telegram media group.
type SendMediaGroupFunc func(ctx context.Context, chatID int64, threadID int, files []File) error

const maxMediaGroupSize = 10

var blockRe = regexp.MustCompile("(?s)```nclaw:sendfile\n(.*?)\n```")

type command struct {
	Path    string `json:"path"`
	Caption string `json:"caption"`
}

// StripBlocks removes nclaw:sendfile code blocks from text without processing them.
func StripBlocks(text string) string {
	return strings.TrimSpace(blockRe.ReplaceAllString(text, ""))
}

// ExecuteBlocks extracts nclaw:sendfile blocks from text and sends the files.
// When sendMediaGroup is provided and there are 2+ files, files are grouped into media groups.
// Does not modify the input text.
func ExecuteBlocks(
	ctx context.Context, sendDoc SendDocFunc, sendMediaGroup SendMediaGroupFunc,
	text string, chatID int64, threadID int, dir string,
) {
	matches := blockRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return
	}

	var files []File
	for _, match := range matches {
		f, ok := resolveFile(match[1], dir)
		if ok {
			files = append(files, f)
		}
	}

	if len(files) == 0 {
		return
	}

	if len(files) == 1 || sendMediaGroup == nil {
		sendFilesIndividually(ctx, sendDoc, files, chatID, threadID)
		return
	}

	sendFilesAsGroups(ctx, sendMediaGroup, files, chatID, threadID)
}

func resolveFile(jsonStr, dir string) (File, bool) {
	var cmd command
	if err := json.Unmarshal([]byte(jsonStr), &cmd); err != nil {
		log.Printf("sendfile: invalid JSON: %v", err)
		return File{}, false
	}

	filePath := cmd.Path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(dir, filePath)
	}
	filePath = filepath.Clean(filePath)
	resolved, err := filepath.EvalSymlinks(filePath)
	if err != nil {
		log.Printf("sendfile: resolve error for %s: %v", filePath, err)
		return File{}, false
	}
	if !isAllowedPath(resolved, dir) {
		log.Printf("sendfile: path %q escapes allowed directories, rejected", cmd.Path)
		return File{}, false
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		log.Printf("sendfile: read error for %s: %v", filePath, err)
		return File{}, false
	}

	log.Printf("sendfile: resolved %s (%d bytes)", cmd.Path, len(data))
	return File{Filename: filepath.Base(cmd.Path), Data: data, Caption: cmd.Caption}, true
}

func sendFilesIndividually(ctx context.Context, sendDoc SendDocFunc, files []File, chatID int64, threadID int) {
	if sendDoc == nil {
		log.Printf("sendfile: sendDoc callback is nil, skipping")
		return
	}
	for _, f := range files {
		log.Printf("sendfile: sending %s (%d bytes) to chat=%d thread=%d", f.Filename, len(f.Data), chatID, threadID)
		if err := sendDoc(ctx, chatID, threadID, f.Filename, f.Data, f.Caption); err != nil {
			log.Printf("sendfile: send error: %v", err)
		}
	}
}

func sendFilesAsGroups(ctx context.Context, sendMediaGroup SendMediaGroupFunc, files []File, chatID int64, threadID int) {
	for i := 0; i < len(files); i += maxMediaGroupSize {
		end := i + maxMediaGroupSize
		if end > len(files) {
			end = len(files)
		}
		batch := files[i:end]

		names := make([]string, len(batch))
		for j, f := range batch {
			names[j] = f.Filename
		}
		log.Printf("sendfile: sending media group %v to chat=%d thread=%d", names, chatID, threadID)

		if err := sendMediaGroup(ctx, chatID, threadID, batch); err != nil {
			log.Printf("sendfile: media group send error: %v", err)
		}
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
