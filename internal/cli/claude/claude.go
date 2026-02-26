package claude

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nickalie/go-binwrapper"
	"github.com/nickalie/nclaw/internal/cli"
	"github.com/nickalie/nclaw/internal/cli/streamjson"
)

// Compile-time check: *Claude implements cli.Client.
var _ cli.Client = (*Claude)(nil)

// outputFormat represents the output format for the CLI.
type outputFormat string

// formatStreamJSON is the output format used by query methods to capture multi-turn output.
const formatStreamJSON outputFormat = "stream-json"

// Claude wraps the Claude Code CLI binary.
type Claude struct {
	bin             *binwrapper.BinWrapper
	model           string
	fallbackModel   string
	outputFormat    outputFormat
	systemPrompt    string
	appendPrompt    string
	permissionMode  string
	mcpConfig       string
	jsonSchema      string
	maxTurns        int
	maxBudgetUSD    float64
	allowedTools    []string
	disallowedTools []string
	tools           []string
	dir             string
	addDirs         []string
	env             []string
	stdIn           io.Reader
	skipPermissions bool
	noPersistence   bool
	verbose         bool
}

// New creates a new Claude CLI wrapper.
func New() *Claude {
	bin := binwrapper.NewBinWrapper().
		ExecPath("claude").
		AutoExe()

	return &Claude{bin: bin}
}

// BinPath sets the directory containing the claude binary.
func (c *Claude) BinPath(path string) *Claude {
	c.bin.Dest(path)
	return c
}

// Dir sets the working directory for the claude process.
func (c *Claude) Dir(dir string) cli.Client {
	c.dir = dir
	return c
}

// Model sets the model for queries (e.g. "sonnet", "opus", or a full model name).
func (c *Claude) Model(model string) *Claude {
	c.model = model
	return c
}

// FallbackModel sets an automatic fallback model when the default is overloaded.
func (c *Claude) FallbackModel(model string) *Claude {
	c.fallbackModel = model
	return c
}

// SystemPrompt replaces the entire default system prompt.
func (c *Claude) SystemPrompt(prompt string) *Claude {
	c.systemPrompt = prompt
	return c
}

// AppendSystemPrompt appends custom text to the default system prompt.
func (c *Claude) AppendSystemPrompt(prompt string) cli.Client {
	c.appendPrompt = prompt
	return c
}

// PermissionMode sets the permission mode (e.g. "plan").
func (c *Claude) PermissionMode(mode string) *Claude {
	c.permissionMode = mode
	return c
}

// MCPConfig sets the path to MCP server configuration file.
func (c *Claude) MCPConfig(path string) *Claude {
	c.mcpConfig = path
	return c
}

// JSONSchema sets a JSON Schema for validated structured output.
func (c *Claude) JSONSchema(schema string) *Claude {
	c.jsonSchema = schema
	return c
}

// MaxTurns limits the number of agentic turns.
func (c *Claude) MaxTurns(n int) *Claude {
	c.maxTurns = n
	return c
}

// MaxBudget sets the maximum dollar amount to spend on API calls.
func (c *Claude) MaxBudget(usd float64) *Claude {
	c.maxBudgetUSD = usd
	return c
}

// AllowedTools sets tools that execute without prompting for permission.
func (c *Claude) AllowedTools(tools ...string) *Claude {
	c.allowedTools = tools
	return c
}

// DisallowedTools sets tools that are removed from the model's context.
func (c *Claude) DisallowedTools(tools ...string) *Claude {
	c.disallowedTools = tools
	return c
}

// Tools restricts which built-in tools Claude can use.
func (c *Claude) Tools(tools ...string) *Claude {
	c.tools = tools
	return c
}

// AddDirs adds additional working directories for Claude to access.
func (c *Claude) AddDirs(dirs ...string) *Claude {
	c.addDirs = dirs
	return c
}

// SkipPermissions enables skipping all permission prompts.
// Use with caution.
func (c *Claude) SkipPermissions() cli.Client {
	c.skipPermissions = true
	return c
}

// NoSessionPersistence disables session persistence so sessions are not saved to disk.
func (c *Claude) NoSessionPersistence() *Claude {
	c.noPersistence = true
	return c
}

// Verbose enables verbose logging output.
func (c *Claude) Verbose() *Claude {
	c.verbose = true
	return c
}

// Timeout sets the execution timeout for the claude process.
func (c *Claude) Timeout(d time.Duration) *Claude {
	c.bin.Timeout(d)
	return c
}

// StdIn sets the stdin reader for piping content to claude.
func (c *Claude) StdIn(r io.Reader) *Claude {
	c.stdIn = r
	return c
}

// Env sets environment variables for the claude process.
func (c *Claude) Env(vars []string) *Claude {
	c.env = vars
	return c
}

// Ask sends a query in print mode and returns the response.
// Uses stream-json output to capture all assistant messages across multi-turn execution.
func (c *Claude) Ask(query string) (*cli.Result, error) {
	c.outputFormat = formatStreamJSON
	c.verbose = true
	c.prepare("-p")
	return c.runAndParse(query)
}

