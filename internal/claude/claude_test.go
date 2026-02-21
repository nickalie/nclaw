package claude

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	c := New()
	assert.NotNil(t, c)
	assert.NotNil(t, c.bin)
}

func TestBuilderChaining(t *testing.T) {
	c := New()

	// Each builder method should return the same instance.
	assert.Same(t, c, c.Dir("/tmp"))
	assert.Same(t, c, c.Model("opus"))
	assert.Same(t, c, c.FallbackModel("sonnet"))
	assert.Same(t, c, c.SystemPrompt("sys"))
	assert.Same(t, c, c.AppendSystemPrompt("extra"))
	assert.Same(t, c, c.PermissionMode("plan"))
	assert.Same(t, c, c.MCPConfig("/config.json"))
	assert.Same(t, c, c.JSONSchema("{}"))
	assert.Same(t, c, c.MaxTurns(5))
	assert.Same(t, c, c.MaxBudget(1.50))
	assert.Same(t, c, c.AllowedTools("Read", "Write"))
	assert.Same(t, c, c.DisallowedTools("Bash"))
	assert.Same(t, c, c.Tools("Read"))
	assert.Same(t, c, c.AddDirs("/extra"))
	assert.Same(t, c, c.SkipPermissions())
	assert.Same(t, c, c.NoSessionPersistence())
	assert.Same(t, c, c.Verbose())
	assert.Same(t, c, c.Timeout(30*time.Second))
	assert.Same(t, c, c.StdIn(strings.NewReader("input")))
	assert.Same(t, c, c.Env([]string{"FOO=bar"}))
}

func TestBuilderFieldValues(t *testing.T) {
	c := New().
		Dir("/work").
		Model("opus").
		FallbackModel("sonnet").
		SystemPrompt("system").
		AppendSystemPrompt("append").
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
