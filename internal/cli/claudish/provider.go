package claudish

import "github.com/nickalie/nclaw/internal/cli"

// Provider implements cli.Provider for the claudish CLI backend.
type Provider struct {
	model         string
	modelOpus     string
	modelSonnet   string
	modelHaiku    string
	modelSubagent string
}

// Compile-time check: *Provider implements cli.Provider.
var _ cli.Provider = (*Provider)(nil)

// NewProvider creates a new claudish CLI provider with model configuration.
func NewProvider(model, modelOpus, modelSonnet, modelHaiku, modelSubagent string) *Provider {
	return &Provider{
		model:         model,
		modelOpus:     modelOpus,
		modelSonnet:   modelSonnet,
		modelHaiku:    modelHaiku,
		modelSubagent: modelSubagent,
	}
}

// NewClient creates a new claudish CLI client with model config pre-set.
func (p *Provider) NewClient() cli.Client {
	c := New()
	c.model = p.model
	c.modelOpus = p.modelOpus
	c.modelSonnet = p.modelSonnet
	c.modelHaiku = p.modelHaiku
	c.modelSubagent = p.modelSubagent
	return c
}

// PreInvoke is a no-op for claudish (it manages its own proxy/auth).
func (p *Provider) PreInvoke() error {
	return nil
}

// Version returns the claudish CLI version string.
func (p *Provider) Version() (string, error) {
	return New().Version()
}

// Name returns the backend name.
func (p *Provider) Name() string {
	return "claudish"
}
