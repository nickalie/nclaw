package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/nickalie/nclaw/internal/model"
	"github.com/nickalie/nclaw/internal/pipeline"
	"github.com/nickalie/nclaw/internal/sendfile"
	"github.com/nickalie/nclaw/internal/telegram"
)

func setupTestServer(t *testing.T) (*Server, *Manager) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.WebhookRegistration{}))

	send := func(_ context.Context, _ int64, _ int, _, _ string) error { return nil }
	mgr := NewManager(database, &mockProvider{}, "example.com", t.TempDir(), telegram.NewChatLocker())
	mgr.SetPipeline(pipeline.New(send, sendfile.Senders{}, true))
	srv := NewServer(mgr)
	return srv, mgr
}

func TestServer_WebhookRoute_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/nonexistent-uuid", nil)
	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_WebhookRoute_ActiveWebhook(t *testing.T) {
	srv, mgr := setupTestServer(t)
	defer mgr.Wait()

	wh, err := mgr.Create("test hook", 100, 0)
	require.NoError(t, err)

	body := strings.NewReader(`{"event":"push","ref":"refs/heads/main"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+wh.ID, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "test-value")

	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_WebhookRoute_InactiveWebhook(t *testing.T) {
	srv, mgr := setupTestServer(t)

	wh, err := mgr.Create("paused hook", 100, 0)
	require.NoError(t, err)
	mgr.db.Model(&model.WebhookRegistration{}).Where("id = ?", wh.ID).Update("status", model.WebhookStatusPaused)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+wh.ID, nil)
	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_WebhookRoute_GET(t *testing.T) {
	srv, mgr := setupTestServer(t)
	defer mgr.Wait()

	wh, err := mgr.Create("get hook", 100, 0)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/"+wh.ID+"?param=value&foo=bar", nil)
	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_WebhookRoute_PUT(t *testing.T) {
	srv, mgr := setupTestServer(t)
	defer mgr.Wait()

	wh, err := mgr.Create("put hook", 100, 0)
	require.NoError(t, err)

	body := strings.NewReader("updated data")
	req := httptest.NewRequest(http.MethodPut, "/webhooks/"+wh.ID, body)
	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_Shutdown(t *testing.T) {
	srv, _ := setupTestServer(t)
	err := srv.Shutdown()
	assert.NoError(t, err)
}

func TestServer_UnknownRoute(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestExtractHeaders(t *testing.T) {
	srv, mgr := setupTestServer(t)
	defer mgr.Wait()

	wh, err := mgr.Create("header test", 100, 0)
	require.NoError(t, err)

	body := strings.NewReader("test body")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+wh.ID, body)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Test", "hello")

	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_WebhookRoute_Busy(t *testing.T) {
	srv, mgr := setupTestServer(t)

	wh, err := mgr.Create("busy hook", 100, 0)
	require.NoError(t, err)

	// Fill the semaphore to capacity.
	for range maxConcurrentWebhooks {
		mgr.sem <- struct{}{}
	}
	defer func() {
		for range maxConcurrentWebhooks {
			<-mgr.sem
		}
	}()

	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+wh.ID, nil)
	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestServer_ResponseBody(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/nonexistent", nil)
	resp, err := srv.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "Not Found", string(bodyBytes))
}
