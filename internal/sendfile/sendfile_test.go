package sendfile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var noopSendDoc SendDocFunc = func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
	return nil
}

func TestExecuteBlocks_NoMatch(t *testing.T) {
	// Should not panic with nil sendDoc when there are no blocks.
	ExecuteBlocks(context.TODO(), Senders{}, "plain reply", 0, 0, "")
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
	ExecuteBlocks(context.TODO(), Senders{Doc: sendDoc}, text, 42, 0, dir)

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
	ExecuteBlocks(context.TODO(), Senders{Doc: sendDoc}, text, 1, 0, dir)

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
	ExecuteBlocks(context.TODO(), Senders{Doc: sendDoc}, text, 1, 0, dir)

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
	ExecuteBlocks(context.TODO(), Senders{Doc: sendDoc}, text, 1, 0, dir)

	// Block should still be stripped by StripBlocks.
	assert.Equal(t, "before\n\nafter", StripBlocks(text))
}

func TestExecuteBlocks_InvalidJSON(t *testing.T) {
	text := "text\n```nclaw:sendfile\n{invalid json}\n```\nmore"
	// Should not panic on invalid JSON.
	ExecuteBlocks(context.TODO(), Senders{Doc: noopSendDoc}, text, 1, 0, "")
	assert.Equal(t, "text\n\nmore", StripBlocks(text))
}

func TestExecuteBlocks_NilSendDoc(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o644))

	text := "text\n```nclaw:sendfile\n{\"path\":\"test.txt\"}\n```\nmore"
	// Should not panic with nil sendDoc.
	ExecuteBlocks(context.TODO(), Senders{}, text, 1, 0, dir)
	assert.Equal(t, "text\n\nmore", StripBlocks(text))
}

func TestExecuteBlocks_MediaGroup(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.png"), []byte("img1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.png"), []byte("img2"), 0o644))

	var groupSent bool
	var sentFiles []File
	sendMediaGroup := func(_ context.Context, chatID int64, threadID int, files []File) error {
		groupSent = true
		sentFiles = files
		assert.Equal(t, int64(10), chatID)
		assert.Equal(t, 5, threadID)
		return nil
	}

	text := "```nclaw:sendfile\n{\"path\":\"a.png\",\"caption\":\"first\"}\n```\n" +
		"```nclaw:sendfile\n{\"path\":\"b.png\",\"caption\":\"second\"}\n```"
	ExecuteBlocks(context.TODO(), Senders{MediaGroup: sendMediaGroup}, text, 10, 5, dir)

	assert.True(t, groupSent)
	require.Len(t, sentFiles, 2)
	assert.Equal(t, "a.png", sentFiles[0].Filename)
	assert.Equal(t, "b.png", sentFiles[1].Filename)
	assert.Equal(t, "first", sentFiles[0].Caption)
	assert.Equal(t, "second", sentFiles[1].Caption)
}

func TestExecuteBlocks_SingleFileFallsBackToSendDoc(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "only.txt"), []byte("data"), 0o644))

	var docSent bool
	sendDoc := func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
		docSent = true
		return nil
	}
	var groupCalled bool
	sendMediaGroup := func(_ context.Context, _ int64, _ int, _ []File) error {
		groupCalled = true
		return nil
	}

	text := "```nclaw:sendfile\n{\"path\":\"only.txt\"}\n```"
	ExecuteBlocks(context.TODO(), Senders{Doc: sendDoc, MediaGroup: sendMediaGroup}, text, 1, 0, dir)

	assert.True(t, docSent, "single file should use sendDoc")
	assert.False(t, groupCalled, "single file should not use media group")
}

func TestExecuteBlocks_NilMediaGroupFallsBackToSendDoc(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644))

	var docCount int
	sendDoc := func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
		docCount++
		return nil
	}

	text := "```nclaw:sendfile\n{\"path\":\"a.txt\"}\n```\n```nclaw:sendfile\n{\"path\":\"b.txt\"}\n```"
	ExecuteBlocks(context.TODO(), Senders{Doc: sendDoc}, text, 1, 0, dir)

	assert.Equal(t, 2, docCount, "without media group func, should fall back to individual sends")
}

func TestExecuteBlocks_AudioUsesSendAudio(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "song.mp3"), []byte("audio"), 0o644))

	var audioCalled bool
	sendAudio := func(_ context.Context, _ int64, _ int, filename string, _ []byte, _ string) error {
		audioCalled = true
		assert.Equal(t, "song.mp3", filename)
		return nil
	}
	var docCalled bool
	sendDoc := func(_ context.Context, _ int64, _ int, _ string, _ []byte, _ string) error {
		docCalled = true
		return nil
	}

	text := "```nclaw:sendfile\n{\"path\":\"song.mp3\"}\n```"
	ExecuteBlocks(context.TODO(), Senders{Doc: sendDoc, Audio: sendAudio}, text, 1, 0, dir)

	assert.True(t, audioCalled, "audio file should use sendAudio")
	assert.False(t, docCalled, "audio file should not use sendDoc")
}

