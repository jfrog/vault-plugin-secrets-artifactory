package artifactory

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) revokeToken(config adminConfiguration, secret logical.Secret) error {
	accessToken := secret.InternalData["access_token"].(string)

	values := url.Values{}
	values.Set("token", accessToken)

	resp, err := b.performArtifactoryRequest(config, "/api/security/token/revoke", values)
	if err != nil {
		b.Backend.Logger().Warn("error making request", "response", resp, "err", err)
		return err
	}
	//noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got nonn-200 status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("could not revoke token: HTTP response %v", resp.StatusCode)
	}

	return nil
}

func (b *backend) createToken(config adminConfiguration, role artifactoryRole, accessTokenTTL time.Duration) (*createTokenResponse, error) {
	values := url.Values{}

	if role.GrantType != "" {
		values.Set("grant_type", role.GrantType)
	}

	values.Set("username", role.Username)
	values.Set("scope", role.Scope)

	// A refreshable access token gets replaced by a new access token, which is not
	// what a consumer of tokens from this backend would be expecting; instead they'd
	// likely just request a new token periodically.
	values.Set("refreshable", "false")

	if accessTokenTTL > 0 {
		values.Set("expires_in", fmt.Sprintf("%d", int64(accessTokenTTL.Seconds())))
	} else {
		values.Set("expires_in", "0") // never expires
	}

	if role.Audience != "" {
		values.Set("audience", role.Audience)
	}

	resp, err := b.performArtifactoryRequest(config, "/api/security/token", values)
	if err != nil {
		b.Backend.Logger().Warn("error making request", "response", resp, "err", err)
		return nil, err
	}
	//noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got non-200 status code", "statusCode", resp.StatusCode)
		return nil, fmt.Errorf("could not create access token: HTTP response %v", resp.StatusCode)
	}

	var createdToken createTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&createdToken); err != nil {
		b.Backend.Logger().Warn("could not parse response", "response", resp, "err", err)
		return nil, err
	}

	return &createdToken, nil
}

func (b *backend) performArtifactoryRequest(config adminConfiguration, path string, values url.Values) (*http.Response, error) {
	u, err := url.ParseRequestURI(config.ArtifactoryURL + path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return b.httpClient.Do(req)
}
