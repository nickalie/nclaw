package cli

// Result holds the output from a CLI invocation.
type Result struct {
	// Text is the final assistant message (suitable for display).
	Text string
	// FullText contains all assistant messages concatenated.
	// Useful for scanning command blocks (sendfile, schedule, webhook)
	// that may appear in non-final messages during multi-turn execution.
	FullText string
}

// Client is a per-request builder for invoking a CLI backend.
type Client interface {
	Dir(dir string) Client
	SkipPermissions() Client
	AppendSystemPrompt(prompt string) Client
	Ask(query string) (*Result, error)
	Continue(query string) (*Result, error)
}

// Provider is a singleton per backend that creates clients and handles
// backend-specific lifecycle tasks.
type Provider interface {
	NewClient() Client
	PreInvoke() error // e.g., token refresh; no-op for codex/copilot
	Version() (string, error)
	Name() string
}
