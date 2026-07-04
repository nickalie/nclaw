package streamjson

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOutput_MultiTurn(t *testing.T) {
	output := `{"type":"system","subtype":"init","session_id":"abc"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Here is the file.\n` + "```" + `nclaw:sendfile\n{\"path\":\"report.pdf\"}\n` + "```" + `"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Write"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"Done! The file has been sent."}]}}
{"type":"result","result":"Done! The file has been sent.","session_id":"abc"}`

	result := ParseOutput([]byte(output))

	assert.Equal(t, "Done! The file has been sent.", result.Text)
	assert.Contains(t, result.FullText, "nclaw:sendfile")
	assert.Contains(t, result.FullText, "report.pdf")
	assert.Contains(t, result.FullText, "Done! The file has been sent.")
}

func TestParseOutput_SingleTurn(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world!"}]}}
{"type":"result","result":"Hello world!","session_id":"abc"}`

	result := ParseOutput([]byte(output))

	assert.Equal(t, "Hello world!", result.Text)
	assert.Equal(t, "Hello world!", result.FullText)
}

func TestParseOutput_Empty(t *testing.T) {
	result := ParseOutput([]byte(""))

	assert.Equal(t, "", result.Text)
	assert.Equal(t, "", result.FullText)
}

func TestParseOutput_PlainText(t *testing.T) {
	result := ParseOutput([]byte("Just plain text"))

	assert.Equal(t, "Just plain text", result.Text)
	assert.Equal(t, "Just plain text", result.FullText)
}

func TestParseOutput_NoResultEvent(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"World"}]}}`

	result := ParseOutput([]byte(output))

	assert.Equal(t, "Hello\nWorld", result.Text)
	assert.Equal(t, "Hello\nWorld", result.FullText)
}

func TestParseOutput_ToolUseBlocksIgnored(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Let me check."},{"type":"tool_use","id":"t1","name":"Read","input":{}}]}}
{"type":"result","result":"All done.","session_id":"abc"}`

	result := ParseOutput([]byte(output))

	assert.Equal(t, "All done.", result.Text)
	assert.Equal(t, "Let me check.", result.FullText)
}

func TestParseOutput_MultipleTextBlocks(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Part 1"},{"type":"text","text":"Part 2"}]}}
{"type":"result","result":"Final","session_id":"abc"}`

	result := ParseOutput([]byte(output))

	assert.Equal(t, "Final", result.Text)
	assert.Equal(t, "Part 1\nPart 2", result.FullText)
}

func TestParseOutput_ResultOnlyNoAssistant(t *testing.T) {
	output := `{"type":"system","subtype":"init","session_id":"abc"}
{"type":"result","result":"Some result with blocks","session_id":"abc"}`

	result := ParseOutput([]byte(output))

	assert.Equal(t, "Some result with blocks", result.Text)
	assert.Equal(t, "Some result with blocks", result.FullText)
}

func TestParseOutput_MalformedJSONSkipped(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Before"}]}}
{this is not valid json}
{"type":"assistant","message":{"content":[{"type":"text","text":"After"}]}}
{"type":"result","result":"Final","session_id":"abc"}`

	result := ParseOutput([]byte(output))

	assert.Equal(t, "Final", result.Text)
	assert.Contains(t, result.FullText, "Before")
	assert.Contains(t, result.FullText, "After")
}

func TestExtractAssistantText_EmptyMessage(t *testing.T) {
	assert.Equal(t, "", extractAssistantText(nil))
	assert.Equal(t, "", extractAssistantText(json.RawMessage("")))
	assert.Equal(t, "", extractAssistantText(json.RawMessage("{}")))
}

func TestExtractAssistantText_TextContent(t *testing.T) {
	msg := json.RawMessage(`{"content":[{"type":"text","text":"hello"}]}`)
	assert.Equal(t, "hello", extractAssistantText(msg))
}

func TestStreamWriter_EmitsAssistantMessages(t *testing.T) {
	var got []string
	w := NewStreamWriter(func(m string) { got = append(got, m) })

	output := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"first"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"second"}]}}
{"type":"result","result":"second"}
`
	n, err := w.Write([]byte(output))
	assert.NoError(t, err)
	assert.Equal(t, len(output), n)
	assert.Equal(t, []string{"first", "second"}, got)
	assert.Equal(t, output, string(w.Bytes()))
}

func TestStreamWriter_ReassemblesFragmentedLines(t *testing.T) {
	var got []string
	w := NewStreamWriter(func(m string) { got = append(got, m) })

	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello world"}]}}` + "\n"
	// Feed one byte at a time to exercise partial-line buffering.
	for i := 0; i < len(line); i++ {
		_, err := w.Write([]byte{line[i]})
		assert.NoError(t, err)
	}

	assert.Equal(t, []string{"hello world"}, got)
}

func TestStreamWriter_NoTrailingNewlineNotEmitted(t *testing.T) {
	var got []string
	w := NewStreamWriter(func(m string) { got = append(got, m) })

	// A final line without a newline is not a complete NDJSON record yet.
	_, _ = w.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"partial"}]}}`))

	assert.Empty(t, got)
	// The pending line is flushed and emitted by Result, not by Write.
	assert.Contains(t, string(w.Bytes()), "partial")
}

func TestStreamWriter_NilCallback(t *testing.T) {
	w := NewStreamWriter(nil)
	_, err := w.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"x"}]}}` + "\n"))
	assert.NoError(t, err)
	assert.Contains(t, string(w.Bytes()), "x")
}

func TestStreamWriter_ResultMatchesParseOutput(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"first"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"second"}]}}
{"type":"result","result":"final"}
`
	w := NewStreamWriter(nil)
	_, err := w.Write([]byte(output))
	assert.NoError(t, err)

	assert.Equal(t, ParseOutput([]byte(output)), w.Result())
}

func TestStreamWriter_ResultFlushesUnterminatedLine(t *testing.T) {
	var got []string
	w := NewStreamWriter(func(m string) { got = append(got, m) })

	_, _ = w.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"first"}]}}` + "\n"))
	// A final complete-but-unterminated line stays pending until Result flushes it.
	_, _ = w.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"last"}]}}`))
	assert.Equal(t, []string{"first"}, got)

	result := w.Result()
	assert.Equal(t, []string{"first", "last"}, got)
	assert.Equal(t, []string{"first", "last"}, result.Messages)
	assert.Contains(t, result.FullText, "last")
}

func TestStreamWriter_ResultLargeLineNotDropped(t *testing.T) {
	var got []string
	w := NewStreamWriter(func(m string) { got = append(got, m) })

	huge := strings.Repeat("a", 2*1024*1024)
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + huge + `"}]}}` + "\n"
	_, err := w.Write([]byte(line))
	assert.NoError(t, err)

	result := w.Result()
	assert.Equal(t, []string{huge}, got)
	assert.Equal(t, []string{huge}, result.Messages)
	assert.Contains(t, result.FullText, huge)
}
