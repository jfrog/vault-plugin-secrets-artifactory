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
		"http://myserver.com:80/access/api/v1/tokens/84c0626b-7973-40c9-9d37-701aaf73cfb4",
		httpmock.NewStringResponder(200, ""))

	// Valid jwt Access Token
	// TokenID: 84c0626b-7973-40c9-9d37-701aaf73cfb4
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": `eyJ2ZXIiOiIyIiwidHlwIjoiSldUIiwiYWxnIjoiUlMyNTYiLCJraW` +
			`QiOiJkMUxJUFRHbmY0RHZzQ2k0MzhodU9KdWN3bi1lSTBHc0lVR2g0bGhhdE53In0.eyJ` +
			`zdWIiOiJqZmFjQDAxaDQyNGh2d3B5dHprMWF6eGg2azgwN2U1L3VzZXJzL2FkbWluIiwi` +
			`c2NwIjoiYXBwbGllZC1wZXJtaXNzaW9ucy9hZG1pbiIsImF1ZCI6IipAKiIsImlzcyI6I` +
			`mpmZmVAMDFoNDI0aHZ3cHl0emsxYXp4aDZrODA3ZTUiLCJleHAiOjE3NTMyOTM5OTMsIm` +
			`lhdCI6MTY5MDIyMTk5MywianRpIjoiODRjMDYyNmItNzk3My00MGM5LTlkMzctNzAxYWF` +
			`mNzNjZmI0In0.VXoZR--oQLRTqTLx3Ogz1UUrzT9hlihWQ8m_JgOucZEYwIjGa2P58wUW` +
			`vUAxonkiqyvmFfEk8H1vyiaBQ0F9vQ7v16d3D3nfEDW71g09M3NnsKu065-pbjPRGUmSi` +
			`SvV0WC3Gla5Ui31IA_vVhyc-kPDzoWpHwBWgOMWkJwP0ZrvQ5bwzKrwNQi6YB0SIX2eSH` +
			`RpReef19W_4BpOUrqMrcDamB3mskwxcYFUMA45FRoV_JVxZsIMOyNNfDlNy01r5bA6ZcY` +
			`EaseaQpU7skMCW07rUiWq4Z6U0xZEduKPlowJm9xbrBM13FEQTG4b4mW7yyOD4gqQ49wD` +
			`GGXvhLVFoQ`,
		"url": "http://myserver.com:80/artifactory",
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/rotate",
		Storage:   config.StorageView,
	})
	assert.NoError(t, err)
	assert.Nil(t, resp)
}
