package claude

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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
	assert.Same(t, c, c.Dir("/tmp").(*Claude))
	assert.Same(t, c, c.AppendSystemPrompt("extra").(*Claude))
	assert.Same(t, c, c.SkipPermissions().(*Claude))

	// Claude-specific builder methods return *Claude directly.
	assert.Same(t, c, c.Model("opus"))
	assert.Same(t, c, c.FallbackModel("sonnet"))
	assert.Same(t, c, c.SystemPrompt("sys"))
	assert.Same(t, c, c.PermissionMode("plan"))
	assert.Same(t, c, c.MCPConfig("/config.json"))
	assert.Same(t, c, c.JSONSchema("{}"))
	assert.Same(t, c, c.MaxTurns(5))
	assert.Same(t, c, c.MaxBudget(1.50))
	assert.Same(t, c, c.AllowedTools("Read", "Write"))
	assert.Same(t, c, c.DisallowedTools("Bash"))
	assert.Same(t, c, c.Tools("Read"))
	assert.Same(t, c, c.AddDirs("/extra"))
	assert.Same(t, c, c.NoSessionPersistence())
	assert.Same(t, c, c.Verbose())
	assert.Same(t, c, c.Timeout(30*time.Second))
	assert.Same(t, c, c.StdIn(strings.NewReader("input")))
	assert.Same(t, c, c.Env([]string{"FOO=bar"}))
	assert.Same(t, c, c.ExecPath("/opt/claude/bin/claude"))
}

func TestBuilderFieldValues(t *testing.T) {
	c := New().
		Model("opus").
		FallbackModel("sonnet").
		SystemPrompt("system").
		PermissionMode("plan").
		MCPConfig("/mcp.json").
		JSONSchema("{\"type\":\"object\"}").
		MaxTurns(10).
		MaxBudget(5.00).
		AllowedTools("Read", "Write").
		DisallowedTools("Bash").
		Tools("Read", "Grep").
		AddDirs("/extra1", "/extra2").
		Env([]string{"KEY=val"})

	// Interface methods called separately (they return cli.Client).
	c.Dir("/work")
	c.AppendSystemPrompt("append")

	assert.Equal(t, "/work", c.dir)
	assert.Equal(t, "opus", c.model)
	assert.Equal(t, "sonnet", c.fallbackModel)
	assert.Equal(t, "system", c.systemPrompt)
	assert.Equal(t, "append", c.appendPrompt)
	assert.Equal(t, "plan", c.permissionMode)
	assert.Equal(t, "/mcp.json", c.mcpConfig)
	assert.Equal(t, "{\"type\":\"object\"}", c.jsonSchema)
	assert.Equal(t, 10, c.maxTurns)
	assert.InDelta(t, 5.00, c.maxBudgetUSD, 0.001)
	assert.Equal(t, []string{"Read", "Write"}, c.allowedTools)
	assert.Equal(t, []string{"Bash"}, c.disallowedTools)
	assert.Equal(t, []string{"Read", "Grep"}, c.tools)
	assert.Equal(t, []string{"/extra1", "/extra2"}, c.addDirs)
	assert.Equal(t, []string{"KEY=val"}, c.env)
}

func TestSkipPermissions(t *testing.T) {
	c := New()
	assert.False(t, c.skipPermissions)
	c.SkipPermissions()
	assert.True(t, c.skipPermissions)
}

func TestNoSessionPersistence(t *testing.T) {
	c := New()
	assert.False(t, c.noPersistence)
	c.NoSessionPersistence()
	assert.True(t, c.noPersistence)
}

func TestVerbose(t *testing.T) {
	c := New()
	assert.False(t, c.verbose)
	c.Verbose()
	assert.True(t, c.verbose)
}

func TestBuildEnv_FiltersCLAUDECODE(t *testing.T) {
	t.Setenv("CLAUDECODE", "true")

	c := New().Env([]string{"EXTRA=1"})
	env := c.buildEnv()

	for _, v := range env {
		assert.False(t, strings.HasPrefix(v, "CLAUDECODE="),
			"CLAUDECODE should be filtered from env")
	}
	assert.Contains(t, env, "EXTRA=1")
}

