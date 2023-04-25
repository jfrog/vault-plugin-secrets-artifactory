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
	t.Run("empty", e.PathConfigRotateEmpty)
	t.Run("withDetails", e.PathConfigRotateWithDetails)
	e.Cleanup(t)
}

func (e *accTestEnv) PathConfigRotateEmpty(t *testing.T) {
	before := e.ReadConfigAdmin(t)
	e.UpdateConfigRotate(t, testData{}) // empty write
	after := e.ReadConfigAdmin(t)
	assert.NotEqual(t, before["access_token_sha256sum"], after["access_token_sha256"])
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
