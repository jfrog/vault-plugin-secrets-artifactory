package artifactory

import (
	"bytes"
	"context"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

// I've centralized all the tests involving the interplay of TTLs.

// Role with no Max TTL must use the system max TTL when creating tokens.
func TestBackend_NoRoleMaxTTLUsesSystemMaxTTL(t *testing.T) {
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "https://127.0.0.1",
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

	// Fake the Artifactory response
	b.httpClient = newTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body: ioutil.NopCloser(bytes.NewBufferString(`
{
   "access_token":   "adsdgbtybbeeyh...",
   "expires_in":    0,
   "scope":         "api:* member-of-groups:readers",
   "token_type":    "Bearer",
   "refresh_token": "fgsfgsdugh8dgu9s8gy9hsg..."
}
`)),
			Header: make(http.Header),
		}, nil
	})

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
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "https://127.0.0.1",
		"max_ttl":      10 * time.Minute,
	})

	// Role with no maximum TTL
	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "test-username",
		"scope":       "test-scope",
		"refreshable": true,
		"max_ttl":     9 * time.Minute,
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.Nil(t, resp)
	assert.NoError(t, err)

	b.httpClient = newTestClient(tokenCreatedResponse(canonicalAccessToken))

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
