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

func withTestCredPath(t *testing.T, path string) {
	t.Helper()
	orig := credPathFunc
	credPathFunc = func() (string, error) { return path, nil }
	t.Cleanup(func() { credPathFunc = orig })
}

func withTestTokenURL(t *testing.T, url string) {
	t.Helper()
	orig := tokenURLVal
	tokenURLVal = url
	t.Cleanup(func() { tokenURLVal = orig })
}

func resetBackoff(t *testing.T) {
	t.Helper()
	orig := lastRefreshFail
	lastRefreshFail = time.Time{}
	t.Cleanup(func() { lastRefreshFail = orig })
}

func writeTestCreds(t *testing.T, path string, data map[string]any) {
	t.Helper()
	content, err := json.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

func TestEnsureValidToken_NoCredentialsFile(t *testing.T) {
	withTestCredPath(t, filepath.Join(t.TempDir(), "nonexistent.json"))
	require.NoError(t, EnsureValidToken())
}

func TestEnsureValidToken_TokenStillValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialFile)
	withTestCredPath(t, path)

	writeTestCreds(t, path, map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":  "valid-token",
			"refreshToken": "refresh-token",
			"expiresAt":    time.Now().Add(time.Hour).UnixMilli(),
		},
	})

	require.NoError(t, EnsureValidToken())
}

func TestEnsureValidToken_RefreshesExpiredToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialFile)
	withTestCredPath(t, path)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req tokenRefreshRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "old-refresh", req.RefreshToken)
		assert.Equal(t, clientID, req.ClientID)

		json.NewEncoder(w).Encode(tokenRefreshResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresIn:    3600,
		})
	}))
	defer server.Close()
	withTestTokenURL(t, server.URL)

	writeTestCreds(t, path, map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":  "old-access",
			"refreshToken": "old-refresh",
			"expiresAt":    time.Now().Add(-time.Minute).UnixMilli(),
			"unknownField": "should-stay",
		},
		"extraTopLevel": "preserved",
	})

	require.NoError(t, EnsureValidToken())

	// Verify credentials file was updated and unknown fields preserved
	updated, err := readJSONFile(path)
	require.NoError(t, err)

	var extraField string
	require.NoError(t, json.Unmarshal(updated["extraTopLevel"], &extraField))
	assert.Equal(t, "preserved", extraField)

	var oauth map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(updated["claudeAiOauth"], &oauth))

	var accessToken string
	require.NoError(t, json.Unmarshal(oauth["accessToken"], &accessToken))
	assert.Equal(t, "new-access", accessToken)

	var refreshToken string
	require.NoError(t, json.Unmarshal(oauth["refreshToken"], &refreshToken))
	assert.Equal(t, "new-refresh", refreshToken)

	// Verify unknown oauth field preserved
	var unknownField string
	require.NoError(t, json.Unmarshal(oauth["unknownField"], &unknownField))
	assert.Equal(t, "should-stay", unknownField)
}

func TestEnsureValidToken_NoRefreshToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialFile)
	withTestCredPath(t, path)

	writeTestCreds(t, path, map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken": "token",
			"expiresAt":   time.Now().Add(-time.Minute).UnixMilli(),
		},
	})

	require.NoError(t, EnsureValidToken())
}

func TestEnsureValidToken_EmptyTokensFromServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialFile)
	withTestCredPath(t, path)
	resetBackoff(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(tokenRefreshResponse{})
	}))
	defer server.Close()
	withTestTokenURL(t, server.URL)

	writeTestCreds(t, path, map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":  "old",
			"refreshToken": "old-refresh",
			"expiresAt":    time.Now().Add(-time.Minute).UnixMilli(),
		},
	})

	err := EnsureValidToken()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty tokens")
}

func TestEnsureValidToken_ZeroExpiresIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialFile)
	withTestCredPath(t, path)
	resetBackoff(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(tokenRefreshResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresIn:    0,
		})
	}))
	defer server.Close()
	withTestTokenURL(t, server.URL)

	writeTestCreds(t, path, map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":  "old",
			"refreshToken": "old-refresh",
			"expiresAt":    time.Now().Add(-time.Minute).UnixMilli(),
		},
	})

	err := EnsureValidToken()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-positive expires_in")
}

func TestRefreshAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req tokenRefreshRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "refresh_token", req.GrantType)
		assert.Equal(t, "my-refresh-token", req.RefreshToken)
		assert.Equal(t, clientID, req.ClientID)

		json.NewEncoder(w).Encode(tokenRefreshResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresIn:    86400,
		})
	}))
	defer server.Close()
	withTestTokenURL(t, server.URL)

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
	withTestTokenURL(t, server.URL)

	_, err := refreshAccessToken("bad-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestReadJSONFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.json")
	content := `{"claudeAiOauth":{"accessToken":"tok","unknownField":"val"},"otherKey":"other"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	got, err := readJSONFile(path)
	require.NoError(t, err)
	assert.Contains(t, got, "claudeAiOauth")
	assert.Contains(t, got, "otherKey")
}

func TestReadJSONFile_Nonexistent(t *testing.T) {
	_, err := readJSONFile(filepath.Join(t.TempDir(), "nonexistent.json"))
	assert.Error(t, err)
}

func TestReadJSONFile_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	_, err := readJSONFile(path)
	assert.Error(t, err)
}

func TestEnsureValidToken_BackoffAfterFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialFile)
	withTestCredPath(t, path)
	resetBackoff(t)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()
	withTestTokenURL(t, server.URL)

	writeTestCreds(t, path, map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":  "old",
			"refreshToken": "old-refresh",
			"expiresAt":    time.Now().Add(-time.Minute).UnixMilli(),
		},
	})

	// First call should attempt refresh and fail
	err := EnsureValidToken()
	assert.Error(t, err)
	assert.Equal(t, 1, callCount)

	// Second call should be skipped due to backoff
	err = EnsureValidToken()
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestAtomicWriteJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.json")

	data := map[string]json.RawMessage{
		"key": json.RawMessage(`"value"`),
	}
	require.NoError(t, atomicWriteJSON(path, data))

	got, err := readJSONFile(path)
	require.NoError(t, err)
	var val string
	require.NoError(t, json.Unmarshal(got["key"], &val))
	assert.Equal(t, "value", val)

	// Verify no temp file left behind
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))

	// Check file permissions
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
