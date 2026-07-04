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

	if resultText == "" && len(allText) == 0 {
		text := strings.TrimSpace(string(output))
		return &cli.Result{Text: text, FullText: text}
	}

	return assembleResult(allText, resultText)
}

// assembleResult builds a cli.Result from collected assistant texts and the
// final result-event text, applying the fallbacks shared by ParseOutput and
// StreamWriter.Result.
func assembleResult(allText []string, resultText string) *cli.Result {
	fullText := strings.Join(allText, "\n")

	if resultText == "" {
		resultText = fullText
	}

	if fullText == "" && resultText != "" {
		fullText = resultText
	}

	return &cli.Result{Text: resultText, FullText: fullText, Messages: allText}
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

// StreamWriter is an io.Writer that parses stream-json (NDJSON) output
// incrementally as it is written, invoking onMessage for each complete assistant
// message and accumulating the parsed result. Result assembles the final
// cli.Result from the accumulated state, so each line is parsed exactly once.
type StreamWriter struct {
	onMessage  func(string)
	pending    []byte
	raw        bytes.Buffer
	messages   []string
	resultText string
}

// NewStreamWriter creates a StreamWriter that calls onMessage for each assistant
// message as it streams in. onMessage may be nil, in which case the writer only
// accumulates output for a final Result.
func NewStreamWriter(onMessage func(string)) *StreamWriter {
	return &StreamWriter{onMessage: onMessage}
}

// Write captures raw bytes and processes complete NDJSON lines. It always reports
// the full length as written so the process is never blocked.
func (w *StreamWriter) Write(p []byte) (int, error) {
	w.raw.Write(p)
	w.pending = append(w.pending, p...)

	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := w.pending[:i]
		w.pending = w.pending[i+1:]
		w.handleLine(line)
	}

	return len(p), nil
}

// Bytes returns all raw output written so far, used for error fallback text.
func (w *StreamWriter) Bytes() []byte {
	return w.raw.Bytes()
}

// Result flushes any unterminated final line and assembles the complete
// cli.Result from the accumulated assistant messages and result text.
func (w *StreamWriter) Result() *cli.Result {
	if len(w.pending) > 0 {
		w.handleLine(w.pending)
		w.pending = nil
	}

	if w.resultText == "" && len(w.messages) == 0 {
		text := strings.TrimSpace(w.raw.String())
		return &cli.Result{Text: text, FullText: text}
	}

	return assembleResult(w.messages, w.resultText)
}

// handleLine parses a single NDJSON line, accumulating assistant text (also
// emitted live via onMessage) and the final result text.
func (w *StreamWriter) handleLine(line []byte) {
	if len(line) == 0 {
		return
	}

	var event streamEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return
	}

	switch event.Type {
	case "assistant":
		if text := extractAssistantText(event.Message); text != "" {
			w.messages = append(w.messages, text)
			if w.onMessage != nil {
				w.onMessage(text)
			}
		}
	case "result":
		w.resultText = event.Result
	}
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
