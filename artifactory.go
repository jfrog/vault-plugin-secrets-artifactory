package artifactory

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/vault/sdk/logical"
	"net/http"
	"net/url"
	"strings"
)

func (b *backend) revokeToken(config adminConfiguration, secret logical.Secret) error {
	accessToken := secret.InternalData["access_token"].(string)

	values := url.Values{}
	values.Set("token", accessToken)

	resp, err := b.performArtifactoryRequest(config, "/api/security/token/revoke", values)
	if err != nil {
		b.Backend.Logger().Warn("error making request", "response", resp, "err", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got status code", "statusCode", resp.StatusCode, "response", resp)
		return fmt.Errorf("could not revoke token: HTTP response %v", resp.StatusCode)
	}

	return nil
}

func (b *backend) refreshToken(config adminConfiguration, accessToken, refreshToken string) (*createTokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", refreshToken)
	values.Set("access_token", accessToken)

	resp, err := b.performArtifactoryRequest(config, "/api/security/token", values)
	if err != nil {
		b.Backend.Logger().Warn("error making request", "response", resp, "err", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got status code", "statusCode", resp.StatusCode, "response", resp)
		return nil, fmt.Errorf("could not create access token: HTTP response %v", resp.StatusCode)
	}

	var createdToken createTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&createdToken); err != nil {
		b.Backend.Logger().Warn("could not parse response", "response", resp, "err", err)
		return nil, err
	}

	return &createdToken, nil
}

func (b *backend) createToken(config adminConfiguration, role artifactoryRole) (*createTokenResponse, error) {
	values := url.Values{}
	if role.GrantType != "" {
		values.Set("grant_type", role.GrantType)
	}

	values.Set("username", role.Username)
	values.Set("scope", role.Scope)

	//FIXME expires_in ??

	if role.Refreshable {
		values.Set("refreshable", "true")
	}

	if role.Audience != "" {
		values.Set("audience", role.Audience)
	}

	resp, err := b.performArtifactoryRequest(config, "/api/security/token", values)
	if err != nil {
		b.Backend.Logger().Warn("error making request", "response", resp, "err", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got status code", "statusCode", resp.StatusCode, "response", resp)
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

	return http.DefaultClient.Do(req)
}
