package codex

import "github.com/nickalie/nclaw/internal/cli"

// Provider implements cli.Provider for the OpenAI Codex CLI backend.
type Provider struct{}

// Compile-time check: *Provider implements cli.Provider.
var _ cli.Provider = (*Provider)(nil)

// NewProvider creates a new Codex CLI provider.
func NewProvider() *Provider {
	return &Provider{}
}

// NewClient creates a new Codex CLI client.
func (p *Provider) NewClient() cli.Client {
	return New()
}

// PreInvoke is a no-op for Codex (no token refresh needed).
func (p *Provider) PreInvoke() error {
	return nil
}

// Version returns the Codex CLI version string.
func (p *Provider) Version() (string, error) {
	return New().Version()
}

// Name returns the backend name.
func (p *Provider) Name() string {
	return "codex"
}