func TestBuildEnv_IncludesOSEnv(t *testing.T) {
	c := New()
	env := c.buildEnv()
	// Should include at least PATH from the OS environment
	found := false
	for _, v := range env {
		if strings.HasPrefix(v, "PATH=") {
			found = true
			break
		}
	}
	assert.True(t, found, "should include PATH from OS environment")
}

func TestBuildEnv_AppendsCustomVars(t *testing.T) {
	c := New().Env([]string{"MY_VAR=hello"})
	env := c.buildEnv()
	assert.Contains(t, env, "MY_VAR=hello")
}

func TestOutputFormatConstant(t *testing.T) {
	assert.Equal(t, outputFormat("stream-json"), formatStreamJSON)
}

func TestStdOutStdErr_InitiallyEmpty(t *testing.T) {
	c := New()
	assert.Empty(t, c.StdOut())
	assert.Empty(t, c.StdErr())
}

func TestBuildEnv_NoCustomVars(t *testing.T) {
	c := New()
	env := c.buildEnv()
	// Should have at least as many entries as os.Environ (minus any CLAUDECODE)
	osEnv := os.Environ()
	assert.GreaterOrEqual(t, len(env), len(osEnv)-1)
}

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	assert.NotNil(t, p)
	assert.Equal(t, "claude", p.Name())
}

func TestProvider_NewClient(t *testing.T) {
	p := NewProvider()
	client := p.NewClient()
	assert.NotNil(t, client)

	// The returned client should be a *Claude instance.
	c, ok := client.(*Claude)
	assert.True(t, ok)
	assert.NotNil(t, c.bin)
}

func TestProvider_NewClientWithExecPath(t *testing.T) {
	p := NewProvider("/opt/claude/bin/claude")
	client := p.NewClient()
	c, ok := client.(*Claude)
	assert.True(t, ok)
	assert.Equal(t, "/opt/claude/bin/claude", c.bin.Path())
}

func TestProvider_VersionWithExecPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-test")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\necho custom-claude 1.2.3\n"), 0o755))

	p := NewProvider(path)
	v, err := p.Version()
	assert.NoError(t, err)
	assert.Equal(t, "custom-claude 1.2.3", v)
}

func TestProvider_PreInvoke(t *testing.T) {
	// PreInvoke wraps EnsureValidToken; with no credentials file, it should succeed.
	p := NewProvider()
	withTestCredPath(t, filepath.Join(t.TempDir(), "nonexistent.json"))
	assert.NoError(t, p.PreInvoke())
}

func TestPrepare_ExtraArgs(t *testing.T) {
	c := New()
	c.prepare("-p")
	args := c.bin.Args()
	assert.Contains(t, args, "-p")
}

func TestPrepare_ContinueArgs(t *testing.T) {
	c := New()
	c.prepare("-c", "-p")
	args := c.bin.Args()
	assert.Contains(t, args, "-c")
	assert.Contains(t, args, "-p")
}

func TestPrepareModel_WithModel(t *testing.T) {
	c := New().Model("opus")
	c.outputFormat = formatStreamJSON
	c.bin.Reset()
	c.prepareModel()
	args := c.bin.Args()
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "opus")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
}

func TestPrepareModel_WithFallbackModel(t *testing.T) {
	c := New().Model("opus").FallbackModel("sonnet")
	c.bin.Reset()
	c.prepareModel()
	args := c.bin.Args()
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "opus")
	assert.Contains(t, args, "--fallback-model")
	assert.Contains(t, args, "sonnet")
}

func TestPrepareModel_Empty(t *testing.T) {
	c := New()
	c.bin.Reset()
	c.prepareModel()
	args := c.bin.Args()
	assert.NotContains(t, args, "--model")
	assert.NotContains(t, args, "--fallback-model")
	assert.NotContains(t, args, "--output-format")
}

func TestPreparePrompt_SystemPrompt(t *testing.T) {
	c := New().SystemPrompt("you are a bot")
	c.bin.Reset()
	c.preparePrompt()
	args := c.bin.Args()
	assert.Contains(t, args, "--system-prompt")
	assert.Contains(t, args, "you are a bot")
}

func TestPreparePrompt_AppendPrompt(t *testing.T) {
	c := New()
	c.AppendSystemPrompt("extra instructions")
	c.bin.Reset()
	c.preparePrompt()
	args := c.bin.Args()
	assert.Contains(t, args, "--append-system-prompt")
	assert.Contains(t, args, "extra instructions")
}

