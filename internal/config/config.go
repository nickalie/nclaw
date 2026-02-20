package config

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var requiredKeys = []string{
	"telegram.bot_token",
	"telegram.whitelist_chat_ids",
	"data_dir",
}

// Init loads configuration from files and environment variables.
func Init() error {
	_ = godotenv.Load()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.nclaw")
	viper.AutomaticEnv()
	viper.SetEnvPrefix("NCLAW")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	for _, key := range requiredKeys {
		if viper.GetString(key) == "" {
			envKey := "NCLAW_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
			return fmt.Errorf("%s is required (set %s env var or %s in config)", key, envKey, key)
		}
	}

	return nil
}

// TelegramBotToken returns the configured Telegram bot token.
func TelegramBotToken() string {
	return viper.GetString("telegram.bot_token")
}

// WhitelistChatIDs returns the list of allowed Telegram chat IDs.
func WhitelistChatIDs() []int64 {
	raw := viper.GetString("telegram.whitelist_chat_ids")
	var ids []int64
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			ids = append(ids, id)
		} else {
			log.Printf("config: ignoring invalid whitelist chat ID %q: %v", s, err)
		}
	}
	return ids
}

// DataDir returns the configured data directory path.
func DataDir() string {
	return viper.GetString("data_dir")
}

// DBPath returns the path to the SQLite database file.
func DBPath() string {
	if p := viper.GetString("db_path"); p != "" {
		return p
	}
	return filepath.Join(DataDir(), "nclaw.db")
}

// WebhookBaseDomain returns the configured base domain for webhook URLs.
func WebhookBaseDomain() string {
	return viper.GetString("webhook.base_domain")
}

// WebhookPort returns the configured webhook server listen address, defaulting to ":3000".
func WebhookPort() string {
	if p := viper.GetString("webhook.port"); p != "" {
		return p
	}
	return ":3000"
}

// Timezone returns the configured timezone name, defaulting to system local.
func Timezone() string {
	if tz := viper.GetString("timezone"); tz != "" {
		return tz
	}
	return time.Now().Location().String()
}
