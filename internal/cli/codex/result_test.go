package codex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseJSONLOutput_MultiTurn(t *testing.T) {
	// Simulates multi-turn execution with multiple agent messages.
	output := `{"type":"thread.started","thread_id":"0199a213-81c0-7800-8aa1-bbab2a035a53"}
{"type":"turn.started"}
{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"bash -lc ls","status":"in_progress"}}
{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"bash -lc ls","status":"completed"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"I found the files. Let me process them."}}
{"type":"item.started","item":{"id":"item_3","type":"command_execution","command":"bash -lc cat report.txt","status":"in_progress"}}
{"type":"item.completed","item":{"id":"item_3","type":"command_execution","command":"bash -lc cat report.txt","status":"completed"}}
{"type":"item.completed","item":{"id":"item_4","type":"agent_message","text":"Here is the report summary."}}
{"type":"turn.completed","usage":{"input_tokens":5000,"output_tokens":200}}`

	result := parseJSONLOutput([]byte(output))

	assert.Equal(t, "Here is the report summary.", result.Text)
	assert.Contains(t, result.FullText, "I found the files. Let me process them.")
	assert.Contains(t, result.FullText, "Here is the report summary.")
}

func TestParseJSONLOutput_SingleMessage(t *testing.T) {
	output := `{"type":"thread.started","thread_id":"abc"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Hello world!"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":10}}`

	result := parseJSONLOutput([]byte(output))

	assert.Equal(t, "Hello world!", result.Text)
	assert.Equal(t, "Hello world!", result.FullText)
}

func TestParseJSONLOutput_Empty(t *testing.T) {
	result := parseJSONLOutput([]byte(""))

	assert.Equal(t, "", result.Text)
	assert.Equal(t, "", result.FullText)
}

func TestParseJSONLOutput_PlainText(t *testing.T) {
	// Fallback: if output is not JSONL, treat as plain text.
	result := parseJSONLOutput([]byte("Just plain text"))

	assert.Equal(t, "Just plain text", result.Text)
	assert.Equal(t, "Just plain text", result.FullText)
}

func TestParseJSONLOutput_NoAgentMessages(t *testing.T) {
	// Only non-message events — should fall back to plain text of raw output.
	output := `{"type":"thread.started","thread_id":"abc"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ls"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":10}}`

	result := parseJSONLOutput([]byte(output))

	// Falls back to raw output as plain text since no agent messages found.
	assert.Contains(t, result.Text, "thread.started")
}

func TestParseJSONLOutput_CommandBlocksInMiddle(t *testing.T) {
	// Simulates command blocks appearing in intermediate messages.
	output := `{"type":"thread.started","thread_id":"abc"}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Here is the file.\n` + "```" + `nclaw:sendfile\n{\"path\":\"report.pdf\"}\n` + "```" + `"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"Done! The file has been sent."}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":10}}`

	result := parseJSONLOutput([]byte(output))

	assert.Equal(t, "Done! The file has been sent.", result.Text)
	assert.Contains(t, result.FullText, "nclaw:sendfile")
	assert.Contains(t, result.FullText, "report.pdf")
	assert.Contains(t, result.FullText, "Done! The file has been sent.")
}

func TestParseJSONLOutput_MalformedJSONSkipped(t *testing.T) {
	output := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Before"}}
{this is not valid json}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"After"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":10}}`

	result := parseJSONLOutput([]byte(output))

	assert.Equal(t, "After", result.Text)
	assert.Contains(t, result.FullText, "Before")
	assert.Contains(t, result.FullText, "After")
}

func TestParseJSONLOutput_ItemStartedIgnored(t *testing.T) {
	// item.started events should not be captured as messages.
	output := `{"type":"item.started","item":{"id":"item_1","type":"agent_message","text":"Starting..."}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Done."}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":10}}`

	result := parseJSONLOutput([]byte(output))

	assert.Equal(t, "Done.", result.Text)
	assert.Equal(t, "Done.", result.FullText)
}

func TestParseJSONLOutput_EmptyAgentMessageSkipped(t *testing.T) {
	output := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":""}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"Actual message."}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":10}}`

	result := parseJSONLOutput([]byte(output))

	assert.Equal(t, "Actual message.", result.Text)
	assert.Equal(t, "Actual message.", result.FullText)
}

func TestCollectAgentMessages_EmptyInput(t *testing.T) {
	messages := collectAgentMessages([]byte(""))
	assert.Empty(t, messages)
}

func TestCollectAgentMessages_OnlyAgentMessages(t *testing.T) {
	output := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"First"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"Second"}}
{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Third"}}`

	messages := collectAgentMessages([]byte(output))

	assert.Equal(t, []string{"First", "Second", "Third"}, messages)
}

func TestCollectAgentMessages_MixedItemTypes(t *testing.T) {
	output := `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ls"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"Found files."}}
{"type":"item.completed","item":{"id":"item_3","type":"reasoning","text":"thinking..."}}
{"type":"item.completed","item":{"id":"item_4","type":"agent_message","text":"Done."}}`

	messages := collectAgentMessages([]byte(output))

	assert.Equal(t, []string{"Found files.", "Done."}, messages)
}