func TestPreparePrompt_PermissionMode(t *testing.T) {
	c := New().PermissionMode("plan")
	c.bin.Reset()
	c.preparePrompt()
	args := c.bin.Args()
	assert.Contains(t, args, "--permission-mode")
	assert.Contains(t, args, "plan")
}

func TestPreparePrompt_MCPConfig(t *testing.T) {
	c := New().MCPConfig("/path/to/config.json")
	c.bin.Reset()
	c.preparePrompt()
	args := c.bin.Args()
	assert.Contains(t, args, "--mcp-config")
	assert.Contains(t, args, "/path/to/config.json")
}

func TestPreparePrompt_JSONSchema(t *testing.T) {
	c := New().JSONSchema(`{"type":"object"}`)
	c.bin.Reset()
	c.preparePrompt()
	args := c.bin.Args()
	assert.Contains(t, args, "--json-schema")
	assert.Contains(t, args, `{"type":"object"}`)
}

func TestPreparePrompt_Empty(t *testing.T) {
	c := New()
	c.bin.Reset()
	c.preparePrompt()
	args := c.bin.Args()
	assert.NotContains(t, args, "--system-prompt")
	assert.NotContains(t, args, "--append-system-prompt")
	assert.NotContains(t, args, "--permission-mode")
	assert.NotContains(t, args, "--mcp-config")
	assert.NotContains(t, args, "--json-schema")
}

func TestPrepareLimits_MaxTurns(t *testing.T) {
	c := New().MaxTurns(10)
	c.bin.Reset()
	c.prepareLimits()
	args := c.bin.Args()
	assert.Contains(t, args, "--max-turns")
	assert.Contains(t, args, "10")
}

func TestPrepareLimits_MaxBudget(t *testing.T) {
	c := New().MaxBudget(5.50)
	c.bin.Reset()
	c.prepareLimits()
	args := c.bin.Args()
	assert.Contains(t, args, "--max-budget-usd")
	assert.Contains(t, args, "5.50")
}

func TestPrepareLimits_Empty(t *testing.T) {
	c := New()
	c.bin.Reset()
	c.prepareLimits()
	args := c.bin.Args()
	assert.NotContains(t, args, "--max-turns")
	assert.NotContains(t, args, "--max-budget-usd")
}

func TestPrepareTools_AllowedTools(t *testing.T) {
	c := New().AllowedTools("Read", "Write")
	c.bin.Reset()
	c.prepareTools()
	args := c.bin.Args()
	assert.Contains(t, args, "--allowedTools")
	assert.Contains(t, args, "Read")
	assert.Contains(t, args, "Write")
}

func TestPrepareTools_DisallowedTools(t *testing.T) {
	c := New().DisallowedTools("Bash", "Edit")
	c.bin.Reset()
	c.prepareTools()
	args := c.bin.Args()
	assert.Contains(t, args, "--disallowedTools")
	assert.Contains(t, args, "Bash")
	assert.Contains(t, args, "Edit")
}

func TestPrepareTools_Tools(t *testing.T) {
	c := New().Tools("Read", "Grep")
	c.bin.Reset()
	c.prepareTools()
	args := c.bin.Args()
	assert.Contains(t, args, "--tools")
	assert.Contains(t, args, "Read,Grep")
}

func TestPrepareTools_AddDirs(t *testing.T) {
	c := New().AddDirs("/extra1", "/extra2")
	c.bin.Reset()
	c.prepareTools()
	args := c.bin.Args()
	assert.Contains(t, args, "--add-dir")
	assert.Contains(t, args, "/extra1")
	assert.Contains(t, args, "/extra2")
}

func TestPrepareTools_Empty(t *testing.T) {
	c := New()
	c.bin.Reset()
	c.prepareTools()
	args := c.bin.Args()
	assert.NotContains(t, args, "--allowedTools")
	assert.NotContains(t, args, "--disallowedTools")
	assert.NotContains(t, args, "--tools")
	assert.NotContains(t, args, "--add-dir")
}

func TestPrepareFlags_SkipPermissions(t *testing.T) {
	c := New()
	c.SkipPermissions()
	c.bin.Reset()
	c.prepareFlags()
	args := c.bin.Args()
	assert.Contains(t, args, "--dangerously-skip-permissions")
}

