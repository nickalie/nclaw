package gemini

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"strings"

	"github.com/nickalie/nclaw/internal/cli"
)

// streamEvent represents a single NDJSON event from `gemini --output-format stream-json`.
// Gemini emits events: init, message, tool_use, tool_result, error, result.
// We only care about "message" (role=assistant) for content extraction.
type streamEvent struct {
	Type string `json:"type"`
	Role string `json:"role,omitempty"`
	// Content is a flat string (unlike Claude's array of content blocks).
	Content string `json:"content,omitempty"`
}

// parseStreamJSONOutput parses Gemini's stream-json NDJSON output and extracts
// assistant messages into a cli.Result.
// Text = last assistant message, FullText = all assistant messages joined by newlines.
func parseStreamJSONOutput(output []byte) *cli.Result {
	messages := collectAssistantMessages(output)

	if len(messages) == 0 {
		text := strings.TrimSpace(string(output))
		return &cli.Result{Text: text, FullText: text}
	}

	fullText := strings.Join(messages, "\n")
	lastMessage := messages[len(messages)-1]

	return &cli.Result{Text: lastMessage, FullText: fullText}
}

// collectAssistantMessages scans NDJSON lines and groups consecutive assistant
// message events into logical messages. Gemini may emit multiple "message" chunks
// for a single assistant turn; these are concatenated. A new logical message starts
// when a non-assistant event (tool_use, tool_result, etc.) appears between assistant events.
func collectAssistantMessages(output []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var messages []string
	var current strings.Builder

	for scanner.Scan() {
		line := scanner.Bytes()
		text := extractAssistantContent(line)

		if text != "" {
			current.WriteString(text)
		} else if current.Len() > 0 && isNonAssistantEvent(line) {
			messages = append(messages, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		messages = append(messages, current.String())
	}

	if err := scanner.Err(); err != nil {
		log.Printf("gemini: stream-json scan error (output may be truncated): %v", err)
	}

	return messages
}

// isNonAssistantEvent returns true if the line is a valid NDJSON event that is
// not an assistant message (e.g. tool_use, tool_result, user message).
// Empty lines and malformed JSON return false to avoid splitting on noise.
func isNonAssistantEvent(line []byte) bool {
	if len(line) == 0 {
		return false
	}

	var event streamEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return false
	}

	if event.Type == "" {
		return false
	}

	return event.Type != "message" || event.Role != "assistant"
}

// extractAssistantContent parses a single NDJSON line and returns the assistant
// message content, or empty string if the line is not an assistant message.
func extractAssistantContent(line []byte) string {
	if len(line) == 0 {
		return ""
	}

	var event streamEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return ""
	}

	if event.Type == "message" && event.Role == "assistant" && event.Content != "" {
		return event.Content
	}

	return ""
}
