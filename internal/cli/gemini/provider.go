package gemini

import "github.com/nickalie/nclaw/internal/cli"

// Provider implements cli.Provider for the Google Gemini CLI backend.
type Provider struct{}

// Compile-time check: *Provider implements cli.Provider.
var _ cli.Provider = (*Provider)(nil)

// NewProvider creates a new Gemini CLI provider.
func NewProvider() *Provider {
	return &Provider{}
}

// NewClient creates a new Gemini CLI client.
func (p *Provider) NewClient() cli.Client {
	return New()
}

// PreInvoke is a no-op for Gemini (no token refresh needed).
func (p *Provider) PreInvoke() error {
	return nil
}

// Version returns the Gemini CLI version string.
func (p *Provider) Version() (string, error) {
	return New().Version()
}

// Name returns the backend name.
func (p *Provider) Name() string {
	return "gemini"
}
