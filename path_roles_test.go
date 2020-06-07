package artifactory

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
)

// When there are no roles, an error must be returned.
func TestBackend_PathRoleList_Empty(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, _ := Backend(config)
	if err := b.Setup(context.Background(), config); err != nil {
		t.Fatal(err)
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ListOperation,
		Path:      "roles",
		Storage:   config.StorageView,
	})

	assert.NotNil(t, resp)
	assert.NoError(t, err)
	assert.Empty(t, resp.Warnings)
	assert.True(t, resp.IsError())
}

// The backend must be configured before it will accept roles
func TestBackend_PathRoleList_CannotAddRoleWhenNotConfigured(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, _ := Backend(config)
	if err := b.Setup(context.Background(), config); err != nil {
		t.Fatal(err)
	}

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

	assert.NotNil(t, resp)
	assert.NoError(t, err)
	assert.Empty(t, resp.Warnings)
	assert.True(t, resp.IsError())
}

// A configured backend must accept a new role.
func TestBackend_PathRoleList_AddRole(t *testing.T) {
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "test-url",
	})

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
}

// Listing roles must return the name of the role.
func TestBackend_PathRoleListReturnsRole(t *testing.T) {
	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "test-url",
	})

	roleData := map[string]interface{}{
		"role":     "test-role",
		"username": "test-username",
		"scope":    "test-scope",
	}

	_, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.NoError(t, err)

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ListOperation,
		Path:      "roles",
		Storage:   config.StorageView,
	})
	assert.NotNil(t, resp)
	assert.NoError(t, err)
	assert.False(t, resp.IsError())

	assert.Len(t, resp.Data, 1)
	assert.Len(t, resp.Data["keys"], 1)
	assert.EqualValues(t, "test-role", resp.Data["keys"].([]string)[0])
}

// Simple test that enforces what goes in is what comes out.
func TestBackend_PathRoleWriteThenRead(t *testing.T) {

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "ignored",
		"url":          "ignored",
	})

	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "test-username",
		"scope":       "test-scope",
		"refreshable": true,
		"audience":    "test-audience",
		"default_ttl": 30 * time.Minute,
		"max_ttl":     45 * time.Minute,
	}

	_, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
		Data:      roleData,
	})
	assert.NoError(t, err)

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "roles/test-role",
		Storage:   config.StorageView,
	})
	assert.NotNil(t, resp)
	assert.NoError(t, err)

	assert.EqualValues(t, "test-username", resp.Data["username"])
	assert.EqualValues(t, "test-scope", resp.Data["scope"])
	assert.True(t, resp.Data["refreshable"].(bool))
	assert.EqualValues(t, "test-audience", resp.Data["audience"])
	assert.EqualValues(t, 30*time.Minute.Seconds(), resp.Data["default_ttl"])
	assert.EqualValues(t, 45*time.Minute.Seconds(), resp.Data["max_ttl"])
}

// TODO test required fields
// TODO test deleting a role removes it
