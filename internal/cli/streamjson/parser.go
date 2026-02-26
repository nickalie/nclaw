package streamjson

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"strings"

	"github.com/nickalie/nclaw/internal/cli"
)

// streamEvent represents a single event from stream-json (NDJSON) output.
type streamEvent struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message,omitempty"`
	Result  string          `json:"result,omitempty"`
}

// assistantMessage represents the content of an assistant message.
type assistantMessage struct {
	Content []contentBlock `json:"content"`
}

// contentBlock represents a single content block in an assistant message.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ParseOutput parses stream-json (NDJSON) output and extracts all assistant
// text and the final result into a cli.Result.
func ParseOutput(output []byte) *cli.Result {
	allText, resultText := collectStreamEvents(output)
	fullText := strings.Join(allText, "\n")

	if resultText == "" {
		resultText = fullText
	}

	if fullText == "" && resultText != "" {
		fullText = resultText
	}

	if resultText == "" && len(allText) == 0 {
		text := strings.TrimSpace(string(output))
		return &cli.Result{Text: text, FullText: text}
	}

	return &cli.Result{Text: resultText, FullText: fullText}
}

func collectStreamEvents(output []byte) (allText []string, resultText string) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event streamEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		switch event.Type {
		case "assistant":
			if text := extractAssistantText(event.Message); text != "" {
				allText = append(allText, text)
			}
		case "result":
			resultText = event.Result
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("streamjson: scan error (output may be truncated): %v", err)
	}

	return allText, resultText
}

func extractAssistantText(msg json.RawMessage) string {
	if len(msg) == 0 {
		return ""
	}

	var message assistantMessage
	if err := json.Unmarshal(msg, &message); err != nil {
		return ""
	}

	var parts []string
	for _, block := range message.Content {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}

	return strings.Join(parts, "\n")
}
