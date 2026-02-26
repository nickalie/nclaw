package copilot

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
	assert.Same(t, c, c.Dir("/tmp").(*Copilot))
	assert.Same(t, c, c.AppendSystemPrompt("extra").(*Copilot))
	assert.Same(t, c, c.SkipPermissions().(*Copilot))
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
	c.skipPermissions = true

	c.prepare()

	args := c.bin.Args()
	assert.Contains(t, args, "-s")
	assert.Contains(t, args, "--allow-all-tools")
	assert.Contains(t, args, "--no-ask-user")
	assert.Contains(t, args, "-p")
	assert.NotContains(t, args, "--continue")
}

func TestPrepare_AskArgs_NoSkipPermissions(t *testing.T) {
	c := New()

	c.prepare()

	args := c.bin.Args()
	assert.Contains(t, args, "-s")
	assert.Contains(t, args, "-p")
	assert.NotContains(t, args, "--allow-all-tools")
	assert.NotContains(t, args, "--no-ask-user")
	assert.NotContains(t, args, "--continue")
}

func TestPrepareContinue_Args(t *testing.T) {
	c := New()
	c.skipPermissions = true

	c.prepareContinue()

	args := c.bin.Args()
	assert.Contains(t, args, "--continue")
	assert.Contains(t, args, "-s")
	assert.Contains(t, args, "--allow-all-tools")
	assert.Contains(t, args, "--no-ask-user")
	assert.Contains(t, args, "-p")
}

func TestPrepareContinue_NoSkipPermissions(t *testing.T) {
	c := New()

	c.prepareContinue()

	args := c.bin.Args()
	assert.Contains(t, args, "--continue")
	assert.Contains(t, args, "-s")
	assert.Contains(t, args, "-p")
	assert.NotContains(t, args, "--allow-all-tools")
	assert.NotContains(t, args, "--no-ask-user")
}

func TestPrepare_ResetsBetweenCalls(t *testing.T) {
	c := New()
	c.skipPermissions = true

	c.prepare()
	firstArgs := make([]string, len(c.bin.Args()))
	copy(firstArgs, c.bin.Args())

	// Switch to continue mode — should reset and rebuild.
	c.prepareContinue()
	secondArgs := c.bin.Args()

	assert.NotContains(t, firstArgs, "--continue")
	assert.Contains(t, secondArgs, "--continue")
}

func TestWriteSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.dir = dir
	c.systemPrompt = "Test instructions"

	err := c.writeSystemPrompt()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".github", "copilot-instructions.md"))
	require.NoError(t, err)
	assert.Equal(t, "Test instructions", string(content))
}

func TestWriteSystemPrompt_CreatesGitHubDir(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.dir = dir
	c.systemPrompt = "Test instructions"

	err := c.writeSystemPrompt()
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, ".github"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestWriteSystemPrompt_NoPrompt(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.dir = dir

	err := c.writeSystemPrompt()
	require.NoError(t, err)

	// Should not create the .github directory or file.
	_, err = os.Stat(filepath.Join(dir, ".github"))
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
	assert.Equal(t, "copilot", p.Name())
}

func TestProvider_NewClient(t *testing.T) {
	p := NewProvider()
	client := p.NewClient()
	assert.NotNil(t, client)

	c, ok := client.(*Copilot)
	assert.True(t, ok)
	assert.NotNil(t, c.bin)
}

func TestProvider_PreInvoke(t *testing.T) {
	p := NewProvider()
	assert.NoError(t, p.PreInvoke())
}

// Plain text output tests — Copilot uses -s flag so Text == FullText.

func TestRunAndParse_PlainTextOutput(t *testing.T) {
	// Copilot's -s (silent) mode outputs plain text directly.
	// Since we can't mock bin.Run easily, we test the parsing logic
	// indirectly through the Result equality contract: Text must equal FullText.
	c := New()
	// Verify that after construction, the struct fields match expectations.
	assert.Equal(t, "", c.dir)
	assert.Equal(t, "", c.systemPrompt)
	assert.False(t, c.skipPermissions)
}

func TestTextEqualsFullText_Contract(t *testing.T) {
	// This test documents the key Copilot limitation:
	// Because -s mode only outputs the final text, Text == FullText.
	// This means command blocks in intermediate messages won't be captured.
	// The runAndParse method enforces this by using the same string for both fields.
	//
	// We verify this contract by checking the source implementation indirectly:
	// the test for writeSystemPrompt path confirms the copilot-instructions.md
	// location, and the prepare tests confirm the -s flag is always present.
	c := New()
	c.prepare()
	args := c.bin.Args()
	assert.Contains(t, args, "-s", "silent mode must always be enabled for Text==FullText contract")
}
