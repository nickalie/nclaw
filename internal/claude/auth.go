package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultTokenURL = "https://platform.claude.com/v1/oauth/token"
	clientID        = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	refreshMargin   = 5 * time.Minute
	credentialFile  = ".credentials.json"
)

var (
	refreshMu   sync.Mutex
	tokenURLVal = defaultTokenURL
)

type oauthTokens struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"`
	Scopes           []string `json:"scopes"`
	SubscriptionType *string  `json:"subscriptionType"`
	RateLimitTier    *string  `json:"rateLimitTier"`
}

type credentialsFile struct {
	ClaudeAiOauth *oauthTokens `json:"claudeAiOauth"`
}

type tokenRefreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	Scope        string `json:"scope"`
}

type tokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
}

// EnsureValidToken checks if the OAuth token is about to expire and refreshes it if needed.
// It returns nil if no refresh was needed or the refresh succeeded.
func EnsureValidToken() error {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	credsPath := credentialsPath()
	if credsPath == "" {
		return nil
	}

	creds, err := readCredentials(credsPath)
	if err != nil {
		return nil // no credentials file, let CLI handle auth
	}

	oauth := creds.ClaudeAiOauth
	if oauth == nil || oauth.RefreshToken == "" {
		return nil // no OAuth tokens or no refresh token
	}

	expiresAt := time.UnixMilli(oauth.ExpiresAt)
	if time.Until(expiresAt) > refreshMargin {
		return nil // token still valid
	}

	log.Printf("claude/auth: token expires at %s, refreshing...", expiresAt.Format(time.RFC3339))

	newTokens, err := refreshAccessToken(oauth.RefreshToken)
	if err != nil {
		return fmt.Errorf("claude/auth: refresh failed: %w", err)
	}

	oauth.AccessToken = newTokens.AccessToken
	oauth.RefreshToken = newTokens.RefreshToken
	oauth.ExpiresAt = time.Now().Add(time.Duration(newTokens.ExpiresIn) * time.Second).UnixMilli()

	if err := writeCredentials(credsPath, creds); err != nil {
		return fmt.Errorf("claude/auth: write credentials: %w", err)
	}

	log.Printf("claude/auth: token refreshed, new expiry: %s",
		time.UnixMilli(oauth.ExpiresAt).Format(time.RFC3339))

	return nil
}

func credentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", credentialFile)
}

func readCredentials(path string) (*credentialsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var creds credentialsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

func writeCredentials(path string, creds *credentialsFile) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func setTokenURL(url string) { tokenURLVal = url }

func refreshAccessToken(refreshToken string) (*tokenRefreshResponse, error) {
	reqBody := tokenRefreshRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		ClientID:     clientID,
		Scope:        "user:inference user:mcp_servers user:profile user:sessions:claude_code",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(tokenURLVal, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", tokenURLVal, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody json.RawMessage
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("token refresh returned %d: %s", resp.StatusCode, errBody)
	}

	var tokens tokenRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &tokens, nil
}
