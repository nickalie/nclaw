package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	c := New()
	assert.NotNil(t, c)
	assert.NotNil(t, c.bin)
}

func TestBuilderChaining(t *testing.T) {
	c := New()

	// Interface-returning methods (cli.Client) return the same underlying instance.
	assert.Same(t, c, c.Dir("/tmp").(*Codex))
	assert.Same(t, c, c.AppendSystemPrompt("extra").(*Codex))
	assert.Same(t, c, c.SkipPermissions().(*Codex))
}

func TestDir(t *testing.T) {
	c := New()
	c.Dir("/work")
	assert.Equal(t, "/work", c.dir)
}

func TestSkipPermissions(t *testing.T) {
	c := New()
	assert.False(t, c.skipPermissions)
	c.SkipPermissions()
	assert.True(t, c.skipPermissions)
}

func TestAppendSystemPrompt(t *testing.T) {
	c := New()
	c.AppendSystemPrompt("custom instructions")
	assert.Equal(t, "custom instructions", c.systemPrompt)
}

func TestPrepare_AskArgs(t *testing.T) {
	c := New()
	c.dir = "/work"
	c.skipPermissions = true

	c.prepare()

	args := c.bin.Args()
	assert.Contains(t, args, "exec")
	assert.Contains(t, args, "--json")
	assert.Contains(t, args, "--full-auto")
	assert.Contains(t, args, "--cd")
	assert.Contains(t, args, "/work")
}

func TestPrepare_ContinueArgs(t *testing.T) {
	c := New()
	c.dir = "/work"

	c.prepare("resume", "--last")

	args := c.bin.Args()
	assert.Contains(t, args, "exec")
	assert.Contains(t, args, "--json")
	assert.Contains(t, args, "--cd")
	assert.Contains(t, args, "/work")
	assert.Contains(t, args, "resume")
	assert.Contains(t, args, "--last")
}

func TestPrepare_NoSkipPermissions(t *testing.T) {
	c := New()

	c.prepare()

	args := c.bin.Args()
	assert.Contains(t, args, "exec")
	assert.Contains(t, args, "--json")
	assert.NotContains(t, args, "--full-auto")
}

func TestPrepare_NoDir(t *testing.T) {
	c := New()

	c.prepare()

	args := c.bin.Args()
	assert.NotContains(t, args, "--cd")
}

func TestWriteSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.dir = dir
	c.systemPrompt = "Test instructions"

	err := c.writeSystemPrompt()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Equal(t, "Test instructions", string(content))
}

func TestWriteSystemPrompt_NoPrompt(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.dir = dir

	err := c.writeSystemPrompt()
	require.NoError(t, err)

	// Should not create the file.
	_, err = os.Stat(filepath.Join(dir, "AGENTS.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestWriteSystemPrompt_NoDir(t *testing.T) {
	c := New()
	c.systemPrompt = "Test instructions"

	err := c.writeSystemPrompt()
	require.NoError(t, err)
}

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	assert.NotNil(t, p)
	assert.Equal(t, "codex", p.Name())
}

func TestProvider_NewClient(t *testing.T) {
	p := NewProvider()
	client := p.NewClient()
	assert.NotNil(t, client)

	c, ok := client.(*Codex)
	assert.True(t, ok)
	assert.NotNil(t, c.bin)
}

func TestProvider_PreInvoke(t *testing.T) {
	p := NewProvider()
	assert.NoError(t, p.PreInvoke())
}

func TestPrepare_ResetsBetweenCalls(t *testing.T) {
	c := New()
	c.dir = "/first"
	c.skipPermissions = true

	c.prepare()
	firstArgs := make([]string, len(c.bin.Args()))
	copy(firstArgs, c.bin.Args())

	// Second prepare should reset and rebuild.
	c.dir = "/second"
	c.prepare("resume", "--last")
	secondArgs := c.bin.Args()

	assert.Contains(t, firstArgs, "/first")
	assert.NotContains(t, firstArgs, "resume")
	assert.Contains(t, secondArgs, "/second")
	assert.Contains(t, secondArgs, "resume")
}

func TestPrepare_FullConfig(t *testing.T) {
	c := New()
	c.dir = "/work"
	c.skipPermissions = true

	c.prepare()

	args := c.bin.Args()
	assert.Contains(t, args, "exec")
	assert.Contains(t, args, "--json")
	assert.Contains(t, args, "--full-auto")
	assert.Contains(t, args, "--cd")
	assert.Contains(t, args, "/work")
	// -p should not be present (codex doesn't use it)
	assert.NotContains(t, args, "-p")
}

func TestPrepare_ExtraArgsAppended(t *testing.T) {
	c := New()

	c.prepare("resume", "--last")

	args := c.bin.Args()
	// Extra args should come after base args.
	assert.Contains(t, args, "exec")
	assert.Contains(t, args, "--json")
	assert.Contains(t, args, "resume")
	assert.Contains(t, args, "--last")
}

func TestPrepare_MinimalConfig(t *testing.T) {
	c := New()

	c.prepare()

	args := c.bin.Args()
	assert.Contains(t, args, "exec")
	assert.Contains(t, args, "--json")
	assert.NotContains(t, args, "--full-auto")
	assert.NotContains(t, args, "--cd")
}

func TestWriteSystemPrompt_Overwrites(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.dir = dir
	c.systemPrompt = "First version"

	err := c.writeSystemPrompt()
	require.NoError(t, err)

	// Overwrite with new content.
	c.systemPrompt = "Second version"
	err = c.writeSystemPrompt()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Equal(t, "Second version", string(content))
}

func TestBuilderFieldValues(t *testing.T) {
	c := New()
	c.Dir("/work")
	c.AppendSystemPrompt("custom prompt")
	c.SkipPermissions()

	assert.Equal(t, "/work", c.dir)
	assert.Equal(t, "custom prompt", c.systemPrompt)
	assert.True(t, c.skipPermissions)
}

func writeFakeCodex(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codex")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	return path
}

func TestAsk_IsEphemeral(t *testing.T) {
	c := New()
	c.bin.ExecPath(writeFakeCodex(t))

	_, err := c.Ask("hello")
	require.NoError(t, err)
	assert.Contains(t, c.bin.Args(), "--ephemeral")
}

func TestContinue_IsNotEphemeral(t *testing.T) {
	c := New()
	c.bin.ExecPath(writeFakeCodex(t))

	_, err := c.Continue("hello")
	require.NoError(t, err)
	assert.NotContains(t, c.bin.Args(), "--ephemeral")
}
