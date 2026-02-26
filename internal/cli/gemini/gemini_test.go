package gemini

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	g := New()
	assert.NotNil(t, g)
	assert.NotNil(t, g.bin)
}

func TestBuilderChaining(t *testing.T) {
	g := New()

	// Interface-returning methods (cli.Client) return the same underlying instance.
	assert.Same(t, g, g.Dir("/tmp").(*Gemini))
	assert.Same(t, g, g.AppendSystemPrompt("extra").(*Gemini))
	assert.Same(t, g, g.SkipPermissions().(*Gemini))
}

func TestDir(t *testing.T) {
	g := New()
	g.Dir("/work")
	assert.Equal(t, "/work", g.dir)
}

func TestSkipPermissions(t *testing.T) {
	g := New()
	assert.False(t, g.skipPermissions)
	g.SkipPermissions()
	assert.True(t, g.skipPermissions)
}

func TestAppendSystemPrompt(t *testing.T) {
	g := New()
	g.AppendSystemPrompt("custom instructions")
	assert.Equal(t, "custom instructions", g.systemPrompt)
}

func TestNew_DefaultFields(t *testing.T) {
	g := New()
	assert.Equal(t, "", g.dir)
	assert.Equal(t, "", g.systemPrompt)
	assert.False(t, g.skipPermissions)
}

func TestPrepare_AskArgs(t *testing.T) {
	g := New()
	g.skipPermissions = true

	g.prepare()

	args := g.bin.Args()
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--approval-mode")
	assert.Contains(t, args, "yolo")
	assert.Contains(t, args, "-p")
	assert.NotContains(t, args, "--resume")
}

func TestPrepare_AskArgs_NoSkipPermissions(t *testing.T) {
	g := New()

	g.prepare()

	args := g.bin.Args()
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "-p")
	assert.NotContains(t, args, "--approval-mode")
	assert.NotContains(t, args, "yolo")
	assert.NotContains(t, args, "--resume")
}

func TestPrepareContinue_Args(t *testing.T) {
	g := New()
	g.skipPermissions = true

	g.prepareContinue()

	args := g.bin.Args()
	assert.Contains(t, args, "--resume")
	assert.Contains(t, args, "latest")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--approval-mode")
	assert.Contains(t, args, "yolo")
	assert.Contains(t, args, "-p")
}

func TestPrepareContinue_NoSkipPermissions(t *testing.T) {
	g := New()

	g.prepareContinue()

	args := g.bin.Args()
	assert.Contains(t, args, "--resume")
	assert.Contains(t, args, "latest")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "-p")
	assert.NotContains(t, args, "--approval-mode")
	assert.NotContains(t, args, "yolo")
}

func TestPrepare_ResetsBetweenCalls(t *testing.T) {
	g := New()
	g.skipPermissions = true

	g.prepare()
	firstArgs := make([]string, len(g.bin.Args()))
	copy(firstArgs, g.bin.Args())

	// Switch to continue mode — should reset and rebuild.
	g.prepareContinue()
	secondArgs := g.bin.Args()

	assert.NotContains(t, firstArgs, "--resume")
	assert.Contains(t, secondArgs, "--resume")
}

func TestWriteSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	g := New()
	g.dir = dir
	g.systemPrompt = "Test instructions"

	err := g.writeSystemPrompt()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "GEMINI.md"))
	require.NoError(t, err)
	assert.Equal(t, "Test instructions", string(content))
}

func TestWriteSystemPrompt_NoPrompt(t *testing.T) {
	dir := t.TempDir()
	g := New()
	g.dir = dir

	err := g.writeSystemPrompt()
	require.NoError(t, err)

	// Should not create the file.
	_, err = os.Stat(filepath.Join(dir, "GEMINI.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestWriteSystemPrompt_NoDir(t *testing.T) {
	g := New()
	g.systemPrompt = "Test instructions"

	err := g.writeSystemPrompt()
	require.NoError(t, err)
}

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	assert.NotNil(t, p)
	assert.Equal(t, "gemini", p.Name())
}

func TestProvider_NewClient(t *testing.T) {
	p := NewProvider()
	client := p.NewClient()
	assert.NotNil(t, client)

	g, ok := client.(*Gemini)
	assert.True(t, ok)
	assert.NotNil(t, g.bin)
}

func TestProvider_PreInvoke(t *testing.T) {
	p := NewProvider()
	assert.NoError(t, p.PreInvoke())
}
