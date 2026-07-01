package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestTelegramBotToken(t *testing.T) {
	viper.Set("telegram.bot_token", "test-token-123")
	defer viper.Reset()

	assert.Equal(t, "test-token-123", TelegramBotToken())
}

func TestDataDir(t *testing.T) {
	viper.Set("data_dir", "/custom/data")
	defer viper.Reset()

	assert.Equal(t, "/custom/data", DataDir())
}

func TestDBPath_Default(t *testing.T) {
	viper.Set("data_dir", "/data")
	defer viper.Reset()

	assert.Equal(t, "/data/nclaw.db", DBPath())
}

func TestDBPath_Override(t *testing.T) {
	viper.Set("db_path", "/custom/path.db")
	defer viper.Reset()

	assert.Equal(t, "/custom/path.db", DBPath())
}

func TestTimezone_Default(t *testing.T) {
	viper.Reset()

	tz := Timezone()
	assert.NotEmpty(t, tz)
}

func TestTimezone_Configured(t *testing.T) {
	viper.Set("timezone", "Europe/Berlin")
	defer viper.Reset()

	assert.Equal(t, "Europe/Berlin", Timezone())
}

func TestInit_MissingRequired(t *testing.T) {
	viper.Reset()
	// Clear env vars so AutomaticEnv doesn't satisfy required keys.
	for _, key := range []string{"NCLAW_TELEGRAM_BOT_TOKEN", "NCLAW_DATA_DIR"} {
		t.Setenv(key, "")
	}

	err := Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is required")
}

func TestInit_AllRequired(t *testing.T) {
	viper.Reset()
	viper.Set("telegram.bot_token", "token")
	viper.Set("data_dir", "/tmp/test")
	defer viper.Reset()

	err := Init()
	assert.NoError(t, err)
}

func TestWhitelistChatIDs(t *testing.T) {
	viper.Set("telegram.whitelist_chat_ids", "111,222,333")
	defer viper.Reset()

	ids := WhitelistChatIDs()
	assert.Equal(t, []int64{111, 222, 333}, ids)
}

func TestWhitelistChatIDs_Single(t *testing.T) {
	viper.Set("telegram.whitelist_chat_ids", "42")
	defer viper.Reset()

	ids := WhitelistChatIDs()
	assert.Equal(t, []int64{42}, ids)
}

func TestWebhookBaseDomain(t *testing.T) {
	// Value should be a bare domain (no protocol) since WebhookURL prepends "https://".
	viper.Set("webhook.base_domain", "example.com")
	defer viper.Reset()

	assert.Equal(t, "example.com", WebhookBaseDomain())
}

func TestWebhookBaseDomain_Empty(t *testing.T) {
	viper.Reset()

	assert.Equal(t, "", WebhookBaseDomain())
}

func TestWebhookPort_Default(t *testing.T) {
	viper.Reset()

	assert.Equal(t, ":3000", WebhookPort())
}

func TestWebhookPort_Override(t *testing.T) {
	viper.Set("webhook.port", ":8080")
	defer viper.Reset()

	assert.Equal(t, ":8080", WebhookPort())
}

func TestCLI_Default(t *testing.T) {
	viper.Reset()

	assert.Equal(t, "claude", CLI())
}

func TestCLI_Configured(t *testing.T) {
	viper.Set("cli", "codex")
	defer viper.Reset()

	assert.Equal(t, "codex", CLI())
}

func TestCLI_CaseInsensitive(t *testing.T) {
	viper.Set("cli", "COPILOT")
	defer viper.Reset()

	assert.Equal(t, "copilot", CLI())
}

func TestCLI_AutoDetectClaudish(t *testing.T) {
	viper.Reset()
	viper.Set("model", "gpt-4o")
	defer viper.Reset()

	assert.Equal(t, "claudish", CLI())
}

func TestCLI_ExplicitOverridesAutoDetect(t *testing.T) {
	viper.Set("cli", "codex")
	viper.Set("model", "gpt-4o")
	defer viper.Reset()

	assert.Equal(t, "codex", CLI())
}

func TestCLI_ExplicitClaudishWithoutModel(t *testing.T) {
	viper.Set("cli", "claudish")
	defer viper.Reset()

	assert.Equal(t, "claudish", CLI())
}

func TestValidCLIBackends(t *testing.T) {
	backends := ValidCLIBackends()
	assert.Equal(t, []string{"claude", "claudish", "codex", "copilot", "gemini"}, backends)
}

func TestClaudeExecPath(t *testing.T) {
	viper.Set("claude_exec_path", "/opt/claude/bin/claude")
	defer viper.Reset()

	assert.Equal(t, "/opt/claude/bin/claude", ClaudeExecPath())
}

func TestModel(t *testing.T) {
	viper.Set("model", "gpt-4o")
	defer viper.Reset()

	assert.Equal(t, "gpt-4o", Model())
}

func TestModel_Empty(t *testing.T) {
	viper.Reset()

	assert.Equal(t, "", Model())
}

func TestModelOpus(t *testing.T) {
	viper.Set("model_opus", "claude-opus-4-6")
	defer viper.Reset()

	assert.Equal(t, "claude-opus-4-6", ModelOpus())
}

func TestModelSonnet(t *testing.T) {
	viper.Set("model_sonnet", "claude-sonnet-4-6")
	defer viper.Reset()

	assert.Equal(t, "claude-sonnet-4-6", ModelSonnet())
}

func TestModelHaiku(t *testing.T) {
	viper.Set("model_haiku", "claude-haiku-4-5-20251001")
	defer viper.Reset()

	assert.Equal(t, "claude-haiku-4-5-20251001", ModelHaiku())
}

func TestModelSubagent(t *testing.T) {
	viper.Set("model_subagent", "gpt-4o-mini")
	defer viper.Reset()

	assert.Equal(t, "gpt-4o-mini", ModelSubagent())
}

func TestLogSecurityWarnings_NoWhitelist(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	// Should not panic when no whitelist is configured.
	LogSecurityWarnings()
}

func TestLogSecurityWarnings_WithWhitelist(t *testing.T) {
	viper.Set("telegram.whitelist_chat_ids", "100,200")
	defer viper.Reset()

	// Should not panic when whitelist is configured.
	LogSecurityWarnings()
}

func TestWhitelistChatIDs_Empty(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ids := WhitelistChatIDs()
	assert.Empty(t, ids)
}

func TestWhitelistChatIDs_InvalidValues(t *testing.T) {
	viper.Set("telegram.whitelist_chat_ids", "abc,123,def")
	defer viper.Reset()

	ids := WhitelistChatIDs()
	assert.Equal(t, []int64{123}, ids)
}

func TestInit_PartialRequired(t *testing.T) {
	viper.Reset()
	// Set bot_token but not data_dir.
	t.Setenv("NCLAW_TELEGRAM_BOT_TOKEN", "some-token")
	t.Setenv("NCLAW_DATA_DIR", "")

	err := Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "data_dir")
}
