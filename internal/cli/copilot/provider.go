package copilot

import "github.com/nickalie/nclaw/internal/cli"

// Provider implements cli.Provider for the GitHub Copilot CLI backend.
type Provider struct{}

// Compile-time check: *Provider implements cli.Provider.
var _ cli.Provider = (*Provider)(nil)

// NewProvider creates a new Copilot CLI provider.
func NewProvider() *Provider {
	return &Provider{}
}

// NewClient creates a new Copilot CLI client.
func (p *Provider) NewClient() cli.Client {
	return New()
}

// PreInvoke is a no-op for Copilot (no token refresh needed).
func (p *Provider) PreInvoke() error {
	return nil
}

// Version returns the Copilot CLI version string.
func (p *Provider) Version() (string, error) {
	return New().Version()
}

// Name returns the backend name.
func (p *Provider) Name() string {
	return "copilot"
}
