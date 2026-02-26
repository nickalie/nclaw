package claudish

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	c := New()
	assert.NotNil(t, c)
	assert.NotNil(t, c.bin)
}

func TestBuilderChaining(t *testing.T) {
	c := New()

	// Interface-returning methods (cli.Client) return the same underlying instance.
	assert.Same(t, c, c.Dir("/tmp").(*Claudish))
	assert.Same(t, c, c.AppendSystemPrompt("extra").(*Claudish))
	assert.Same(t, c, c.SkipPermissions().(*Claudish))
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
	c.model = "gpt-4o"
	c.skipPermissions = true
	c.systemPrompt = "be helpful"

	c.prepare("-p")

	args := c.bin.Args()
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "gpt-4o")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--append-system-prompt")
	assert.Contains(t, args, "be helpful")
	assert.Contains(t, args, "--dangerously-skip-permissions")
	assert.Contains(t, args, "--verbose")
}

func TestPrepare_ContinueArgs(t *testing.T) {
	c := New()
	c.dir = "/work"

	c.prepare("-c", "-p")

	args := c.bin.Args()
	assert.Contains(t, args, "-c")
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--verbose")
}

func TestPrepare_NoModel(t *testing.T) {
	c := New()

	c.prepare("-p")

	args := c.bin.Args()
	assert.NotContains(t, args, "--model")
	// output-format and verbose are always set
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "--verbose")
}

func TestPrepare_NoSkipPermissions(t *testing.T) {
	c := New()

	c.prepare("-p")

	args := c.bin.Args()
	assert.NotContains(t, args, "--dangerously-skip-permissions")
}

func TestPrepare_NoSystemPrompt(t *testing.T) {
	c := New()

	c.prepare("-p")

	args := c.bin.Args()
	assert.NotContains(t, args, "--append-system-prompt")
}

func TestPrepare_ResetsBetweenCalls(t *testing.T) {
	c := New()
	c.dir = "/first"
	c.model = "model-a"
	c.skipPermissions = true

	c.prepare("-p")
	firstArgs := make([]string, len(c.bin.Args()))
	copy(firstArgs, c.bin.Args())

	// Second prepare should reset and rebuild.
	c.dir = "/second"
	c.model = "model-b"
	c.prepare("-c", "-p")
	secondArgs := c.bin.Args()

	assert.Contains(t, firstArgs, "model-a")
	assert.NotContains(t, firstArgs, "-c")
	assert.Contains(t, secondArgs, "model-b")
	assert.Contains(t, secondArgs, "-c")
}

func TestBuildEnv_ModelTiers(t *testing.T) {
	c := New()
	c.modelOpus = "opus-model"
	c.modelSonnet = "sonnet-model"
	c.modelHaiku = "haiku-model"
	c.modelSubagent = "subagent-model"

	env := c.buildEnv()

	assert.Contains(t, env, "CLAUDISH_MODEL_OPUS=opus-model")
	assert.Contains(t, env, "CLAUDISH_MODEL_SONNET=sonnet-model")
	assert.Contains(t, env, "CLAUDISH_MODEL_HAIKU=haiku-model")
	assert.Contains(t, env, "CLAUDISH_MODEL_SUBAGENT=subagent-model")
}

