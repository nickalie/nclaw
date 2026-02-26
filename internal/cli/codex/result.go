package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"strings"

	"github.com/nickalie/nclaw/internal/cli"
)

// jsonlEvent represents a single JSONL event from the Codex CLI --json output.
type jsonlEvent struct {
	Type string     `json:"type"`
	Item *jsonlItem `json:"item,omitempty"`
}

// jsonlItem represents an item within a Codex JSONL event.
type jsonlItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// parseJSONLOutput parses JSONL output from Codex CLI --json flag
// and extracts all agent messages for FullText and the last agent message for Text.
func parseJSONLOutput(output []byte) *cli.Result {
	messages := collectAgentMessages(output)

	if len(messages) == 0 {
		// Not JSONL or no agent messages — treat raw output as plain text.
		text := strings.TrimSpace(string(output))
		return &cli.Result{Text: text, FullText: text}
	}

	fullText := strings.Join(messages, "\n")
	lastMessage := messages[len(messages)-1]

	return &cli.Result{Text: lastMessage, FullText: fullText}
}

// collectAgentMessages scans JSONL lines for item.completed events
// with type "agent_message" and returns their text content.
func collectAgentMessages(output []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var messages []string

	for scanner.Scan() {
		if text := extractAgentMessage(scanner.Bytes()); text != "" {
			messages = append(messages, text)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("codex: JSONL scan error (output may be truncated): %v", err)
	}

	return messages
}

// extractAgentMessage parses a single JSONL line and returns the agent message text,
// or empty string if the line is not a completed agent message.
func extractAgentMessage(line []byte) string {
	if len(line) == 0 {
		return ""
	}

	var event jsonlEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return ""
	}

	if event.Type == "item.completed" && event.Item != nil &&
		event.Item.Type == "agent_message" {
		return event.Item.Text
	}

	return ""
}