func TestMediaTypeFromExt(t *testing.T) {
	assert.Equal(t, MediaAudio, mediaTypeFromExt(".mp3"))
	assert.Equal(t, MediaAudio, mediaTypeFromExt(".MP3"))
	assert.Equal(t, MediaAudio, mediaTypeFromExt(".ogg"))
	assert.Equal(t, MediaAudio, mediaTypeFromExt(".flac"))
	assert.Equal(t, MediaPhoto, mediaTypeFromExt(".jpg"))
	assert.Equal(t, MediaPhoto, mediaTypeFromExt(".png"))
	assert.Equal(t, MediaVideo, mediaTypeFromExt(".mp4"))
	assert.Equal(t, MediaVideo, mediaTypeFromExt(".mov"))
	assert.Equal(t, MediaDocument, mediaTypeFromExt(".txt"))
	assert.Equal(t, MediaDocument, mediaTypeFromExt(".pdf"))
	assert.Equal(t, MediaDocument, mediaTypeFromExt(""))
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

type recordingSenders struct {
	docs   []string
	groups [][]string
}

func (r *recordingSenders) senders(groupErr error) Senders {
	return Senders{
		Doc: func(_ context.Context, _ int64, _ int, filename string, _ []byte, _ string) error {
			r.docs = append(r.docs, filename)
			return nil
		},
		MediaGroup: func(_ context.Context, _ int64, _ int, files []File) error {
			names := make([]string, len(files))
			for i, f := range files {
				names[i] = f.Filename
			}
			r.groups = append(r.groups, names)
			return groupErr
		},
	}
}

func sendfileBlocks(t *testing.T, dir string, names ...string) string {
	t.Helper()
	var b strings.Builder
	for _, n := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte(n), 0o644))
		fmt.Fprintf(&b, "```nclaw:sendfile\n{\"path\":%q}\n```\n", n)
	}
	return b.String()
}

func TestExecuteBlocks_MixedTypesGroupedByAlbumKind(t *testing.T) {
	dir := t.TempDir()
	text := sendfileBlocks(t, dir, "chart.png", "report.pdf", "clip.mp4", "data.xlsx", "song.mp3")

	var r recordingSenders
	ExecuteBlocks(context.TODO(), r.senders(nil), text, 1, 0, dir)

	assert.Equal(t, [][]string{{"chart.png", "clip.mp4"}, {"report.pdf", "data.xlsx"}}, r.groups)
	assert.Equal(t, []string{"song.mp3"}, r.docs)
}

func TestExecuteBlocks_TrailingSingleFileSentIndividually(t *testing.T) {
	dir := t.TempDir()
	names := make([]string, 11)
	for i := range names {
		names[i] = fmt.Sprintf("f%02d.txt", i)
	}
	text := sendfileBlocks(t, dir, names...)

	var r recordingSenders
	ExecuteBlocks(context.TODO(), r.senders(nil), text, 1, 0, dir)

	require.Len(t, r.groups, 1)
	assert.Len(t, r.groups[0], 10)
	assert.Equal(t, []string{"f10.txt"}, r.docs)
}

func TestExecuteBlocks_RejectedGroupFallsBackToIndividualSends(t *testing.T) {
	dir := t.TempDir()
	text := sendfileBlocks(t, dir, "a.txt", "b.txt")

	var r recordingSenders
	ExecuteBlocks(context.TODO(), r.senders(errors.New("400 Bad Request")), text, 1, 0, dir)

	assert.Len(t, r.groups, 1)
	assert.Equal(t, []string{"a.txt", "b.txt"}, r.docs)
}

func TestExecuteBlocks_AbsolutePathWithRelativeChatDir(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := filepath.Join("data", "1")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "out.txt"), []byte("x"), 0o644))
	abs, err := filepath.Abs(filepath.Join(dir, "out.txt"))
	require.NoError(t, err)

	var r recordingSenders
	text := fmt.Sprintf("```nclaw:sendfile\n{\"path\":%q}\n```", abs)
	ExecuteBlocks(context.TODO(), r.senders(nil), text, 1, 0, dir)

	assert.Equal(t, []string{"out.txt"}, r.docs)
}

func TestExecuteBlocks_RelativeChatDirStillRejectsEscape(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	tmp := filepath.Join(root, "tmp")
	require.NoError(t, os.MkdirAll(tmp, 0o755))
	t.Setenv("TMPDIR", tmp) // the test root itself lives under the OS temp dir, which is allowed
	dir := filepath.Join("data", "1")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("data", "secret.txt"), []byte("x"), 0o644))

	var r recordingSenders
	text := "```nclaw:sendfile\n{\"path\":\"../secret.txt\"}\n```"
	ExecuteBlocks(context.TODO(), r.senders(nil), text, 1, 0, dir)

	assert.Empty(t, r.docs)
}
