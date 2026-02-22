package sendfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var noopSendDoc SendDocFunc = func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
	return nil
}

func TestExecuteBlocks_NoMatch(t *testing.T) {
	// Should not panic with nil sendDoc when there are no blocks.
	ExecuteBlocks(context.TODO(), nil, "plain reply", 0, 0, "")
}

func TestExecuteBlocks_SendsFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o644))

	var sent bool
	sendDoc := func(_ context.Context, chatID int64, threadID int, filename string, data []byte, caption string) error {
		sent = true
		assert.Equal(t, int64(42), chatID)
		assert.Equal(t, 0, threadID)
		assert.Equal(t, "test.txt", filename)
		assert.Equal(t, []byte("hello"), data)
		assert.Equal(t, "cap", caption)
		return nil
	}

	text := "text\n```nclaw:sendfile\n{\"path\":\"test.txt\",\"caption\":\"cap\"}\n```\nmore"
	ExecuteBlocks(context.TODO(), sendDoc, text, 42, 0, dir)

	assert.True(t, sent)
	assert.Equal(t, "text\n\nmore", StripBlocks(text))
}

func TestExecuteBlocks_SendsFileAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "abs.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("absolute"), 0o644))

	var sent bool
	sendDoc := func(_ context.Context, _ int64, _ int, filename string, data []byte, _ string) error {
		sent = true
		assert.Equal(t, "abs.txt", filename)
		assert.Equal(t, []byte("absolute"), data)
		return nil
	}

	text := fmt.Sprintf("```nclaw:sendfile\n{\"path\":%q}\n```", filePath)
	ExecuteBlocks(context.TODO(), sendDoc, text, 1, 0, dir)

	assert.True(t, sent)
	assert.Empty(t, StripBlocks(text))
}

func TestExecuteBlocks_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	var called bool
	sendDoc := func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
		called = true
		return nil
	}

	text := "```nclaw:sendfile\n{\"path\":\"../../../etc/passwd\"}\n```"
	ExecuteBlocks(context.TODO(), sendDoc, text, 1, 0, dir)

	assert.False(t, called, "sendDoc should not be called for path traversal attempts")
}

func TestExecuteBlocks_SendDocError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "err.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o644))

	sendDoc := func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
		return fmt.Errorf("telegram API error")
	}

	text := "before\n```nclaw:sendfile\n{\"path\":\"err.txt\"}\n```\nafter"
	// Should not panic on sendDoc error.
	ExecuteBlocks(context.TODO(), sendDoc, text, 1, 0, dir)

	// Block should still be stripped by StripBlocks.
	assert.Equal(t, "before\n\nafter", StripBlocks(text))
}

func TestExecuteBlocks_InvalidJSON(t *testing.T) {
	text := "text\n```nclaw:sendfile\n{invalid json}\n```\nmore"
	// Should not panic on invalid JSON.
	ExecuteBlocks(context.TODO(), noopSendDoc, text, 1, 0, "")
	assert.Equal(t, "text\n\nmore", StripBlocks(text))
}

func TestExecuteBlocks_NilSendDoc(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o644))

	text := "text\n```nclaw:sendfile\n{\"path\":\"test.txt\"}\n```\nmore"
	// Should not panic with nil sendDoc.
	ExecuteBlocks(context.TODO(), nil, text, 1, 0, dir)
	assert.Equal(t, "text\n\nmore", StripBlocks(text))
}

func TestIsAllowedPath(t *testing.T) {
	chatDir := t.TempDir()
	tmpDir := os.TempDir()

	// File inside chat dir is allowed.
	assert.True(t, isAllowedPath(filepath.Join(chatDir, "file.txt"), chatDir))

	// File inside OS temp dir is allowed.
	assert.True(t, isAllowedPath(filepath.Join(tmpDir, "file.txt"), chatDir))

	// File outside both dirs is rejected.
	assert.False(t, isAllowedPath("/etc/passwd", chatDir))
}

func TestBlockRegex(t *testing.T) {
	input := "text\n```nclaw:sendfile\n{\"path\":\"file.txt\"}\n```\nmore"
	matches := blockRe.FindAllStringSubmatch(input, -1)
	assert.Len(t, matches, 1)
	assert.Equal(t, "{\"path\":\"file.txt\"}", matches[0][1])
}

func TestBlockRegex_Multiple(t *testing.T) {
	input := "```nclaw:sendfile\n{\"path\":\"a.txt\"}\n```\nmiddle\n```nclaw:sendfile\n{\"path\":\"b.txt\"}\n```"
	matches := blockRe.FindAllStringSubmatch(input, -1)
	assert.Len(t, matches, 2)
	assert.Equal(t, "{\"path\":\"a.txt\"}", matches[0][1])
	assert.Equal(t, "{\"path\":\"b.txt\"}", matches[1][1])
}

func TestStripBlocks(t *testing.T) {
	reply := "before\n```nclaw:sendfile\n{\"path\":\"file.txt\"}\n```\nafter"
	result := StripBlocks(reply)
	assert.Equal(t, "before\n\nafter", result)
}

func TestStripBlocks_NoBlocks(t *testing.T) {
	result := StripBlocks("plain text")
	assert.Equal(t, "plain text", result)
}
