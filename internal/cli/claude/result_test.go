package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStreamOutput_MultiTurn(t *testing.T) {
	// Simulates multi-turn execution where sendfile block is in an intermediate message.
	output := `{"type":"system","subtype":"init","session_id":"abc"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Here is the file.\n` + "```" + `nclaw:sendfile\n{\"path\":\"report.pdf\"}\n` + "```" + `"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Write"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"Done! The file has been sent."}]}}
{"type":"result","result":"Done! The file has been sent.","session_id":"abc"}`

	result := parseStreamOutput([]byte(output))

	assert.Equal(t, "Done! The file has been sent.", result.Text)
	assert.Contains(t, result.FullText, "nclaw:sendfile")
	assert.Contains(t, result.FullText, "report.pdf")
	assert.Contains(t, result.FullText, "Done! The file has been sent.")
}

func TestParseStreamOutput_SingleTurn(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world!"}]}}
{"type":"result","result":"Hello world!","session_id":"abc"}`

	result := parseStreamOutput([]byte(output))

	assert.Equal(t, "Hello world!", result.Text)
	assert.Equal(t, "Hello world!", result.FullText)
}

func TestParseStreamOutput_Empty(t *testing.T) {
	result := parseStreamOutput([]byte(""))

	assert.Equal(t, "", result.Text)
	assert.Equal(t, "", result.FullText)
}

func TestParseStreamOutput_PlainText(t *testing.T) {
	// Fallback: if output is not NDJSON, treat as plain text.
	result := parseStreamOutput([]byte("Just plain text"))

	assert.Equal(t, "Just plain text", result.Text)
	assert.Equal(t, "Just plain text", result.FullText)
}

func TestParseStreamOutput_NoResultEvent(t *testing.T) {
	// If there's no result event, FullText is used as Text.
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"World"}]}}`

	result := parseStreamOutput([]byte(output))

	assert.Equal(t, "Hello\nWorld", result.Text)
	assert.Equal(t, "Hello\nWorld", result.FullText)
}

func TestParseStreamOutput_ToolUseBlocksIgnored(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Let me check."},{"type":"tool_use","id":"t1","name":"Read","input":{}}]}}
{"type":"result","result":"All done.","session_id":"abc"}`

	result := parseStreamOutput([]byte(output))

	assert.Equal(t, "All done.", result.Text)
	assert.Equal(t, "Let me check.", result.FullText)
}

func TestParseStreamOutput_MultipleTextBlocks(t *testing.T) {
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Part 1"},{"type":"text","text":"Part 2"}]}}
{"type":"result","result":"Final","session_id":"abc"}`

	result := parseStreamOutput([]byte(output))

	assert.Equal(t, "Final", result.Text)
	assert.Equal(t, "Part 1\nPart 2", result.FullText)
}

func TestParseStreamOutput_ResultOnlyNoAssistant(t *testing.T) {
	// Edge case: result event exists but no assistant events were emitted.
	// FullText should still contain the result text so command blocks are found.
	output := `{"type":"system","subtype":"init","session_id":"abc"}
{"type":"result","result":"Some result with blocks","session_id":"abc"}`

	result := parseStreamOutput([]byte(output))

	assert.Equal(t, "Some result with blocks", result.Text)
	assert.Equal(t, "Some result with blocks", result.FullText)
}

func TestParseStreamOutput_MalformedJSONSkipped(t *testing.T) {
	// Malformed lines should be skipped; valid events before and after are preserved.
	output := `{"type":"assistant","message":{"content":[{"type":"text","text":"Before"}]}}
{this is not valid json}
{"type":"assistant","message":{"content":[{"type":"text","text":"After"}]}}
{"type":"result","result":"Final","session_id":"abc"}`

	result := parseStreamOutput([]byte(output))

	assert.Equal(t, "Final", result.Text)
	assert.Contains(t, result.FullText, "Before")
	assert.Contains(t, result.FullText, "After")
}

func TestExtractAssistantText_EmptyMessage(t *testing.T) {
	assert.Equal(t, "", extractAssistantText(nil))
	assert.Equal(t, "", extractAssistantText([]byte("")))
	assert.Equal(t, "", extractAssistantText([]byte("{}")))
}

func TestExtractAssistantText_TextContent(t *testing.T) {
	msg := []byte(`{"content":[{"type":"text","text":"hello"}]}`)
	assert.Equal(t, "hello", extractAssistantText(msg))
}
