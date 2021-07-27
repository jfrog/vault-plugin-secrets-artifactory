package artifactory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

// Test that the HTTP request sent to Artifactory matches what the docs say, and that
// handling the response translates into a proper response.
func TestBackend_CreateTokenSuccess(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		"https://myserver.com/artifactory/api/system/version",
		httpmock.NewStringResponder(200, artVersion))

	httpmock.RegisterResponder(
		"POST",
		"https://myserver.com/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "https://myserver.com/artifactory",
	})

	// Setup a role
	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "test-username",
		"scope":       "test-scope",
		"default_ttl": 5 * time.Minute,
		"max_ttl":     10 * time.Minute,
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.NoError(t, err)
	assert.Nil(t, resp)

	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   config.StorageView,
	})
	assert.NotNil(t, resp)
	fmt.Printf("resp :%v", resp)
	assert.NoError(t, err)

	// Verify response
	assert.EqualValues(t, 10*time.Minute, resp.Secret.MaxTTL)
	assert.EqualValues(t, 5*time.Minute, resp.Secret.TTL)

	assert.EqualValues(t, "eyXsdgbtybbeeyh...", resp.Data["access_token"])
	assert.EqualValues(t, "test-role", resp.Data["role"])
	assert.EqualValues(t, "api:* member-of-groups:example", resp.Data["scope"])

}

// Test that an error is returned if Artifactory is unavailable.
func TestBackend_CreateTokenArtifactoryUnavailable(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		"http://myserver.com/artifactory/api/system/version",
		httpmock.NewStringResponder(200, artVersion))

	httpmock.RegisterResponder(
		"POST",
		"http://myserver.com/artifactory/api/security/token",
		httpmock.NewStringResponder(400, ""))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com/artifactory",
	})

	// Setup a role
	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "test-username",
		"scope":       "test-scope",
		"default_ttl": 5 * time.Minute,
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.NoError(t, err)
	assert.Nil(t, resp)

	// Send Request
	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   config.StorageView,
	})
	// Make sure we get the error.
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// Test that an error is returned if the access token is invalid for the operation being performed.
func TestBackend_CreateTokenUnauthorized(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		"http://myserver.com/artifactory/api/system/version",
		httpmock.NewStringResponder(200, artVersion))

	httpmock.RegisterResponder(
		"POST",
		"http://myserver.com/artifactory/api/security/token",
		httpmock.NewStringResponder(401, ""))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com/artifactory",
	})

	// Setup a role
	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "test-username",
		"scope":       "test-scope",
		"default_ttl": 5 * time.Minute,
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.NoError(t, err)
	assert.Nil(t, resp)

	// Send Request
	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   config.StorageView,
	})

	// Make sure we get the error.
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// Test that an error is returned when the nginx in front of Artifactory can't reach Artifactory.
// It happens.
func TestBackend_CreateTokenArtifactoryMisconfigured(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		"http://myserver.com/artifactory/api/system/version",
		httpmock.NewStringResponder(200, artVersion))

	httpmock.RegisterResponder(
		"POST",
		"http://myserver.com/artifactory/api/security/token",
		httpmock.NewStringResponder(401, `<html><body><h1>Bad Gateway</h1><hr/></body></html>`))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com/artifactory",
	})

	// Setup a role
	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "test-username",
		"scope":       "test-scope",
		"default_ttl": 5 * time.Minute,
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.NoError(t, err)
	assert.Nil(t, resp)

	// Send Request
	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   config.StorageView,
	})

	// Make sure we get the error.
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestBackend_RevokeToken(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		"http://myserver.com/artifactory/api/system/version",
		httpmock.NewStringResponder(200, artVersion))

	httpmock.RegisterResponder(
		"POST",
		"http://myserver.com/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	httpmock.RegisterResponder(
		"POST",
		"http://myserver.com/artifactory/api/security/token/revoke",
		httpmock.NewStringResponder(200, ""))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com/artifactory",
	})

	// Setup a role
	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "test-username",
		"scope":       "test-scope",
		"default_ttl": 5 * time.Minute,
		"max_ttl":     10 * time.Minute,
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.NoError(t, err)
	assert.Nil(t, resp)

	// Send Request
	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   config.StorageView,
	})
	assert.NotNil(t, resp)
	assert.NoError(t, err)

	secret := resp.Secret

	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.RevokeOperation,
		Secret:    secret,
		Storage:   config.StorageView,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}
