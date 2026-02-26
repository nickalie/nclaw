package claudish

import (
	"fmt"
	"os"
	"strings"

	"github.com/nickalie/go-binwrapper"
	"github.com/nickalie/nclaw/internal/cli"
	"github.com/nickalie/nclaw/internal/cli/streamjson"
)

// Compile-time check: *Claudish implements cli.Client.
var _ cli.Client = (*Claudish)(nil)

// Claudish wraps the claudish CLI binary, which proxies Claude Code to alternative model providers.
type Claudish struct {
	bin             *binwrapper.BinWrapper
	dir             string
	systemPrompt    string
	skipPermissions bool
	model           string
	modelOpus       string
	modelSonnet     string
	modelHaiku      string
	modelSubagent   string
}

// New creates a new Claudish CLI wrapper.
func New() *Claudish {
	bin := binwrapper.NewBinWrapper().
		ExecPath("claudish").
		AutoExe()

	return &Claudish{bin: bin}
}

// Dir sets the working directory for the claudish process.
func (c *Claudish) Dir(dir string) cli.Client {
	c.dir = dir
	return c
}

// SkipPermissions enables skipping all permission prompts.
func (c *Claudish) SkipPermissions() cli.Client {
	c.skipPermissions = true
	return c
}

// AppendSystemPrompt appends custom text to the default system prompt.
func (c *Claudish) AppendSystemPrompt(prompt string) cli.Client {
	c.systemPrompt = prompt
	return c
}

// Ask sends a query in print mode and returns the response.
func (c *Claudish) Ask(query string) (*cli.Result, error) {
	c.prepare("-p")
	return c.runAndParse(query)
}

// Continue sends a query continuing the most recent conversation.
func (c *Claudish) Continue(query string) (*cli.Result, error) {
	c.prepare("-c", "-p")
	return c.runAndParse(query)
}

// runAndParse executes the CLI and parses stream-json output into a Result.
func (c *Claudish) runAndParse(query string) (*cli.Result, error) {
	if err := c.bin.Run(query); err != nil {
		result := streamjson.ParseOutput(c.bin.StdOut())
		if result.Text == "" && result.FullText == "" {
			text := c.sanitizeOutput(string(c.bin.CombinedOutput()))
			result = &cli.Result{Text: text, FullText: text}
		}
		return result, fmt.Errorf("claudish: %w", err)
	}

	return streamjson.ParseOutput(c.bin.StdOut()), nil
}

// sanitizeOutput strips lines containing sensitive variable names from error output
// to prevent accidental disclosure via Telegram messages.
func (c *Claudish) sanitizeOutput(output string) string {
	lines := strings.Split(output, "\n")
	filtered := make([]string, 0, len(lines))

	for _, line := range lines {
		if !strings.Contains(line, "_API_KEY") {
			filtered = append(filtered, line)
		}
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

// Version returns the claudish CLI version string.
func (c *Claudish) Version() (string, error) {
	c.bin.Reset()

	if err := c.bin.Run("--version"); err != nil {
		return strings.TrimSpace(string(c.bin.CombinedOutput())), fmt.Errorf("claudish: %w", err)
	}

	return strings.TrimSpace(string(c.bin.StdOut())), nil
}

// prepare resets the binwrapper and rebuilds all arguments from stored configuration.
func (c *Claudish) prepare(extra ...string) {
	c.bin.Reset()

	for _, arg := range extra {
		c.bin.Arg(arg)
	}

	c.prepareIO()
	c.prepareModel()
	c.preparePrompt()
	c.prepareFlags()
}

func (c *Claudish) prepareIO() {
	if c.dir != "" {
		c.bin.Dir(c.dir)
	}

	c.bin.Env(c.buildEnv())
}

func (c *Claudish) buildEnv() []string {
	overrides := c.claudishOverrideKeys()
	env := make([]string, 0, len(os.Environ())+8)

	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		if key == "CLAUDECODE" || overrides[key] {
			continue
		}
		env = append(env, v)
	}

	return append(env, c.claudishEnvVars()...)
}

// claudishOverrideKeys returns the set of env var keys that have nclaw-configured replacements.
// Only keys with non-empty values are included, so bare env vars pass through when nclaw has no override.
func (c *Claudish) claudishOverrideKeys() map[string]bool {
	pairs := c.claudishPairs()
	keys := make(map[string]bool, len(pairs))
	for _, p := range pairs {
		if p.val != "" {
			keys[p.key] = true
		}
	}
	return keys
}

type envPair struct{ key, val string }

func (c *Claudish) claudishPairs() []envPair {
	return []envPair{
		{"CLAUDISH_MODEL_OPUS", c.modelOpus},
		{"CLAUDISH_MODEL_SONNET", c.modelSonnet},
		{"CLAUDISH_MODEL_HAIKU", c.modelHaiku},
		{"CLAUDISH_MODEL_SUBAGENT", c.modelSubagent},
	}
}

// claudishEnvVars returns environment variables for claudish API keys and model tiers.
func (c *Claudish) claudishEnvVars() []string {
	var vars []string
	for _, p := range c.claudishPairs() {
		if p.val != "" {
			vars = append(vars, p.key+"="+p.val)
		}
	}

	return vars
}

func (c *Claudish) prepareModel() {
	if c.model != "" {
		c.bin.Arg("--model", c.model)
	}

	c.bin.Arg("--output-format", "stream-json")
}

func (c *Claudish) preparePrompt() {
	if c.systemPrompt != "" {
		c.bin.Arg("--append-system-prompt", c.systemPrompt)
	}
}

func (c *Claudish) prepareFlags() {
	if c.skipPermissions {
		c.bin.Arg("--dangerously-skip-permissions")
	}

	c.bin.Arg("--verbose")
}
