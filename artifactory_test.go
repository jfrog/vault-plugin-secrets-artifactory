package artifactory

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
)

// Test that the HTTP request sent to Artifactory matches what the docs say, and that
// handling the response translates into a proper response.
func TestBackend_CreateTokenSuccess(t *testing.T) {
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "https://127.0.0.1",
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

	// Fake http Client, verifies the request and returns a textbook response
	b.httpClient = newTestClient(func(req *http.Request) (*http.Response, error) {
		assert.EqualValues(t, http.MethodPost, req.Method)
		assert.EqualValues(t, "https://127.0.0.1/api/security/token", req.URL.String())
		assert.EqualValues(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))

		bodyBytes, _ := ioutil.ReadAll(req.Body)
		body := string(bodyBytes)

		assert.Contains(t, body, "username=test-username")
		assert.Contains(t, body, "scope=test-scope")

		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString(canonicalAccessToken)),
			Header:     make(http.Header),
		}, nil
	})

	// Send Request
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
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "https://127.0.0.1",
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

	// Fake http client that just returns an error.
	b.httpClient = newTestClient(func(req *http.Request) (*http.Response, error) {
		return nil, http.ErrHandlerTimeout
	})

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
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "https://127.0.0.1",
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

	// Fake http client that just returns an error.
	b.httpClient = newTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}, nil
	})

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
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "https://127.0.0.1",
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

	// Fake http client that returns some HTML..
	b.httpClient = newTestClient(tokenCreatedResponse(`<html><body><h1>Bad Gateway</h1><hr/></body></html>`))

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
