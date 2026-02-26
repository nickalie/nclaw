package streamjson

import (
	"encoding/json"
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
