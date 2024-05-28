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

func TestAcceptanceBackend_PathUserTokenCreate_no_access_token(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	accTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	accTestEnv.AccessToken = ""

	t.Run("configure backend with no access token", accTestEnv.UpdatePathConfig)
	t.Run("create token for admin user", accTestEnv.CreatePathUserToken_no_access_token)
	t.Run("cleanup backend", accTestEnv.DeletePathConfig)
}
