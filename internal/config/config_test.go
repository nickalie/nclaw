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

	err := Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is required")
}

func TestInit_AllRequired(t *testing.T) {
	viper.Reset()
	viper.Set("telegram.bot_token", "token")
	viper.Set("telegram.whitelist_chat_ids", "123")
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