func TestBuildEnv_EmptyKeysOmitted(t *testing.T) {
	// Unset any host env vars so the test is deterministic regardless of environment.
	keys := []string{
		"CLAUDISH_MODEL_OPUS", "CLAUDISH_MODEL_SONNET", "CLAUDISH_MODEL_HAIKU", "CLAUDISH_MODEL_SUBAGENT",
	}
	for _, key := range keys {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	c := New()

	env := c.buildEnv()

	for _, v := range env {
		assert.False(t, strings.HasPrefix(v, "CLAUDISH_MODEL_OPUS="))
		assert.False(t, strings.HasPrefix(v, "CLAUDISH_MODEL_SONNET="))
		assert.False(t, strings.HasPrefix(v, "CLAUDISH_MODEL_HAIKU="))
		assert.False(t, strings.HasPrefix(v, "CLAUDISH_MODEL_SUBAGENT="))
	}
}

func TestBuildEnv_FiltersCLAUDECODE(t *testing.T) {
	t.Setenv("CLAUDECODE", "true")

	c := New()
	env := c.buildEnv()

	for _, v := range env {
		assert.False(t, strings.HasPrefix(v, "CLAUDECODE="),
			"CLAUDECODE should be filtered from env")
	}
}

func TestBuildEnv_IncludesOSEnv(t *testing.T) {
	c := New()
	env := c.buildEnv()

	found := false
	for _, v := range env {
		if strings.HasPrefix(v, "PATH=") {
			found = true
			break
		}
	}
	assert.True(t, found, "should include PATH from OS environment")
}

func TestBuildEnv_NoKeys(t *testing.T) {
	c := New()
	env := c.buildEnv()
	// Should have at least as many entries as os.Environ (minus any CLAUDECODE).
	osEnv := os.Environ()
	assert.GreaterOrEqual(t, len(env), len(osEnv)-1)
}

func TestNewProvider(t *testing.T) {
	p := NewProvider("gpt-4o", "opus-m", "sonnet-m", "haiku-m", "sub-m")
	assert.NotNil(t, p)
	assert.Equal(t, "claudish", p.Name())
	assert.Equal(t, "gpt-4o", p.model)
	assert.Equal(t, "opus-m", p.modelOpus)
	assert.Equal(t, "sonnet-m", p.modelSonnet)
	assert.Equal(t, "haiku-m", p.modelHaiku)
	assert.Equal(t, "sub-m", p.modelSubagent)
}

func TestProvider_NewClient(t *testing.T) {
	p := NewProvider("gpt-4o", "opus-m", "sonnet-m", "haiku-m", "sub-m")
	client := p.NewClient()
	assert.NotNil(t, client)

	c, ok := client.(*Claudish)
	assert.True(t, ok)
	assert.NotNil(t, c.bin)
	assert.Equal(t, "gpt-4o", c.model)
	assert.Equal(t, "opus-m", c.modelOpus)
	assert.Equal(t, "sonnet-m", c.modelSonnet)
	assert.Equal(t, "haiku-m", c.modelHaiku)
	assert.Equal(t, "sub-m", c.modelSubagent)
}

func TestProvider_PreInvoke(t *testing.T) {
	p := NewProvider("", "", "", "", "")
	assert.NoError(t, p.PreInvoke())
}

func TestProvider_NewClient_EmptyConfig(t *testing.T) {
	p := NewProvider("", "", "", "", "")
	client := p.NewClient()

	c, ok := client.(*Claudish)
	assert.True(t, ok)
	assert.Empty(t, c.model)
	assert.Empty(t, c.modelOpus)
}

func TestBuildEnv_APIKeysPassThrough(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("GEMINI_API_KEY", "gem-key")

	c := New()
	env := c.buildEnv()

	assert.Contains(t, env, "OPENROUTER_API_KEY=or-key")
	assert.Contains(t, env, "GEMINI_API_KEY=gem-key")
}

func TestSanitizeOutput_StripsSensitiveLines(t *testing.T) {
	c := New()
	output := "Error starting claudish\nOPENROUTER_API_KEY=sk-secret-123\nGEMINI_API_KEY=gem-secret\nZHIPU_API_KEY=zhipu-secret\nSome safe error message"
	sanitized := c.sanitizeOutput(output)

	assert.NotContains(t, sanitized, "_API_KEY")
	assert.Contains(t, sanitized, "Error starting claudish")
	assert.Contains(t, sanitized, "Some safe error message")
}

func TestSanitizeOutput_PreservesCleanOutput(t *testing.T) {
	c := New()
	output := "claudish: command not found"
	sanitized := c.sanitizeOutput(output)
	assert.Equal(t, "claudish: command not found", sanitized)
}

func TestSanitizeOutput_EmptyOutput(t *testing.T) {
	c := New()
	assert.Equal(t, "", c.sanitizeOutput(""))
	assert.Equal(t, "", c.sanitizeOutput("  \n  "))
}
