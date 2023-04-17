package artifactory

import (
	"testing"
)

func TestAcceptanceBackend_PathRotate(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	accTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	accTestEnv.RotatePathConfig(t)
}
