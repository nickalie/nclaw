package claude

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialFile)

	creds := &credentialsFile{
		ClaudeAiOauth: &oauthTokens{
			AccessToken:  "access",
			RefreshToken: "refresh",
			ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
			Scopes:       []string{"user:inference"},
		},
	}

	data, err := json.Marshal(creds)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))

	got, err := readCredentials(path)
	require.NoError(t, err)
	assert.Equal(t, "access", got.ClaudeAiOauth.AccessToken)
	assert.Equal(t, "refresh", got.ClaudeAiOauth.RefreshToken)
}

func TestWriteCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialFile)

	creds := &credentialsFile{
		ClaudeAiOauth: &oauthTokens{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresAt:    1234567890000,
			Scopes:       []string{"user:inference"},
		},
	}

	require.NoError(t, writeCredentials(path, creds))

	got, err := readCredentials(path)
	require.NoError(t, err)
	assert.Equal(t, "new-access", got.ClaudeAiOauth.AccessToken)
	assert.Equal(t, "new-refresh", got.ClaudeAiOauth.RefreshToken)

	// Check file permissions
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestRefreshAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req tokenRefreshRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "refresh_token", req.GrantType)
		assert.Equal(t, "my-refresh-token", req.RefreshToken)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tokenRefreshResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresIn:    86400,
		})
	}))
	defer server.Close()

	// Temporarily override the token URL for testing
	origURL := tokenURLVal
	defer func() { setTokenURL(origURL) }()
	setTokenURL(server.URL)

	tokens, err := refreshAccessToken("my-refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "new-access", tokens.AccessToken)
	assert.Equal(t, "new-refresh", tokens.RefreshToken)
	assert.Equal(t, int64(86400), tokens.ExpiresIn)
}

func TestRefreshAccessToken_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	origURL := tokenURLVal
	defer func() { setTokenURL(origURL) }()
	setTokenURL(server.URL)

	_, err := refreshAccessToken("bad-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}
