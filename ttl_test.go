package artifactory

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

// I've centralized all the tests involving the interplay of TTLs.

// Role with no Max TTL must use the system max TTL when creating tokens.
func TestBackend_NoRoleMaxTTLUsesSystemMaxTTL(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, `
		{
		   "access_token":   "adsdgbtybbeeyh...",
		   "expires_in":    0,
		   "scope":         "api:* member-of-groups:readers",
		   "token_type":    "Bearer",
		   "refresh_token": "fgsfgsdugh8dgu9s8gy9hsg..."
		}`),
	)

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	// Role with no maximum TTL
	roleData := map[string]interface{}{
		"role":     "test-role",
		"username": "test-username",
		"scope":    "test-scope",
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.Nil(t, resp)
	assert.NoError(t, err)

	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   config.StorageView,
	})
	assert.NotNil(t, resp)
	assert.NoError(t, err)

	assert.EqualValues(t, config.System.MaxLeaseTTL(), resp.Secret.MaxTTL)
}

// With a role max_ttl not greater than the system max_ttl, the max_ttl for a token must
// be the role max_ttl.
func TestBackend_WorkingWithBothMaxTTLs(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
		"max_ttl":      10 * time.Minute,
	})

	// Role with no maximum TTL
	roleData := map[string]interface{}{
		"role":     "test-role",
		"username": "test-username",
		"scope":    "test-scope",
		"max_ttl":  9 * time.Minute,
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.Nil(t, resp)
	assert.NoError(t, err)

	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   config.StorageView,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, 9*time.Minute, resp.Secret.MaxTTL)
}

// User tokens with no Max TTL must use the system max TTL when creating tokens.
func TestBackend_NoUserTokensMaxTTLUsesSystemMaxTTL(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")
	mockArtifactoryRoleRequest()

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, `
		{
		   "access_token":   "adsdgbtybbeeyh...",
		   "expires_in":    0,
		   "scope":         "api:* member-of-groups:readers",
		   "token_type":    "Bearer",
		   "refresh_token": "fgsfgsdugh8dgu9s8gy9hsg..."
		}
		`))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
	})
	assert.NotNil(t, resp)
	assert.NoError(t, err)

	assert.EqualValues(t, config.System.MaxLeaseTTL(), resp.Secret.MaxTTL)
}

func SetUserTokenMaxTTL(t *testing.T, b *backend, storage logical.Storage, path string, max_ttl time.Duration) {
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      path,
		Storage:   storage,
		Data: map[string]interface{}{
			"max_ttl": max_ttl,
		},
	})
	assert.NoError(t, err)
	assert.False(t, resp.IsError())
}

// Use system max_ttl if: user token config max_ttl > system max_ttl
func TestBackend_UserTokenConfigMaxTTLUseSystem(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")
	mockArtifactoryRoleRequest()

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	configPath := configUserTokenPath + "/admin"
	backend_max_ttl := b.System().MaxLeaseTTL()
	user_token_config_ttl := backend_max_ttl + 1*time.Minute
	SetUserTokenMaxTTL(t, b, config.StorageView, configPath, user_token_config_ttl)

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, backend_max_ttl, resp.Secret.MaxTTL)
}

// Use user_token config max_ttl if: user token config max_ttl < system max_ttl
func TestBackend_UserTokenConfigMaxTTLUseConfigMaxTTL(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")
	mockArtifactoryRoleRequest()

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	configPath := configUserTokenPath + "/admin"
	backend_max_ttl := b.System().MaxLeaseTTL()
	user_token_config_ttl := backend_max_ttl - 10*time.Minute
	SetUserTokenMaxTTL(t, b, config.StorageView, configPath, user_token_config_ttl)

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, user_token_config_ttl, resp.Secret.MaxTTL)
}

// Use request max_ttl if: request ttl < user token config max_ttl < system max_ttl
func TestBackend_UserTokenMaxTTLUseRequestTTL(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")
	mockArtifactoryRoleRequest()

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	backend_max_ttl := b.System().MaxLeaseTTL()
	test_max_ttl := backend_max_ttl - 1*time.Minute

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
		Data: map[string]interface{}{
			"max_ttl": test_max_ttl,
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, test_max_ttl, resp.Secret.MaxTTL)
}

func TestBackend_UserTokenMaxTTLEnforced(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")
	mockArtifactoryRoleRequest()

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	backend_max_ttl := b.System().MaxLeaseTTL()
	test_max_ttl := backend_max_ttl - 1*time.Minute

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
		Data: map[string]interface{}{
			"max_ttl": test_max_ttl,
			"ttl":     test_max_ttl + 10*time.Minute,
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, test_max_ttl, resp.Secret.TTL)
}

func TestBackend_UserTokenTTLRequest(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")
	mockArtifactoryRoleRequest()

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
		Data: map[string]interface{}{
			"ttl": 42 * time.Minute,
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, 42*time.Minute, resp.Secret.TTL)
}

func TestBackend_UserTokenDefaultTTL(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")
	mockArtifactoryRoleRequest()

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      configUserTokenPath + "/admin",
		Storage:   config.StorageView,
		Data: map[string]interface{}{
			"default_ttl": 42 * time.Minute,
		},
	})
	assert.NoError(t, err)
	assert.False(t, resp.IsError())

	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, 42*time.Minute, resp.Secret.TTL)
}
