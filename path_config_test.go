package artifactory

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestAcceptanceBackend_PathConfig(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	accTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("notConfigured", accTestEnv.PathConfigReadUnconfigured)
	t.Run("update", accTestEnv.UpdatePathConfig)
	t.Run("read", accTestEnv.ReadPathConfig)
	t.Run("expiringTokens", accTestEnv.PathConfigUpdateExpiringTokens)
	t.Run("bypassArtifactoryTLSVerification", accTestEnv.PathConfigUpdateBypassArtifactoryTLSVerification)
	t.Run("allowScopedTokens", accTestEnv.PathConfigUpdateAllowScopeOverride)
	t.Run("usernameTemplate", accTestEnv.PathConfigUpdateUsernameTemplate)
	t.Run("delete", accTestEnv.DeletePathConfig)
	t.Run("errors", accTestEnv.PathConfigUpdateErrors)
	t.Run("badAccessToken", accTestEnv.PathConfigReadBadAccessToken)
}

func (e *accTestEnv) PathConfigReadUnconfigured(t *testing.T) {
	resp, err := e.read(configAdminPath)
	assert.Contains(t, resp.Data["error"], "backend not configured")
	assert.NoError(t, err)
}

func (e *accTestEnv) PathConfigUpdateExpiringTokens(t *testing.T) {
	e.pathConfigUpdateBooleanField(t, "use_expiring_tokens")
}

func (e *accTestEnv) PathConfigForceRevocableTokens(t *testing.T) {
	e.pathConfigUpdateBooleanPtrField(t, "force_revocable")
}

func (e *accTestEnv) PathConfigUpdateBypassArtifactoryTLSVerification(t *testing.T) {
	e.pathConfigUpdateBooleanField(t, "bypass_artifactory_tls_verification")
}

func (e *accTestEnv) PathConfigUpdateAllowScopeOverride(t *testing.T) {
	e.pathConfigUpdateBooleanField(t, "allow_scope_override")
}

func (e *accTestEnv) pathConfigUpdateBooleanField(t *testing.T, fieldName string) {
	// Boolean
	e.UpdateConfigAdmin(t, testData{
		fieldName: true,
	})
	data := e.ReadConfigAdmin(t)
	assert.Equal(t, true, data[fieldName])

	e.UpdateConfigAdmin(t, testData{
		fieldName: false,
	})
	data = e.ReadConfigAdmin(t)
	assert.Equal(t, false, data[fieldName])

	// String
	e.UpdateConfigAdmin(t, testData{
		fieldName: "true",
	})
	data = e.ReadConfigAdmin(t)
	assert.Equal(t, true, data[fieldName])

	e.UpdateConfigAdmin(t, testData{
		fieldName: "false",
	})
	data = e.ReadConfigAdmin(t)
	assert.Equal(t, false, data[fieldName])

	// Fail Tests
	resp, err := e.update(configAdminPath, testData{
		fieldName: "Sure, why not",
	})
	assert.NotNil(t, resp)
	assert.Regexp(t, regexp.MustCompile("Field validation failed: error converting input .* strconv.ParseBool: parsing .*: invalid syntax"), resp.Data["error"])
	assert.Nil(t, err)
}

func (e *accTestEnv) pathConfigUpdateBooleanPtrField(t *testing.T, fieldName string) {
	// Boolean
	e.UpdateConfigAdmin(t, testData{
		fieldName: true,
	})
	data := e.ReadConfigAdmin(t)
	assert.Equal(t, true, *data[fieldName].(*bool))

	e.UpdateConfigAdmin(t, testData{
		fieldName: false,
	})
	data = e.ReadConfigAdmin(t)
	assert.Equal(t, false, *data[fieldName].(*bool))

	// String
	e.UpdateConfigAdmin(t, testData{
		fieldName: "true",
	})
	data = e.ReadConfigAdmin(t)
	assert.Equal(t, true, *data[fieldName].(*bool))

	e.UpdateConfigAdmin(t, testData{
		fieldName: "false",
	})
	data = e.ReadConfigAdmin(t)
	assert.Equal(t, false, *data[fieldName].(*bool))

	// Fail Tests
	resp, err := e.update(configAdminPath, testData{
		fieldName: "Sure, why not",
	})
	assert.NotNil(t, resp)
	assert.Regexp(t, regexp.MustCompile("Field validation failed: error converting input .* strconv.ParseBool: parsing .*: invalid syntax"), resp.Data["error"])
	assert.Nil(t, err)
}

