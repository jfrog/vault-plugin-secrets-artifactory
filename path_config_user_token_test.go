package artifactory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAcceptanceBackend_PathConfigUserToken(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}
	accTestEnv := NewConfiguredAcceptanceTestEnv(t)

	t.Run("update default_description", accTestEnv.PathConfigDefaultDescriptionUpdate)
	t.Run("update audience", accTestEnv.PathConfigAudienceUpdate)
	t.Run("update default_ttl", accTestEnv.PathConfigDefaultTTLUpdate)
	t.Run("update max_ttl", accTestEnv.PathConfigMaxTTLUpdate)
}

func (e *accTestEnv) PathConfigDefaultDescriptionUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateStringField(t, "default_description")
}

func (e *accTestEnv) PathConfigAudienceUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateStringField(t, "audience")
}

func (e *accTestEnv) PathConfigDefaultTTLUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateDurationField(t, "default_ttl")
}

func (e *accTestEnv) PathConfigMaxTTLUpdate(t *testing.T) {
	e.pathConfigUserTokenUpdateDurationField(t, "max_ttl")
}

func (e *accTestEnv) pathConfigUserTokenUpdateStringField(t *testing.T, fieldName string) {
	e.UpdateConfigUserToken(t, testData{
		fieldName: "test123",
	})
	data := e.ReadConfigUserToken(t)
	assert.Equal(t, "test123", data[fieldName])

	e.UpdateConfigUserToken(t, testData{
		fieldName: "test456",
	})
	data = e.ReadConfigUserToken(t)
	assert.Equal(t, "test456", data[fieldName])
}

func (e *accTestEnv) pathConfigUserTokenUpdateDurationField(t *testing.T, fieldName string) {
	e.UpdateConfigUserToken(t, testData{
		fieldName: 1.0,
	})
	data := e.ReadConfigUserToken(t)
	assert.Equal(t, 1.0, data[fieldName])

	e.UpdateConfigUserToken(t, testData{
		fieldName: 4.0,
	})
	data = e.ReadConfigUserToken(t)
	assert.Equal(t, 4.0, data[fieldName])
}
