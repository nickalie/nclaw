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
	refreshMu    sync.Mutex
	tokenURLVal  = defaultTokenURL
	credPathFunc = defaultCredentialsPath
	httpClient   = &http.Client{Timeout: 15 * time.Second}
)

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
}

// EnsureValidToken checks if the OAuth token is about to expire and refreshes it if needed.
func EnsureValidToken() error {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	credsPath, err := credPathFunc()
	if err != nil {
		log.Printf("claude/auth: credentials path: %v", err)
		return nil
	}

	return refreshIfNeeded(credsPath)
}

func refreshIfNeeded(credsPath string) error {
	root, oauth, refreshToken, expiresAt, err := loadTokenState(credsPath)
	if err != nil || refreshToken == "" {
		return nil
	}

	if time.Until(time.UnixMilli(expiresAt)) > refreshMargin {
		return nil
	}

	log.Printf("claude/auth: token expires at %s, refreshing...",
		time.UnixMilli(expiresAt).Format(time.RFC3339))

	return performRefresh(credsPath, root, oauth, refreshToken)
}

func loadTokenState(path string) (root, oauth map[string]json.RawMessage, refreshToken string, expiresAt int64, err error) {
	root, err = readJSONFile(path)
	if err != nil {
		return nil, nil, "", 0, err
	}

	oauthRaw, ok := root["claudeAiOauth"]
	if !ok {
		return nil, nil, "", 0, fmt.Errorf("no claudeAiOauth key")
	}

	if err = json.Unmarshal(oauthRaw, &oauth); err != nil {
		return nil, nil, "", 0, err
	}

	refreshToken, expiresAt = extractTokenInfo(oauth)
	return root, oauth, refreshToken, expiresAt, nil
}

func performRefresh(credsPath string, root, oauth map[string]json.RawMessage, refreshToken string) error {
	newTokens, err := refreshAccessToken(refreshToken)
	if err != nil {
		return fmt.Errorf("claude/auth: refresh failed: %w", err)
	}

	if newTokens.AccessToken == "" || newTokens.RefreshToken == "" {
		return fmt.Errorf("claude/auth: refresh returned empty tokens")
	}

	newExpiry := time.Now().Add(time.Duration(newTokens.ExpiresIn) * time.Second).UnixMilli()
	setRawField(oauth, "accessToken", newTokens.AccessToken)
	setRawField(oauth, "refreshToken", newTokens.RefreshToken)
	setRawField(oauth, "expiresAt", newExpiry)

	updatedOAuth, _ := json.Marshal(oauth)
	root["claudeAiOauth"] = updatedOAuth

	if err := atomicWriteJSON(credsPath, root); err != nil {
		return fmt.Errorf("claude/auth: write credentials: %w", err)
	}

	log.Printf("claude/auth: token refreshed, new expiry: %s",
		time.UnixMilli(newExpiry).Format(time.RFC3339))

	return nil
}

func extractTokenInfo(oauth map[string]json.RawMessage) (refreshToken string, expiresAt int64) {
	if raw, ok := oauth["refreshToken"]; ok {
		_ = json.Unmarshal(raw, &refreshToken)
	}

	if raw, ok := oauth["expiresAt"]; ok {
		_ = json.Unmarshal(raw, &expiresAt)
	}

	return refreshToken, expiresAt
}

func setRawField(m map[string]json.RawMessage, key string, value any) {
	data, _ := json.Marshal(value)
	m[key] = data
}

func readJSONFile(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func atomicWriteJSON(path string, data map[string]json.RawMessage) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

func defaultCredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".claude", credentialFile), nil
}

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

	resp, err := httpClient.Post(tokenURLVal, "application/json", bytes.NewReader(body))
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
