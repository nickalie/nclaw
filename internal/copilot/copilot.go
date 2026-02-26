package copilot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nickalie/go-binwrapper"
	"github.com/nickalie/nclaw/internal/cli"
)

// Compile-time check: *Copilot implements cli.Client.
var _ cli.Client = (*Copilot)(nil)

// Copilot wraps the GitHub Copilot CLI binary.
type Copilot struct {
	bin             *binwrapper.BinWrapper
	dir             string
	systemPrompt    string
	skipPermissions bool
}

// New creates a new Copilot CLI wrapper.
func New() *Copilot {
	bin := binwrapper.NewBinWrapper().
		ExecPath("copilot").
		AutoExe()

	return &Copilot{bin: bin}
}

// Dir sets the working directory for the copilot process.
func (c *Copilot) Dir(dir string) cli.Client {
	c.dir = dir
	return c
}

// SkipPermissions enables allow-all-tools and no-ask-user flags.
func (c *Copilot) SkipPermissions() cli.Client {
	c.skipPermissions = true
	return c
}

// AppendSystemPrompt sets a system prompt to be written to
// .github/copilot-instructions.md in the working directory before invocation.
func (c *Copilot) AppendSystemPrompt(prompt string) cli.Client {
	c.systemPrompt = prompt
	return c
}

// Ask sends a query and returns the plain text response.
func (c *Copilot) Ask(query string) (*cli.Result, error) {
	if err := c.writeSystemPrompt(); err != nil {
		return &cli.Result{}, fmt.Errorf("copilot: write system prompt: %w", err)
	}

	c.prepare()
	return c.runAndParse(query)
}

// Continue sends a query resuming the most recent session.
func (c *Copilot) Continue(query string) (*cli.Result, error) {
	if err := c.writeSystemPrompt(); err != nil {
		return &cli.Result{}, fmt.Errorf("copilot: write system prompt: %w", err)
	}

	c.prepareContinue()
	return c.runAndParse(query)
}

// runAndParse executes the CLI and returns plain text output as a Result.
// Copilot's -s flag outputs only the final text, so Text == FullText.
func (c *Copilot) runAndParse(query string) (*cli.Result, error) {
	if err := c.bin.Run(query); err != nil {
		text := strings.TrimSpace(string(c.bin.CombinedOutput()))
		return &cli.Result{Text: text, FullText: text}, fmt.Errorf("copilot: %w", err)
	}

	text := strings.TrimSpace(string(c.bin.StdOut()))
	return &cli.Result{Text: text, FullText: text}, nil
}

// Version returns the Copilot CLI version string.
func (c *Copilot) Version() (string, error) {
	c.bin.Reset()

	if err := c.bin.Run("version"); err != nil {
		return strings.TrimSpace(string(c.bin.CombinedOutput())), fmt.Errorf("copilot: %w", err)
	}

	return strings.TrimSpace(string(c.bin.StdOut())), nil
}

// writeSystemPrompt writes the system prompt to .github/copilot-instructions.md
// in the working directory.
func (c *Copilot) writeSystemPrompt() error {
	if c.systemPrompt == "" || c.dir == "" {
		return nil
	}

	dir := filepath.Join(c.dir, ".github")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .github dir: %w", err)
	}

	path := filepath.Join(dir, "copilot-instructions.md")
	return os.WriteFile(path, []byte(c.systemPrompt), 0o644)
}

// prepare resets the binwrapper and rebuilds arguments for an Ask call.
func (c *Copilot) prepare() {
	c.bin.Reset()
	c.addCommonArgs()
}

// prepareContinue resets the binwrapper and rebuilds arguments for a Continue call.
func (c *Copilot) prepareContinue() {
	c.bin.Reset()
	c.bin.Arg("--continue")
	c.addCommonArgs()
}

// addCommonArgs adds flags shared between Ask and Continue.
func (c *Copilot) addCommonArgs() {
	if c.dir != "" {
		c.bin.Dir(c.dir)
	}

	// Silent mode: output only final text.
	c.bin.Arg("-s")

	if c.skipPermissions {
		c.bin.Arg("--allow-all-tools")
		c.bin.Arg("--no-ask-user")
	}

	c.bin.Arg("-p")
}
