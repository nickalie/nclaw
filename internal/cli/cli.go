package cli

// Result holds the output from a CLI invocation.
type Result struct {
	// Text is the final assistant message (suitable for display).
	Text string
	// FullText contains all assistant messages concatenated.
	// Useful for scanning command blocks (sendfile, schedule, webhook)
	// that may appear in non-final messages during multi-turn execution.
	FullText string
	// Messages holds each individual assistant message in order. Used when
	// stream-message output is enabled to deliver every message separately
	// instead of only the final one. May be empty for backends or code paths
	// that produce a single message (consumers fall back to Text).
	Messages []string
}

// Client is a per-request builder for invoking a CLI backend.
type Client interface {
	Dir(dir string) Client
	SkipPermissions() Client
	AppendSystemPrompt(prompt string) Client
	Ask(query string) (*Result, error)
	Continue(query string) (*Result, error)
}

// MessageHandler receives each assistant message as soon as it is parsed from a
// backend's streaming output. It is invoked sequentially, in order, while the
// CLI process is still running.
type MessageHandler func(message string)

// StreamingClient is an optional interface a Client may implement to deliver
// assistant messages incrementally as they arrive, enabling real-time output
// instead of a single batched result at the end. Backends that cannot stream
// (plain-text output) simply do not implement it.
type StreamingClient interface {
	OnMessage(handler MessageHandler) Client
}

// Provider is a singleton per backend that creates clients and handles
// backend-specific lifecycle tasks.
type Provider interface {
	NewClient() Client
	PreInvoke() error // e.g., token refresh; no-op for codex/copilot
	Version() (string, error)
	Name() string
}
