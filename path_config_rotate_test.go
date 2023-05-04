package artifactory

import (
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
)

func TestAcceptanceBackend_PathRotate(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	// Unconfigured Test
	unconfigured, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("unconfigured", unconfigured.PathConfigRotateUnconfigured)

	//Configured Tests
	e := NewConfiguredAcceptanceTestEnv(t)
	t.Run("zeroLengthUsername", e.PathConfigRotateZeroLengthUsername)
	t.Run("empty", e.PathConfigRotateEmpty)
	t.Run("withDetails", e.PathConfigRotateWithDetails)
	// Cleanup Token
	e.Cleanup(t)

	// Failure Tests
	t.Run("CreateTokenErr", e.PathConfigRotateCreateTokenErr)
	t.Run("badAccessToken", e.PathConfigRotateBadAccessToken)
}

func (e *accTestEnv) PathConfigRotateUnconfigured(t *testing.T) {
	resp, err := e.update("config/rotate", testData{})
	assert.Contains(t, resp.Data["error"], "backend not configured")
	assert.NoError(t, err)
}

func (e *accTestEnv) PathConfigRotateEmpty(t *testing.T) {
	before := e.ReadConfigAdmin(t)
	e.UpdateConfigRotate(t, testData{}) // empty write
	after := e.ReadConfigAdmin(t)
	assert.NotEqual(t, before["access_token_sha256sum"], after["access_token_sha256"])
}

func (e *accTestEnv) PathConfigRotateZeroLengthUsername(t *testing.T) {
	e.UpdateConfigRotate(t, testData{
		"username": "",
	}) // empty write
	after := e.ReadConfigAdmin(t)
	assert.Equal(t, "admin-vault-secrets-artifactory", after["username"])
}

func (e *accTestEnv) PathConfigRotateWithDetails(t *testing.T) {
	newUsername := "vault-acceptance-test-changed"
	description := "Artifactory Secrets Engine Accceptance Test"
	before := e.ReadConfigAdmin(t)
	e.UpdateConfigRotate(t, testData{
		"username":    newUsername,
		"description": description,
	})
	after := e.ReadConfigAdmin(t)
	assert.NotEqual(t, before["access_token_sha256sum"], after["access_token_sha256"])
	assert.Equal(t, newUsername, after["username"])
	// Not testing Description, because it is not returned in the token (yet)
}

func (e *accTestEnv) PathConfigRotateCreateTokenErr(t *testing.T) {
	tokenId, accessToken := e.createNewNonAdminTestToken(t)
	e.UpdateConfigAdmin(t, testData{
		"access_token": accessToken,
		"url":          e.URL,
	})
	resp, err := e.update("config/rotate", testData{})
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Data["error"], "error creating new token")
	assert.ErrorContains(t, err, "could not create access token")
	e.revokeTestToken(t, e.AccessToken, tokenId)
}

func (e *accTestEnv) PathConfigRotateBadAccessToken(t *testing.T) {
	// Forcibly set a bad token
	entry, err := logical.StorageEntryJSON("config/admin", adminConfiguration{
		AccessToken:    "bogus.token",
		ArtifactoryURL: e.URL,
	})
	assert.NoError(t, err)
	err = e.Storage.Put(e.Context, entry)
	assert.NoError(t, err)
	resp, err := e.update("config/rotate", testData{})
	// assert.Contains(t, resp.Data["error"], "error parsing existing AccessToken")
	assert.Contains(t, resp.Data["error"], "could not get the certificate")
	assert.Error(t, err)
}
