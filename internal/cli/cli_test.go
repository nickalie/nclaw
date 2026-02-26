package cli_test

import (
	"testing"

	"github.com/nickalie/nclaw/internal/cli"
	"github.com/stretchr/testify/assert"
)

// mockClient is a test mock that satisfies cli.Client.
type mockClient struct {
	dir          string
	skipPerms    bool
	systemPrompt string
}

var _ cli.Client = (*mockClient)(nil)

func (m *mockClient) Dir(dir string) cli.Client {
	m.dir = dir
	return m
}

func (m *mockClient) SkipPermissions() cli.Client {
	m.skipPerms = true
	return m
}

func (m *mockClient) AppendSystemPrompt(prompt string) cli.Client {
	m.systemPrompt = prompt
	return m
}

func (m *mockClient) Ask(query string) (*cli.Result, error) {
	return &cli.Result{Text: "ask: " + query, FullText: "ask: " + query}, nil
}

func (m *mockClient) Continue(query string) (*cli.Result, error) {
	return &cli.Result{Text: "continue: " + query, FullText: "continue: " + query}, nil
}

// mockProvider is a test mock that satisfies cli.Provider.
type mockProvider struct {
	preInvokeCalled bool
}

var _ cli.Provider = (*mockProvider)(nil)

func (m *mockProvider) NewClient() cli.Client {
	return &mockClient{}
}

func (m *mockProvider) PreInvoke() error {
	m.preInvokeCalled = true
	return nil
}

func (m *mockProvider) Version() (string, error) {
	return "1.0.0", nil
}

func (m *mockProvider) Name() string {
	return "mock"
}

func TestClientInterface(t *testing.T) {
	var client cli.Client = &mockClient{}

	client = client.Dir("/tmp")
	client = client.SkipPermissions()
	client = client.AppendSystemPrompt("test prompt")

	result, err := client.Ask("hello")
	assert.NoError(t, err)
	assert.Equal(t, "ask: hello", result.Text)
	assert.Equal(t, "ask: hello", result.FullText)

	result, err = client.Continue("world")
	assert.NoError(t, err)
	assert.Equal(t, "continue: world", result.Text)
}

func TestProviderInterface(t *testing.T) {
	provider := &mockProvider{}

	assert.Equal(t, "mock", provider.Name())

	version, err := provider.Version()
	assert.NoError(t, err)
	assert.Equal(t, "1.0.0", version)

	err = provider.PreInvoke()
	assert.NoError(t, err)
	assert.True(t, provider.preInvokeCalled)

	client := provider.NewClient()
	assert.NotNil(t, client)
}
