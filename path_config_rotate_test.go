package artifactory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAcceptanceBackend_PathRotate(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	e := NewConfiguredAcceptanceTestEnv(t)
	before := e.ReadConfigAdmin(t)
	e.UpdateConfigRotate(t, testData{}) // empty write
	after := e.ReadConfigAdmin(t)

	assert.NotEmpty(t, after["access_token_sha256"])
	assert.NotEqual(t, before["access_token_sha256sum"], after["access_token_sha256"])
	e.Cleanup(t)
}

func TestAcceptanceBackend_PathRotateWithDetails(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	newUsername := "vault-acceptance-test-changed"
	description := "Artifactory Secrets Engine Accceptance Test"

	e := NewConfiguredAcceptanceTestEnv(t)
	before := e.ReadConfigAdmin(t)
	e.UpdateConfigRotate(t, testData{
		"username":    newUsername,
		"description": description,
	})
	after := e.ReadConfigAdmin(t)

	assert.NotEqual(t, before["access_token_sha256sum"], after["access_token_sha256"])
	assert.Equal(t, newUsername, after["username"])
	// Not testing Description, because it is not returned in the token (yet)
	e.Cleanup(t)
}
