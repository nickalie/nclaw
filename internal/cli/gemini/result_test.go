package gemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStreamJSONOutput_MultiTurn(t *testing.T) {
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"user","content":"List the files"}
{"type":"tool_use","timestamp":"2025-01-01T00:00:02Z","tool_name":"shell","tool_id":"t1","parameters":{"command":"ls"}}
{"type":"tool_result","timestamp":"2025-01-01T00:00:03Z","tool_id":"t1","status":"success","output":"file1.txt\nfile2.txt"}
{"type":"message","timestamp":"2025-01-01T00:00:04Z","role":"assistant","content":"I found the files. Let me process them."}
{"type":"tool_use","timestamp":"2025-01-01T00:00:05Z","tool_name":"shell","tool_id":"t2","parameters":{"command":"cat report.txt"}}
{"type":"tool_result","timestamp":"2025-01-01T00:00:06Z","tool_id":"t2","status":"success","output":"report content"}
{"type":"message","timestamp":"2025-01-01T00:00:07Z","role":"assistant","content":"Here is the report summary."}
{"type":"result","timestamp":"2025-01-01T00:00:08Z","status":"success","stats":{"total_tokens":500}}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "Here is the report summary.", result.Text)
	assert.Contains(t, result.FullText, "I found the files. Let me process them.")
	assert.Contains(t, result.FullText, "Here is the report summary.")
}

