package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
)

// Result holds the output from a Claude CLI invocation.
type Result struct {
	// Text is the final assistant message (suitable for display).
	Text string
	// FullText contains all assistant messages concatenated.
	// Useful for scanning command blocks (sendfile, schedule, webhook)
	// that may appear in non-final messages during multi-turn execution.
	FullText string
}

// streamEvent represents a single event from Claude CLI stream-json output.
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

// parseStreamOutput parses stream-json (NDJSON) output from Claude CLI
// and extracts all assistant text and the final result.
func parseStreamOutput(output []byte) *Result {
	allText, resultText := collectStreamEvents(output)
	fullText := strings.Join(allText, "\n")

	if resultText == "" {
		resultText = fullText
	}

	// Ensure FullText always contains at least what Text contains,
	// e.g. when a result event exists but no assistant events were emitted.
	if fullText == "" && resultText != "" {
		fullText = resultText
	}

	// If nothing was parsed (not NDJSON), treat raw output as plain text.
	if resultText == "" && len(allText) == 0 {
		text := strings.TrimSpace(string(output))
		return &Result{Text: text, FullText: text}
	}

	return &Result{Text: resultText, FullText: fullText}
}

// collectStreamEvents scans NDJSON lines and collects assistant text and the result.
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

	return allText, resultText
}

// extractAssistantText extracts text content from an assistant message JSON.
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
