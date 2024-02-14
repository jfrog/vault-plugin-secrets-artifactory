package artifactory

import (
	"testing"
)

func TestAcceptanceBackend_PathUserTokenCreate(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	accTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("configure backend", accTestEnv.UpdatePathConfig)
	t.Run("create token for admin user", accTestEnv.CreatePathUserToken)
	t.Run("cleanup backend", accTestEnv.DeletePathConfig)
}

func TestAcceptanceBackend_PathUserTokenCreate_overrides(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	accTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("configure backend", accTestEnv.UpdatePathConfig)
	t.Run("create token for admin user", accTestEnv.CreatePathUserToken_overrides)
	t.Run("cleanup backend", accTestEnv.DeletePathConfig)
}