func TestParseStreamJSONOutput_SingleMessage(t *testing.T) {
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Hello world!"}
{"type":"result","timestamp":"2025-01-01T00:00:02Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "Hello world!", result.Text)
	assert.Equal(t, "Hello world!", result.FullText)
}

func TestParseStreamJSONOutput_Empty(t *testing.T) {
	result := parseStreamJSONOutput([]byte(""))

	assert.Equal(t, "", result.Text)
	assert.Equal(t, "", result.FullText)
}

func TestParseStreamJSONOutput_PlainText(t *testing.T) {
	result := parseStreamJSONOutput([]byte("Just plain text"))

	assert.Equal(t, "Just plain text", result.Text)
	assert.Equal(t, "Just plain text", result.FullText)
}

func TestParseStreamJSONOutput_NoAssistantMessages(t *testing.T) {
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"user","content":"Hello"}
{"type":"tool_use","timestamp":"2025-01-01T00:00:02Z","tool_name":"shell","tool_id":"t1","parameters":{}}
{"type":"result","timestamp":"2025-01-01T00:00:03Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	// Falls back to raw output as plain text since no assistant messages found.
	assert.Contains(t, result.Text, "init")
}

func TestParseStreamJSONOutput_CommandBlocksInMiddle(t *testing.T) {
	// Two assistant messages separated by a tool_use event are distinct turns.
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Here is the file.\n` + "```" + `nclaw:sendfile\n{\"path\":\"report.pdf\"}\n` + "```" + `"}
{"type":"tool_use","timestamp":"2025-01-01T00:00:02Z","tool_name":"sendfile","tool_id":"t1","parameters":{}}
{"type":"message","timestamp":"2025-01-01T00:00:03Z","role":"assistant","content":"Done! The file has been sent."}
{"type":"result","timestamp":"2025-01-01T00:00:04Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "Done! The file has been sent.", result.Text)
	assert.Contains(t, result.FullText, "nclaw:sendfile")
	assert.Contains(t, result.FullText, "report.pdf")
	assert.Contains(t, result.FullText, "Done! The file has been sent.")
}

func TestParseStreamJSONOutput_MalformedJSONSkipped(t *testing.T) {
	// Malformed JSON between assistant messages does not split them (treated as noise).
	output := `{"type":"message","timestamp":"2025-01-01T00:00:00Z","role":"assistant","content":"Before"}
{this is not valid json}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"After"}
{"type":"result","timestamp":"2025-01-01T00:00:02Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "BeforeAfter", result.Text)
	assert.Equal(t, "BeforeAfter", result.FullText)
}

func TestParseStreamJSONOutput_UserMessagesIgnored(t *testing.T) {
	output := `{"type":"message","timestamp":"2025-01-01T00:00:00Z","role":"user","content":"What is Go?"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Go is a programming language."}
{"type":"result","timestamp":"2025-01-01T00:00:02Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "Go is a programming language.", result.Text)
	assert.Equal(t, "Go is a programming language.", result.FullText)
}

func TestParseStreamJSONOutput_EmptyContentSkipped(t *testing.T) {
	output := `{"type":"message","timestamp":"2025-01-01T00:00:00Z","role":"assistant","content":""}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Actual message."}
{"type":"result","timestamp":"2025-01-01T00:00:02Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "Actual message.", result.Text)
	assert.Equal(t, "Actual message.", result.FullText)
}

func TestParseStreamJSONOutput_ErrorEvent(t *testing.T) {
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Trying something..."}
{"type":"error","timestamp":"2025-01-01T00:00:02Z","severity":"warning","message":"rate limited"}
{"type":"message","timestamp":"2025-01-01T00:00:03Z","role":"assistant","content":"Done after retry."}
{"type":"result","timestamp":"2025-01-01T00:00:04Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "Done after retry.", result.Text)
	assert.Contains(t, result.FullText, "Trying something...")
	assert.Contains(t, result.FullText, "Done after retry.")
}

func TestCollectAssistantMessages_EmptyInput(t *testing.T) {
	messages := collectAssistantMessages([]byte(""))
	assert.Empty(t, messages)
}

func TestCollectAssistantMessages_OnlyAssistantMessages(t *testing.T) {
	// Consecutive assistant messages with no intervening events are concatenated
	// as chunks of the same logical message.
	output := `{"type":"message","timestamp":"2025-01-01T00:00:00Z","role":"assistant","content":"First"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Second"}
{"type":"message","timestamp":"2025-01-01T00:00:02Z","role":"assistant","content":"Third"}`

	messages := collectAssistantMessages([]byte(output))

	assert.Equal(t, []string{"FirstSecondThird"}, messages)
}

func TestCollectAssistantMessages_MixedEventTypes(t *testing.T) {
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"user","content":"do it"}
{"type":"tool_use","timestamp":"2025-01-01T00:00:02Z","tool_name":"shell","tool_id":"t1","parameters":{}}
{"type":"message","timestamp":"2025-01-01T00:00:03Z","role":"assistant","content":"Found files."}
{"type":"tool_result","timestamp":"2025-01-01T00:00:04Z","tool_id":"t1","status":"success","output":"ok"}
{"type":"message","timestamp":"2025-01-01T00:00:05Z","role":"assistant","content":"Done."}`

	messages := collectAssistantMessages([]byte(output))

	assert.Equal(t, []string{"Found files.", "Done."}, messages)
}

func TestExtractAssistantContent_EmptyLine(t *testing.T) {
	assert.Equal(t, "", extractAssistantContent(nil))
	assert.Equal(t, "", extractAssistantContent([]byte("")))
}

func TestExtractAssistantContent_AssistantMessage(t *testing.T) {
	line := `{"type":"message","role":"assistant","content":"hello"}`
	assert.Equal(t, "hello", extractAssistantContent([]byte(line)))
}

func TestExtractAssistantContent_UserMessage(t *testing.T) {
	line := `{"type":"message","role":"user","content":"hello"}`
	assert.Equal(t, "", extractAssistantContent([]byte(line)))
}

func TestExtractAssistantContent_NonMessageEvent(t *testing.T) {
	line := `{"type":"init","session_id":"abc","model":"gemini-2.5-pro"}`
	assert.Equal(t, "", extractAssistantContent([]byte(line)))
}

func TestExtractAssistantContent_InvalidJSON(t *testing.T) {
	assert.Equal(t, "", extractAssistantContent([]byte("{not json}")))
}

func TestParseStreamJSONOutput_ChunkedAssistantMessage(t *testing.T) {
	// Gemini may emit multiple message chunks for a single assistant turn.
	// Consecutive chunks must be concatenated into a single logical message.
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Hello "}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"world! "}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"How are you?"}
{"type":"result","timestamp":"2025-01-01T00:00:02Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "Hello world! How are you?", result.Text)
	assert.Equal(t, "Hello world! How are you?", result.FullText)
}

func TestParseStreamJSONOutput_ChunkedMultiTurn(t *testing.T) {
	// Multiple assistant turns, each with chunked messages.
	// Chunks within a turn are concatenated; turns are separated by tool events.
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Let me "}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"check that."}
{"type":"tool_use","timestamp":"2025-01-01T00:00:02Z","tool_name":"shell","tool_id":"t1","parameters":{"command":"ls"}}
{"type":"tool_result","timestamp":"2025-01-01T00:00:03Z","tool_id":"t1","status":"success","output":"file.txt"}
{"type":"message","timestamp":"2025-01-01T00:00:04Z","role":"assistant","content":"Found "}
{"type":"message","timestamp":"2025-01-01T00:00:04Z","role":"assistant","content":"the file."}
{"type":"result","timestamp":"2025-01-01T00:00:05Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Equal(t, "Found the file.", result.Text)
	assert.Equal(t, "Let me check that.\nFound the file.", result.FullText)
}

func TestParseStreamJSONOutput_ChunkedCommandBlock(t *testing.T) {
	// A command block split across chunks must be reconstructed correctly.
	output := `{"type":"init","timestamp":"2025-01-01T00:00:00Z","session_id":"abc","model":"gemini-2.5-pro"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"Here is the file.\n` + "```" + `nclaw:sendfile\n"}
{"type":"message","timestamp":"2025-01-01T00:00:01Z","role":"assistant","content":"{\"path\":\"report.pdf\"}\n` + "```" + `"}
{"type":"result","timestamp":"2025-01-01T00:00:02Z","status":"success"}`

	result := parseStreamJSONOutput([]byte(output))

	assert.Contains(t, result.FullText, "nclaw:sendfile")
	assert.Contains(t, result.FullText, "report.pdf")
}

func TestCollectAssistantMessages_ChunksGroupedByTurn(t *testing.T) {
	output := `{"type":"message","role":"assistant","content":"a"}
{"type":"message","role":"assistant","content":"b"}
{"type":"tool_use","tool_name":"shell","tool_id":"t1","parameters":{}}
{"type":"message","role":"assistant","content":"c"}
{"type":"message","role":"assistant","content":"d"}`

	messages := collectAssistantMessages([]byte(output))

	assert.Equal(t, []string{"ab", "cd"}, messages)
}

func TestIsNonAssistantEvent(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"tool_use", `{"type":"tool_use","tool_name":"shell"}`, true},
		{"tool_result", `{"type":"tool_result","tool_id":"t1","status":"success"}`, true},
		{"user_message", `{"type":"message","role":"user","content":"hi"}`, true},
		{"init", `{"type":"init","session_id":"abc"}`, true},
		{"error", `{"type":"error","severity":"warning"}`, true},
		{"result", `{"type":"result","status":"success"}`, true},
		{"assistant_message", `{"type":"message","role":"assistant","content":"hi"}`, false},
		{"empty_line", ``, false},
		{"malformed_json", `{not json}`, false},
		{"empty_object", `{}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isNonAssistantEvent([]byte(tt.line)))
		})
	}
}
