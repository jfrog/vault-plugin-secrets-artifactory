package artifactory

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/logical"
)

// TestPerformArtifactory_NilHTTPClient_ReturnsError verifies that each
// performArtifactory* helper returns an error (instead of panicking) when the
// backend's HTTP client is nil.
//
// This is the state a standby / performance-standby / replica node ends up in:
// a change to the `config` storage key calls invalidate("config") -> reset(),
// which sets b.httpClient = nil. The client is only (re)initialised by
// initialize() at process start and by pathConfigUpdate() on a write, neither
// of which runs on those nodes, so the client stays nil until the next restart.
func TestPerformArtifactory_NilHTTPClient_ReturnsError(t *testing.T) {
	b, _ := makeBackend(t)

	// A change to the `config` key calls invalidate("config") -> reset(), which
	// nils the HTTP client; on standby/replica nodes it is never re-initialised.
	b.reset()

	config := baseConfiguration{
		AccessToken:    "dummy-non-empty-token",
		ArtifactoryURL: "http://myserver.com",
	}

	t.Run("Get", func(t *testing.T) {
		if _, err := b.performArtifactoryGet(config, "/artifactory/api/system/version"); err == nil {
			t.Fatal("expected an error with a nil httpClient, got nil")
		}
	})
	t.Run("Post", func(t *testing.T) {
		if _, err := b.performArtifactoryPost(config, "/artifactory/api/system/ping", url.Values{}); err == nil {
			t.Fatal("expected an error with a nil httpClient, got nil")
		}
	})
	t.Run("PostWithJSON", func(t *testing.T) {
		if _, err := b.performArtifactoryPostWithJSON(config, "/artifactory/api/system/usage", []byte("{}")); err == nil {
			t.Fatal("expected an error with a nil httpClient, got nil")
		}
	})
	t.Run("Delete", func(t *testing.T) {
		if _, err := b.performArtifactoryDelete(config, "/access/api/v1/tokens/dummy"); err == nil {
			t.Fatal("expected an error with a nil httpClient, got nil")
		}
	})
}

// TestConfigReadWithNilHTTPClient_DoesNotCrash reproduces the original crash
// end to end. Reading config/admin fires `go b.sendUsage(...)` (see
// path_config.go); before the fix that goroutine dereferenced the nil client
// and, being unrecovered, crashed the whole plugin process (the go-plugin gRPC
// server), leaving a dead unix socket and "no such file" transport errors.
//
// Before the fix this test aborts the test binary with a SIGSEGV; after the fix
// the nil-guarded helpers return an error and the process survives.
func TestConfigReadWithNilHTTPClient_DoesNotCrash(t *testing.T) {
	ctx := context.Background()
	b, config := makeBackend(t)
	b.InitializeHttpClient(&adminConfiguration{}) // client set, as on an active node

	// Persist an admin config so the read path has something to read, without an
	// HTTP-triggering config update.
	entry, err := logical.StorageEntryJSON(configAdminPath, adminConfiguration{
		baseConfiguration: baseConfiguration{
			AccessToken:    "dummy-non-empty-token",
			ArtifactoryURL: "http://myserver.com",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.StorageView.Put(ctx, entry); err != nil {
		t.Fatal(err)
	}

	// Standby/replica invalidation nils the client with no re-initialisation.
	b.invalidate(ctx, "config")

	// Reading config/admin fires `go b.sendUsage(...)` and calls getVersion();
	// before the fix the goroutine dereferenced the nil client and crashed the
	// whole plugin process.
	_, _ = b.HandleRequest(ctx, &logical.Request{
		Operation: logical.ReadOperation,
		Path:      configAdminPath,
		Storage:   config.StorageView,
	})

	// Let the detached sendUsage goroutine run; with the fix it returns an error
	// instead of panicking.
	time.Sleep(200 * time.Millisecond)
}

// TestSendUsage_NilHTTPClient_DoesNotPanic asserts the non-critical call-home
// telemetry path is safe when the client is nil. sendUsage is always launched as
// a detached goroutine, so a panic here would crash the whole plugin process.
func TestSendUsage_NilHTTPClient_DoesNotPanic(t *testing.T) {
	b, _ := makeBackend(t)
	b.reset() // httpClient = nil, as on a standby/replica node after invalidate

	// Runs synchronously here; must return without panicking.
	b.sendUsage(baseConfiguration{
		AccessToken:    "dummy-non-empty-token",
		ArtifactoryURL: "http://myserver.com",
	}, "unit-test")
}
