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

// Test that the HTTP request sent to Artifactory matches what the docs say, and that
// handling the response translates into a proper response.
func TestBackend_CreateTokenSuccess(t *testing.T) {

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://127.0.0.1",
	})

	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "test-username",
		"scope":       "test-scope",
		"refreshable": true,
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

	b.httpClient = newTestClient(func(req *http.Request) (*http.Response, error) {
		assert.EqualValues(t, http.MethodPost, req.Method)
		assert.EqualValues(t, "http://127.0.0.1/api/security/token", req.URL.String())
		assert.EqualValues(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))

		bodyBytes, _ := ioutil.ReadAll(req.Body)
		body := string(bodyBytes)

		assert.Contains(t, body, "username=test-username")
		assert.Contains(t, body, "scope=test-scope")
		assert.Contains(t, body, "expires_in=300")
		assert.Contains(t, body, "refreshable=true")

		return &http.Response{
			StatusCode: 200,
			// Literally https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API#ArtifactoryRESTAPI-CreateToken
			Body: ioutil.NopCloser(bytes.NewBufferString(`
{
   "access_token":   "adsdgbtybbeeyh...",
   "expires_in":    3600,
   "scope":         "api:* member-of-groups:readers",
   "token_type":    "Bearer",
   "refresh_token": "fgsfgsdugh8dgu9s8gy9hsg..."
}
`)),
			Header: make(http.Header),
		}, nil
	})

	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "token/test-role",
		Storage:   config.StorageView,
	})
	assert.NotNil(t, resp)
	assert.NoError(t, err)

	assert.EqualValues(t, 3600, resp.Data["expires_in"])
	assert.EqualValues(t, "adsdgbtybbeeyh...", resp.Data["access_token"])
	assert.EqualValues(t, true, resp.Data["refreshable"])
	assert.EqualValues(t, "test-role", resp.Data["role"])
	assert.EqualValues(t, "api:* member-of-groups:readers", resp.Data["scope"])
}
