package artifactory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAcceptanceBackend_PathConfigUserToken(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}
	accTestEnv := NewConfiguredAcceptanceTestEnv(t)

	t.Run("update access_token", accTestEnv.PathConfigAccessTokenUpdate)
	t.Run("update default_description", accTestEnv.PathConfigDefaultDescriptionUpdate)
	t.Run("update audience", accTestEnv.PathConfigAudienceUpdate)
	t.Run("update refreshable", accTestEnv.PathConfigRefreshableUpdate)
	t.Run("update include_reference_token", accTestEnv.PathConfigIncludeReferenceTokenUpdate)
	t.Run("update use_expiring_tokens", accTestEnv.PathConfigUseExpiringTokensUpdate)
	t.Run("update default_ttl", accTestEnv.PathConfigDefaultTTLUpdate)
	t.Run("update max_ttl", accTestEnv.PathConfigMaxTTLUpdate)
}

func (e *accTestEnv) PathConfigAccessTokenUpdate(t *testing.T) {
	e.UpdateConfigUserToken(t, "", testData{
		"access_token": "test123",
	})
	data := e.ReadConfigUserToken(t, "")
	accessTokenHash := data["access_token_sha256"]
	assert.NotEmpty(t, "access_token_sha256")

	e.UpdateConfigUserToken(t, "", testData{
		"access_token": "test456",
	})
	data = e.ReadConfigUserToken(t, "")
	assert.NotEqual(t, data["access_token_sha256"], accessTokenHash)
}

func (e *accTestEnv) PathConfigDefaultDescriptionUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateStringField(t, "default_description")
}

func (e *accTestEnv) PathConfigAudienceUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateStringField(t, "audience")
}

func (e *accTestEnv) PathConfigRefreshableUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateBoolField(t, "refreshable")
}

func (e *accTestEnv) PathConfigIncludeReferenceTokenUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateBoolField(t, "include_reference_token")
}

func (e *accTestEnv) PathConfigUseExpiringTokensUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateBoolField(t, "use_expiring_tokens")
}

func (e *accTestEnv) PathConfigDefaultTTLUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateDurationField(t, "default_ttl")
}

func (e *accTestEnv) PathConfigMaxTTLUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateDurationField(t, "max_ttl")
}

func (e *accTestEnv) pathConfigUserTokenUpdateStringField(t *testing.T, fieldName string) {
	e.UpdateConfigUserToken(t, "", testData{
		fieldName: "test123",
	})
	data := e.ReadConfigUserToken(t, "")
	assert.Equal(t, "test123", data[fieldName])

	e.UpdateConfigUserToken(t, "", testData{
		fieldName: "test456",
	})
	data = e.ReadConfigUserToken(t, "")
	assert.Equal(t, "test456", data[fieldName])
}

func (e *accTestEnv) pathConfigUserTokenUpdateBoolField(t *testing.T, fieldName string) {
	e.UpdateConfigUserToken(t, "", testData{
		fieldName: true,
	})
	data := e.ReadConfigUserToken(t, "")
	assert.Equal(t, true, data[fieldName])

	e.UpdateConfigUserToken(t, "", testData{
		fieldName: false,
	})
	data = e.ReadConfigUserToken(t, "")
	assert.Equal(t, false, data[fieldName])
}

func (e *accTestEnv) pathConfigUserTokenUpdateDurationField(t *testing.T, fieldName string) {
	e.UpdateConfigUserToken(t, "", testData{
		fieldName: 1.0,
	})
	data := e.ReadConfigUserToken(t, "")
	assert.Equal(t, 1.0, data[fieldName])

	e.UpdateConfigUserToken(t, "", testData{
		fieldName: 4.0,
	})
	data = e.ReadConfigUserToken(t, "")
	assert.Equal(t, 4.0, data[fieldName])
}

func TestAcceptanceBackend_PathConfigUserToken_UseUserDefault(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}
	accTestEnv := NewConfiguredAcceptanceTestEnv(t)

	accTestEnv.UpdateConfigUserToken(t, "", testData{
		"default_ttl": time.Duration(2) * time.Hour,
		"max_ttl":     time.Duration(4) * time.Hour,
	})

	data := accTestEnv.ReadConfigUserToken(t, "")
	assert.Equal(t, (time.Duration(2) * time.Hour).Seconds(), data["default_ttl"])
	assert.Equal(t, (time.Duration(4) * time.Hour).Seconds(), data["max_ttl"])

	data = accTestEnv.ReadConfigUserToken(t, "test-user")
	assert.Equal(t, (time.Duration(2) * time.Hour).Seconds(), data["default_ttl"])
	assert.Equal(t, (time.Duration(4) * time.Hour).Seconds(), data["max_ttl"])
}

func TestAcceptanceBackend_PathConfigUserToken_UseSystemDefault(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}
	accTestEnv := NewConfiguredAcceptanceTestEnv(t)

	data := accTestEnv.ReadConfigUserToken(t, "")
	assert.Equal(t, accTestEnv.Backend.System().DefaultLeaseTTL().Seconds(), data["default_ttl"])
	assert.Equal(t, accTestEnv.Backend.System().MaxLeaseTTL().Seconds(), data["max_ttl"])

	data = accTestEnv.ReadConfigUserToken(t, "test-user")
	assert.Equal(t, accTestEnv.Backend.System().DefaultLeaseTTL().Seconds(), data["default_ttl"])
	assert.Equal(t, accTestEnv.Backend.System().MaxLeaseTTL().Seconds(), data["max_ttl"])
}
