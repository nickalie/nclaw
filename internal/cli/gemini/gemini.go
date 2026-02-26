package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nickalie/go-binwrapper"
	"github.com/nickalie/nclaw/internal/cli"
)

// Compile-time check: *Gemini implements cli.Client.
var _ cli.Client = (*Gemini)(nil)

// Gemini wraps the Google Gemini CLI binary.
type Gemini struct {
	bin             *binwrapper.BinWrapper
	dir             string
	systemPrompt    string
	skipPermissions bool
	model           string
}

// New creates a new Gemini CLI wrapper.
func New() *Gemini {
	bin := binwrapper.NewBinWrapper().
		ExecPath("gemini").
		AutoExe()

	return &Gemini{bin: bin}
}

// Dir sets the working directory for the gemini process.
func (g *Gemini) Dir(dir string) cli.Client {
	g.dir = dir
	return g
}

// SkipPermissions enables --approval-mode yolo (auto-approve all actions).
func (g *Gemini) SkipPermissions() cli.Client {
	g.skipPermissions = true
	return g
}

// AppendSystemPrompt sets a system prompt to be written to GEMINI.md
// in the working directory before invocation.
func (g *Gemini) AppendSystemPrompt(prompt string) cli.Client {
	g.systemPrompt = prompt
	return g
}

// Ask sends a query and returns the parsed stream-json response.
func (g *Gemini) Ask(query string) (*cli.Result, error) {
	if err := g.writeSystemPrompt(); err != nil {
		return &cli.Result{}, fmt.Errorf("gemini: write system prompt: %w", err)
	}

	g.prepare()
	return g.runAndParse(query)
}

// Continue sends a query resuming the most recent session.
func (g *Gemini) Continue(query string) (*cli.Result, error) {
	if err := g.writeSystemPrompt(); err != nil {
		return &cli.Result{}, fmt.Errorf("gemini: write system prompt: %w", err)
	}

	g.prepareContinue()
	return g.runAndParse(query)
}

// runAndParse executes the CLI and parses stream-json output into a Result.
func (g *Gemini) runAndParse(query string) (*cli.Result, error) {
	if err := g.bin.Run(query); err != nil {
		result := parseStreamJSONOutput(g.bin.StdOut())
		if result.Text == "" && result.FullText == "" {
			text := strings.TrimSpace(string(g.bin.CombinedOutput()))
			result = &cli.Result{Text: text, FullText: text}
		}

		return result, fmt.Errorf("gemini: %w", err)
	}

	return parseStreamJSONOutput(g.bin.StdOut()), nil
}

// Version returns the Gemini CLI version string.
func (g *Gemini) Version() (string, error) {
	g.bin.Reset()

	if err := g.bin.Run("--version"); err != nil {
		return strings.TrimSpace(string(g.bin.CombinedOutput())), fmt.Errorf("gemini: %w", err)
	}

	return strings.TrimSpace(string(g.bin.StdOut())), nil
}

// writeSystemPrompt writes the system prompt to GEMINI.md in the working directory.
func (g *Gemini) writeSystemPrompt() error {
	if g.systemPrompt == "" || g.dir == "" {
		return nil
	}

	path := filepath.Join(g.dir, "GEMINI.md")
	return os.WriteFile(path, []byte(g.systemPrompt), 0o644)
}

// prepare resets the binwrapper and rebuilds arguments for an Ask call.
func (g *Gemini) prepare() {
	g.bin.Reset()
	g.addCommonArgs()
}

// prepareContinue resets the binwrapper and rebuilds arguments for a Continue call.
func (g *Gemini) prepareContinue() {
	g.bin.Reset()
	g.bin.Arg("--resume", "latest")
	g.addCommonArgs()
}

// addCommonArgs adds flags shared between Ask and Continue.
func (g *Gemini) addCommonArgs() {
	if g.dir != "" {
		g.bin.Dir(g.dir)
	}

	g.bin.Arg("--output-format", "stream-json")

	if g.skipPermissions {
		g.bin.Arg("--approval-mode", "yolo")
	}

	if g.model != "" {
		g.bin.Arg("--model", g.model)
	}

	g.bin.Arg("-p")
}
