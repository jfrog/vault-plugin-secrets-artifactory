package artifactory

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDockerConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		output, err := newDockerConfig("user", "pass", "https://registry.example.com")
		require.NoError(t, err)

		var cfg dockerConfig
		err = json.Unmarshal(output, &cfg)
		require.NoError(t, err)

		assert.Contains(t, cfg.Auths, "registry.example.com")

		expectedAuth := base64.StdEncoding.EncodeToString([]byte("user:pass"))
		assert.Equal(t, expectedAuth, cfg.Auths["registry.example.com"].Auth)
	})

	t.Run("url with port", func(t *testing.T) {
		output, err := newDockerConfig("user", "pass", "https://registry.example.com:8080")
		require.NoError(t, err)

		var cfg dockerConfig
		err = json.Unmarshal(output, &cfg)
		require.NoError(t, err)

		assert.Contains(t, cfg.Auths, "registry.example.com:8080")
	})

	t.Run("invalid url", func(t *testing.T) {
		_, err := newDockerConfig("user", "pass", "://invalid")
		assert.Error(t, err)
	})

	t.Run("empty credentials", func(t *testing.T) {
		output, err := newDockerConfig("", "", "https://example.com")
		require.NoError(t, err)

		var cfg dockerConfig
		err = json.Unmarshal(output, &cfg)
		require.NoError(t, err)

		expectedAuth := base64.StdEncoding.EncodeToString([]byte(":"))
		assert.Equal(t, expectedAuth, cfg.Auths["example.com"].Auth)
	})

	t.Run("json structure", func(t *testing.T) {
		output, err := newDockerConfig("user", "pass", "https://example.com")
		require.NoError(t, err)

		expected := `{"auths":{"example.com":{"auth":"` +
			base64.StdEncoding.EncodeToString([]byte("user:pass")) + `"}}}`
		assert.JSONEq(t, expected, string(output))
	})
}
