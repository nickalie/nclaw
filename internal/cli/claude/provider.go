package claude

import "github.com/nickalie/nclaw/internal/cli"

// Provider implements cli.Provider for the Claude Code CLI backend.
type Provider struct{}

// Compile-time check: *Provider implements cli.Provider.
var _ cli.Provider = (*Provider)(nil)

// NewProvider creates a new Claude CLI provider.
func NewProvider() *Provider {
	return &Provider{}
}

// NewClient creates a new Claude CLI client.
func (p *Provider) NewClient() cli.Client {
	return New()
}

// PreInvoke refreshes the OAuth token if needed before each CLI invocation.
func (p *Provider) PreInvoke() error {
	return EnsureValidToken()
}

// Version returns the Claude CLI version string.
func (p *Provider) Version() (string, error) {
	return New().Version()
}

// Name returns the backend name.
func (p *Provider) Name() string {
	return "claude"
}
