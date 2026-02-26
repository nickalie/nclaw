package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nickalie/go-binwrapper"
	"github.com/nickalie/nclaw/internal/cli"
)

// Compile-time check: *Codex implements cli.Client.
var _ cli.Client = (*Codex)(nil)

// Codex wraps the OpenAI Codex CLI binary.
type Codex struct {
	bin             *binwrapper.BinWrapper
	dir             string
	systemPrompt    string
	skipPermissions bool
}

// New creates a new Codex CLI wrapper.
func New() *Codex {
	bin := binwrapper.NewBinWrapper().
		ExecPath("codex").
		AutoExe()

	return &Codex{bin: bin}
}

// Dir sets the working directory for the codex process.
func (c *Codex) Dir(dir string) cli.Client {
	c.dir = dir
	return c
}

// SkipPermissions enables full-auto mode (no approval prompts).
func (c *Codex) SkipPermissions() cli.Client {
	c.skipPermissions = true
	return c
}

// AppendSystemPrompt sets a system prompt to be written to AGENTS.md
// in the working directory before invocation.
func (c *Codex) AppendSystemPrompt(prompt string) cli.Client {
	c.systemPrompt = prompt
	return c
}

// Ask sends a query in non-interactive mode and returns the response.
func (c *Codex) Ask(query string) (*cli.Result, error) {
	if err := c.writeSystemPrompt(); err != nil {
		return nil, fmt.Errorf("codex: write system prompt: %w", err)
	}

	c.prepare()
	return c.runAndParse(query)
}

// Continue sends a query resuming the most recent session.
func (c *Codex) Continue(query string) (*cli.Result, error) {
	if err := c.writeSystemPrompt(); err != nil {
		return nil, fmt.Errorf("codex: write system prompt: %w", err)
	}

	c.prepare("resume", "--last")
	return c.runAndParse(query)
}

// runAndParse executes the CLI and parses JSONL output into a Result.
func (c *Codex) runAndParse(query string) (*cli.Result, error) {
	if err := c.bin.Run(query); err != nil {
		result := parseJSONLOutput(c.bin.StdOut())
		if result.Text == "" && result.FullText == "" {
			text := strings.TrimSpace(string(c.bin.CombinedOutput()))
			result = &cli.Result{Text: text, FullText: text}
		}
		return result, fmt.Errorf("codex: %w", err)
	}

	return parseJSONLOutput(c.bin.StdOut()), nil
}

// Version returns the Codex CLI version string.
func (c *Codex) Version() (string, error) {
	c.bin.Reset()

	if err := c.bin.Run("--version"); err != nil {
		return strings.TrimSpace(string(c.bin.CombinedOutput())), fmt.Errorf("codex: %w", err)
	}

	return strings.TrimSpace(string(c.bin.StdOut())), nil
}

// writeSystemPrompt writes the system prompt to AGENTS.md in the working directory.
func (c *Codex) writeSystemPrompt() error {
	if c.systemPrompt == "" || c.dir == "" {
		return nil
	}

	path := filepath.Join(c.dir, "AGENTS.md")
	return os.WriteFile(path, []byte(c.systemPrompt), 0o644)
}

// prepare resets the binwrapper and rebuilds all arguments.
func (c *Codex) prepare(extra ...string) {
	c.bin.Reset()

	// Base command: codex exec
	c.bin.Arg("exec")

	// JSONL output
	c.bin.Arg("--json")

	if c.skipPermissions {
		c.bin.Arg("--full-auto")
	}

	if c.dir != "" {
		c.bin.Arg("--cd", c.dir)
	}

	for _, arg := range extra {
		c.bin.Arg(arg)
	}
}