func (e *accTestEnv) PathConfigUpdateUsernameTemplate(t *testing.T) {
	usernameTemplate := "v_{{.DisplayName}}_{{.RoleName}}_{{random 10}}_{{unix_time}}"
	e.UpdateConfigAdmin(t, testData{
		"username_template": usernameTemplate,
	})
	data := e.ReadConfigAdmin(t)
	assert.Equal(t, data["username_template"], usernameTemplate)

	// Bad Template
	resp, err := e.update(configAdminPath, testData{
		"username_template": "bad_{{ .somethingInvalid }}_testing {{",
	})
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Data["error"], "username_template error")
	assert.ErrorContains(t, err, "username_template")
}

// most of these were covered by unit tests, but we want test coverage for acceptance
func (e *accTestEnv) PathConfigUpdateErrors(t *testing.T) {
	// URL Required
	resp, err := e.update(configAdminPath, testData{
		"access_token": "test-access-token",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsError())
	assert.Contains(t, resp.Error().Error(), "url")
}

func (e *accTestEnv) PathConfigReadBadAccessToken(t *testing.T) {
	// Forcibly set a bad token
	entry, err := logical.StorageEntryJSON(configAdminPath, adminConfiguration{
		baseConfiguration: baseConfiguration{
			AccessToken:    "bogus.token",
			ArtifactoryURL: e.URL,
		},
	})
	assert.NoError(t, err)
	err = e.Storage.Put(e.Context, entry)
	assert.NoError(t, err)
	resp, err := e.read(configAdminPath)

	assert.Error(t, err)
	assert.Nil(t, resp)
	// Otherwise success, we don't need to re-test for this
}

func TestBackend_URLRequired(t *testing.T) {
	b, config := makeBackend(t)

	adminConfig := map[string]interface{}{
		"access_token": "test-access-token",
	}

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      configAdminPath,
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

	mockArtifactoryUsageVersionRequests("")

	httpmock.RegisterResponder(
		http.MethodGet,
		"http://myserver.com:80/access/api/v1/cert/root",
		httpmock.NewStringResponder(200, rootCert))

	b, config := configuredBackend(t, map[string]interface{}{
		"access_token": "test-access-token",
		"url":          "http://myserver.com:80",
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      configAdminPath,
		Storage:   config.StorageView,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.EqualValues(t, correctSHA256, resp.Data["access_token_sha256"])
}

func TestBackend_RevokeOnDelete(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockArtifactoryUsageVersionRequests(`{"version" : "7.33.8", "revision" : "73308900"}`)

	httpmock.RegisterResponder(
		http.MethodGet,
		"http://myserver.com:80/access/api/v1/cert/root",
		httpmock.NewStringResponder(200, rootCert))

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
		"url":              "http://myserver.com:80",
		"revoke_on_delete": true,
	})

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      configAdminPath,
		Storage:   config.StorageView,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.EqualValues(t, true, resp.Data["revoke_on_delete"])

	httpmock.RegisterResponder(
		http.MethodDelete,
		"http://myserver.com:80/access/api/v1/tokens/84c0626b-7973-40c9-9d37-701aaf73cfb4",
		httpmock.NewStringResponder(200, ""))

	resp, err = b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      configAdminPath,
		Storage:   config.StorageView,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}