// Continue sends a query continuing the most recent conversation in the current directory.
// Uses stream-json output to capture all assistant messages across multi-turn execution.
func (c *Claude) Continue(query string) (*cli.Result, error) {
	c.outputFormat = formatStreamJSON
	c.verbose = true
	c.prepare("-c", "-p")
	return c.runAndParse(query)
}

// Resume sends a query resuming a specific session by ID or name.
// Uses stream-json output to capture all assistant messages across multi-turn execution.
func (c *Claude) Resume(session, query string) (*cli.Result, error) {
	c.outputFormat = formatStreamJSON
	c.verbose = true
	c.prepare("-r", session, "-p")
	return c.runAndParse(query)
}

// runAndParse executes the CLI and parses stream-json output into a Result.
func (c *Claude) runAndParse(query string) (*cli.Result, error) {
	if err := c.bin.Run(query); err != nil {
		result := streamjson.ParseOutput(c.bin.StdOut())
		if result.Text == "" && result.FullText == "" {
			text := strings.TrimSpace(string(c.bin.CombinedOutput()))
			result = &cli.Result{Text: text, FullText: text}
		}
		return result, fmt.Errorf("claude: %w", err)
	}

	return streamjson.ParseOutput(c.bin.StdOut()), nil
}

// Version returns the Claude CLI version string.
func (c *Claude) Version() (string, error) {
	c.bin.Reset()

	if err := c.bin.Run("--version"); err != nil {
		return strings.TrimSpace(string(c.bin.CombinedOutput())), fmt.Errorf("claude: %w", err)
	}

	return strings.TrimSpace(string(c.bin.StdOut())), nil
}

// StdOut returns the raw stdout bytes from the last execution.
func (c *Claude) StdOut() []byte {
	return c.bin.StdOut()
}

// StdErr returns the raw stderr bytes from the last execution.
func (c *Claude) StdErr() []byte {
	return c.bin.StdErr()
}

// CombinedOutput returns the combined stdout and stderr from the last execution.
func (c *Claude) CombinedOutput() []byte {
	return c.bin.CombinedOutput()
}

// prepare resets the binwrapper and rebuilds all arguments from stored configuration.
func (c *Claude) prepare(extra ...string) {
	c.bin.Reset()

	for _, arg := range extra {
		c.bin.Arg(arg)
	}

	c.prepareIO()
	c.prepareModel()
	c.preparePrompt()
	c.prepareLimits()
	c.prepareTools()
	c.prepareFlags()
}

func (c *Claude) prepareIO() {
	if c.dir != "" {
		c.bin.Dir(c.dir)
	}

	if c.stdIn != nil {
		c.bin.StdIn(c.stdIn)
	}

	c.bin.Env(c.buildEnv())
}

func (c *Claude) buildEnv() []string {
	env := make([]string, 0, len(os.Environ())+len(c.env))

	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "CLAUDECODE=") {
			env = append(env, v)
		}
	}

	env = append(env, c.env...)
	return env
}

func (c *Claude) prepareModel() {
	if c.model != "" {
		c.bin.Arg("--model", c.model)
	}

	if c.fallbackModel != "" {
		c.bin.Arg("--fallback-model", c.fallbackModel)
	}

	if c.outputFormat != "" {
		c.bin.Arg("--output-format", string(c.outputFormat))
	}
}

func (c *Claude) preparePrompt() {
	if c.systemPrompt != "" {
		c.bin.Arg("--system-prompt", c.systemPrompt)
	}

	if c.appendPrompt != "" {
		c.bin.Arg("--append-system-prompt", c.appendPrompt)
	}

	if c.permissionMode != "" {
		c.bin.Arg("--permission-mode", c.permissionMode)
	}

	if c.mcpConfig != "" {
		c.bin.Arg("--mcp-config", c.mcpConfig)
	}

	if c.jsonSchema != "" {
		c.bin.Arg("--json-schema", c.jsonSchema)
	}
}

func (c *Claude) prepareLimits() {
	if c.maxTurns > 0 {
		c.bin.Arg("--max-turns", fmt.Sprintf("%d", c.maxTurns))
	}

	if c.maxBudgetUSD > 0 {
		c.bin.Arg("--max-budget-usd", fmt.Sprintf("%.2f", c.maxBudgetUSD))
	}
}

func (c *Claude) prepareTools() {
	for _, tool := range c.allowedTools {
		c.bin.Arg("--allowedTools", tool)
	}

	for _, tool := range c.disallowedTools {
		c.bin.Arg("--disallowedTools", tool)
	}

	if len(c.tools) > 0 {
		c.bin.Arg("--tools", strings.Join(c.tools, ","))
	}

	for _, dir := range c.addDirs {
		c.bin.Arg("--add-dir", dir)
	}
}

func (c *Claude) prepareFlags() {
	if c.skipPermissions {
		c.bin.Arg("--dangerously-skip-permissions")
	}

	if c.noPersistence {
		c.bin.Arg("--no-session-persistence")
	}

	if c.verbose {
		c.bin.Arg("--verbose")
	}
}
