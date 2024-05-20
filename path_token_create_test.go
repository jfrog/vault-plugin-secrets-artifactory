package artifactory

import (
	"testing"
)

func TestAcceptanceBackend_PathTokenCreate(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	accTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("configure backend", accTestEnv.UpdatePathConfig)
	t.Run("create role", accTestEnv.CreatePathRole)
	t.Run("create admin role", accTestEnv.CreatePathAdminRole)
	t.Run("create token for role", accTestEnv.CreatePathToken)
	t.Run("create scoped down token for admin role", accTestEnv.CreatePathScopedDownToken)
	t.Run("create scoped down token for admin role with bad scope", accTestEnv.CreatePathScopedDownTokenBadScope)
	t.Run("delete role", accTestEnv.DeletePathRole)
	t.Run("cleanup backend", accTestEnv.DeletePathConfig)
}

func TestAcceptanceBackend_PathTokenCreate_overrides(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	accTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("configure backend", accTestEnv.UpdatePathConfig)
	t.Run("create role", accTestEnv.CreatePathRole)
	t.Run("create token for role", accTestEnv.CreatePathToken_overrides)
	t.Run("delete role", accTestEnv.DeletePathRole)
	t.Run("cleanup backend", accTestEnv.DeletePathConfig)
}

func TestAcceptanceBackend_PathTokenCreate_no_access_token(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	accTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	accTestEnv.AccessToken = ""

	t.Run("configure backend with no access token", accTestEnv.UpdatePathConfig)
	t.Run("create role", accTestEnv.CreatePathRole)
	t.Run("create token for role", accTestEnv.CreatePathToken_no_access_token)
	t.Run("delete role", accTestEnv.DeletePathRole)
	t.Run("cleanup backend", accTestEnv.DeletePathConfig)
}