func TestPrepareFlags_NoPersistence(t *testing.T) {
	c := New().NoSessionPersistence()
	c.bin.Reset()
	c.prepareFlags()
	args := c.bin.Args()
	assert.Contains(t, args, "--no-session-persistence")
}

func TestPrepareFlags_Verbose(t *testing.T) {
	c := New().Verbose()
	c.bin.Reset()
	c.prepareFlags()
	args := c.bin.Args()
	assert.Contains(t, args, "--verbose")
}

func TestPrepareFlags_Empty(t *testing.T) {
	c := New()
	c.bin.Reset()
	c.prepareFlags()
	args := c.bin.Args()
	assert.NotContains(t, args, "--dangerously-skip-permissions")
	assert.NotContains(t, args, "--no-session-persistence")
	assert.NotContains(t, args, "--verbose")
}

func TestPrepare_FullConfig(t *testing.T) {
	c := New().
		Model("opus").
		FallbackModel("sonnet").
		SystemPrompt("sys").
		PermissionMode("plan").
		MCPConfig("/mcp.json").
		JSONSchema("{}").
		MaxTurns(5).
		MaxBudget(1.50).
		AllowedTools("Read").
		DisallowedTools("Bash").
		Tools("Read", "Grep").
		AddDirs("/extra").
		NoSessionPersistence().
		Verbose()
	c.AppendSystemPrompt("extra")
	c.SkipPermissions()
	c.Dir("/work")
	c.outputFormat = formatStreamJSON

	c.prepare("-p")

	args := c.bin.Args()
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "opus")
	assert.Contains(t, args, "--fallback-model")
	assert.Contains(t, args, "sonnet")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--system-prompt")
	assert.Contains(t, args, "--append-system-prompt")
	assert.Contains(t, args, "--permission-mode")
	assert.Contains(t, args, "--mcp-config")
	assert.Contains(t, args, "--json-schema")
	assert.Contains(t, args, "--max-turns")
	assert.Contains(t, args, "--max-budget-usd")
	assert.Contains(t, args, "--allowedTools")
	assert.Contains(t, args, "--disallowedTools")
	assert.Contains(t, args, "--tools")
	assert.Contains(t, args, "--add-dir")
	assert.Contains(t, args, "--dangerously-skip-permissions")
	assert.Contains(t, args, "--no-session-persistence")
	assert.Contains(t, args, "--verbose")
}

func TestPrepare_ResetsClearsOldArgs(t *testing.T) {
	c := New().Model("opus")
	c.outputFormat = formatStreamJSON

	c.prepare("-p")
	firstArgs := make([]string, len(c.bin.Args()))
	copy(firstArgs, c.bin.Args())

	c.prepare("-c", "-p")
	secondArgs := c.bin.Args()

	// Second prepare should have -c but first should not.
	assert.NotContains(t, firstArgs, "-c")
	assert.Contains(t, secondArgs, "-c")
	assert.Contains(t, secondArgs, "-p")
}

// writeFakeClaude writes an executable shell script that emits the given stdout
// verbatim, returning its path. Skips the test on Windows.
func writeFakeClaude(t *testing.T, stdout string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell-script binary not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	script := "#!/bin/sh\ncat <<'CLAUDE_EOF'\n" + stdout + "\nCLAUDE_EOF\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func TestOnMessage_StreamsLive(t *testing.T) {
	stdout := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"thinking"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}
{"type":"result","result":"done"}`

	path := writeFakeClaude(t, stdout)

	var got []string
	c := New().ExecPath(path)
	c.OnMessage(func(m string) { got = append(got, m) })

	result, err := c.Ask("hello")
	require.NoError(t, err)

	assert.Equal(t, []string{"thinking", "done"}, got)
	assert.Equal(t, "done", result.Text)
	assert.Equal(t, "thinking\ndone", result.FullText)
	assert.Equal(t, []string{"thinking", "done"}, result.Messages)
}

func TestOnMessage_NotSet_NoStreaming(t *testing.T) {
	stdout := `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}
{"type":"result","result":"hi"}`

	path := writeFakeClaude(t, stdout)

	c := New().ExecPath(path)
	result, err := c.Ask("hello")
	require.NoError(t, err)
	assert.Equal(t, "hi", result.Text)
}
