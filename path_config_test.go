package artifactory

import (
	"context"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestBackend_AccessTokenRequired(t *testing.T) {
	b, config := makeBackend(t)

	adminConfig := map[string]interface{}{
		"url": "https://127.0.0.1",
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/admin",
		Storage:   config.StorageView,
		Data:      adminConfig,
	})
	assert.NoError(t, err)

	assert.NotNil(t, resp)
	assert.True(t, resp.IsError())
	assert.Contains(t, resp.Error().Error(), "access_token")
}

func TestBackend_URLRequired(t *testing.T) {
	b, config := makeBackend(t)

	adminConfig := map[string]interface{}{
		"access_token": "test-access-token",
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/admin",
		Storage:   config.StorageView,
		Data:      adminConfig,
	})
	assert.NoError(t, err)

	assert.NotNil(t, resp)
	assert.True(t, resp.IsError())
	assert.Contains(t, resp.Error().Error(), "url")
}

// When requesting the config, the access_token must be returned sha256 encoded.
// echo -n "test-access-token"  | shasum -a 256
// 597480d4b62ca612193f19e73fe4cc3ad17f0bf9cfc16a7cbf4b5064131c4805  -
func TestBackend_AccessTokenAsSHA256(t *testing.T) {

	const correctSHA256 = "597480d4b62ca612193f19e73fe4cc3ad17f0bf9cfc16a7cbf4b5064131c4805"
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		"http://myserver.com:80/artifactory/api/system/version",
		httpmock.NewStringResponder(200, artVersion))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "config/admin",
		Storage:   config.StorageView,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.EqualValues(t, correctSHA256, resp.Data["access_token_sha256"])
}
