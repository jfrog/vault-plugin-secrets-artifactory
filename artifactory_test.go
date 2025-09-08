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

// Test that the HTTP request sent to Artifactory matches what the docs say, and that
// handling the response translates into a proper response.
func TestBackend_CreateTokenSuccess(t *testing.T) {
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

	mockArtifactoryUsageVersionRequests("")

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(400, ""))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80/artifactory",
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

	mockArtifactoryUsageVersionRequests("")

	errResp := errorResponse{
		Code:    "Boom",
		Message: "foo",
		Detail:  "bar",
	}
	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewJsonResponderOrPanic(401, &errResp))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80/artifactory",
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

	mockArtifactoryUsageVersionRequests("")

	errResp := errorResponse{
		Code:    "Boom",
		Message: "foo",
		Detail:  "bar",
	}
	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewJsonResponderOrPanic(401, &errResp))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
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

	mockArtifactoryUsageVersionRequests("")

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token",
		httpmock.NewStringResponder(200, canonicalAccessToken))

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/security/token/revoke",
		httpmock.NewStringResponder(200, ""))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
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

// Test that the HTTP request sent to Artifactory matches what the docs say, and that
// handling the response translates into a proper response.
func TestBackend_RotateAdminToken(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests(`{"version" : "7.33.8", "revision" : "73308900"}`)

	httpmock.RegisterResponder(
		http.MethodGet,
		"http://myserver.com:80/access/api/v1/cert/root",
		httpmock.NewStringResponder(200, rootCert))

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/access/api/v1/tokens",
		httpmock.NewStringResponder(200, jwtAccessToken))

	httpmock.RegisterResponder(
		http.MethodDelete,
		"http://myserver.com:80/access/api/v1/tokens/1079485d-5a29-41cd-968e-e42fe924a521",
		httpmock.NewStringResponder(200, ""))

	// Valid jwt Access Token
	// TokenID: 1079485d-5a29-41cd-968e-e42fe924a521
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "eyJ2ZXIiOiIyIiwidHlwIjoiSldUIiwiYWxnIjoiUlMyNTYiLCJraWQiOiJRbVVvVnRxVXhfMVRIS1hCVllQWlZnaUI5bj" +
			"B2b3JkVWl4bkZ0MWVJcFFVIn0.eyJpc3MiOiJqZnN1cHBvcnRAMDFrNGt4MDd6M3FhNWZlaGRyODZuMmNrdzkiLCJzdWIiOiJqZmFjQDAx" +
			"aDQyNGh2d3B5dHprMWF6eGg2azgwN2U1L3VzZXJzL2FkbWluIiwic2NwIjoiYXBwbGllZC1wZXJtaXNzaW9ucy9hZG1pbiIsImF1ZCI6Ii" +
			"pAKiIsImlhdCI6MTc1NzMxMTk5OSwianRpIjoiMTA3OTQ4NWQtNWEyOS00MWNkLTk2OGUtZTQyZmU5MjRhNTIxIn0.QxdKshgKzvjZo-ZE" +
			"2Qekj6brp8us5_uUpKxniXwssnXpE8N5VLOsyk3zEXVGdsI7jDne4W8a0pj0f_0AgstWSRQoNGsG3njh-kKj3G89Aq4OkPKG6gaMOUJnlO" +
			"dJiuMpDu6kCAvtY_rvJ6nUHn9RgEhO1OeiGknrJ5L9iJiY_X7Gplyr8ivFzxsaWhIRTlvGdALWTca1l-Eczp3AuSxW65Q6uA357pLUzA2a" +
			"DL01R9CQ9dLZ_TxCVE7QP3mqbayc_uIj288OeR7RdEGvHsNLoF-HA8JlQhE-ZE3o9xN2Q4wxJc7GGSybXYdbP2jPvnP2nVwMGvXKZMbSyM" +
			"2gBVIODw",
		"url": "http://myserver.com:80/artifactory",
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/rotate",
		Storage:   config.StorageView,
	})
	assert.NoError(t, err)
	assert.Nil(t, resp)

	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      configAdminPath,
		Storage:   config.StorageView,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.EqualValues(t, true, resp.Data["revoke_on_delete"])
}
