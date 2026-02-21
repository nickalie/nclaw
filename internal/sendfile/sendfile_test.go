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

func TestProcessReply_NoMatch(t *testing.T) {
	result := ProcessReply(context.TODO(), nil, "plain reply", 0, 0, "")
	assert.Equal(t, "plain reply", result)
}

func TestProcessReply_StripsBlocks(t *testing.T) {
	reply := "before\n```nclaw:sendfile\n{\"path\":\"/nonexistent\",\"caption\":\"test\"}\n```\nafter"
	// send will fail (file doesn't exist) but the block should still be stripped.
	result := ProcessReply(context.TODO(), noopSendDoc, reply, 0, 0, "")
	assert.Equal(t, "before\n\nafter", result)
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

func TestProcessReply_SendsFile(t *testing.T) {
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

	reply := "text\n```nclaw:sendfile\n{\"path\":\"test.txt\",\"caption\":\"cap\"}\n```\nmore"
	result := ProcessReply(context.TODO(), sendDoc, reply, 42, 0, dir)

	assert.True(t, sent)
	assert.Equal(t, "text\n\nmore", result)
}

func TestProcessReply_SendsFileAbsolutePath(t *testing.T) {
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

	reply := fmt.Sprintf("```nclaw:sendfile\n{\"path\":%q}\n```", filePath)
	result := ProcessReply(context.TODO(), sendDoc, reply, 1, 0, dir)

	assert.True(t, sent)
	assert.Empty(t, result)
}

func TestProcessReply_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	var called bool
	sendDoc := func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
		called = true
		return nil
	}

	reply := "```nclaw:sendfile\n{\"path\":\"../../../etc/passwd\"}\n```"
	result := ProcessReply(context.TODO(), sendDoc, reply, 1, 0, dir)

	assert.False(t, called, "sendDoc should not be called for path traversal attempts")
	assert.Empty(t, result)
}

func TestProcessReply_SendDocError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "err.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o644))

	sendDoc := func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
		return fmt.Errorf("telegram API error")
	}

	reply := "before\n```nclaw:sendfile\n{\"path\":\"err.txt\"}\n```\nafter"
	result := ProcessReply(context.TODO(), sendDoc, reply, 1, 0, dir)

	// Block should still be stripped even when sendDoc fails.
	assert.Equal(t, "before\n\nafter", result)
}

func TestProcessReply_InvalidJSON(t *testing.T) {
	reply := "text\n```nclaw:sendfile\n{invalid json}\n```\nmore"
	result := ProcessReply(context.TODO(), noopSendDoc, reply, 1, 0, "")
	assert.Equal(t, "text\n\nmore", result)
}

func TestProcessReply_NilSendDoc(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o644))

	reply := "text\n```nclaw:sendfile\n{\"path\":\"test.txt\"}\n```\nmore"
	result := ProcessReply(context.TODO(), nil, reply, 1, 0, dir)
	assert.Equal(t, "text\n\nmore", result)
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
