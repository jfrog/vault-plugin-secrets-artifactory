package artifactory

import (
	"context"
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
		"POST",
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
		"url":          "http://myserver.com:80/artifactory",
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
		"POST",
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80/artifactory",
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

	httpmock.RegisterResponder(
		"POST",
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
		"url":          "http://myserver.com:80/artifactory",
		// No user_tokens_max_ttl
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

// With a user token max_ttl not greater than the system max ttl, the max ttl for a token must
// be the user token max_ttl.
func TestBackend_UserTokensWorkingWithBothMaxTTLs(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")

	httpmock.RegisterResponder(
		"POST",
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token":        "test-access-token",
		"url":                 "http://myserver.com:80/artifactory",
		"user_tokens_max_ttl": 9 * time.Minute,
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, 9*time.Minute, resp.Secret.MaxTTL)
}

// With a user token max_ttl greater than the system max ttl, the max ttl for a token must
// be the system max_ttl.
func TestBackend_UserTokensSystemMaxTTLLimit(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests("")

	httpmock.RegisterResponder(
		"POST",
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token":        "test-access-token",
		"url":                 "http://myserver.com:80/artifactory",
		"user_tokens_max_ttl": 24 * 365 * 100 * time.Hour,
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "user_token/admin",
		Storage:   config.StorageView,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsError())

	assert.EqualValues(t, config.System.MaxLeaseTTL(), resp.Secret.MaxTTL)
}
