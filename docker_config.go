package artifactory

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
)

type dockerConfig struct {
	Auths map[string]authEntry `json:"auths"`
}

type authEntry struct {
	Auth string `json:"auth"`
}

func newDockerConfig(username, password, server string) ([]byte, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	cfg := &dockerConfig{
		Auths: map[string]authEntry{
			u.Host: {
				Auth: auth,
			},
		},
	}

	return json.Marshal(cfg)
}
