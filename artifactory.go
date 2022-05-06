package artifactory

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) revokeToken(config adminConfiguration, secret logical.Secret) error {

	accessToken := secret.InternalData["access_token"].(string)
	tokenId := secret.InternalData["token_id"].(string)

	values := url.Values{}
	values.Set("token", accessToken)

	newAccessReq, err := b.getSystemStatus(config)
	if err != nil {
		b.Backend.Logger().Warn("could not get artifactory version", "err", err)
		return err
	}

	u, err := url.Parse(config.ArtifactoryURL)
	if err != nil {
		b.Backend.Logger().Warn("could not parse artifactory url", "url", u, "err", err)
		return err
	}

	var resp *http.Response

	if newAccessReq == true {
		resp, err = b.performArtifactoryDelRequest(config, u.Host, u.Scheme+"://"+u.Host+"/access/api/v1/tokens/"+tokenId)
		if err != nil {
			b.Backend.Logger().Warn("error deleting access token", "response", resp, "err", err)
			return err
		}

	} else {

		resp, err = b.performArtifactoryRequest(config, u.Scheme+"://"+u.Host+u.Path+"/api/security/token/revoke", u.Host, values)
		if err != nil {
			b.Backend.Logger().Warn("error deleting token", "response", resp, "err", err)
			return err
		}
	}
	//noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got non-200 status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("could not revoke token: HTTP response %v", resp.StatusCode)
	}

	return nil
}

func (b *backend) createToken(config adminConfiguration, role artifactoryRole) (*createTokenResponse, error) {
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

	// Artifactory will not let you revoke a token that has an expiry unless it also meets
	// criteria that can only be set in its configuration file. The version of Artifactory
	// I'm testing against will actually delete a token when you ask it to revoke by token_id,
	// but the token is still usable even after it's deleted. See RTFACT-15293.
	values.Set("expires_in", "0") // never expires

	if role.Audience != "" {
		values.Set("audience", role.Audience)
	}

	newAccessReq, err := b.getSystemStatus(config)
	if err != nil {
		b.Backend.Logger().Warn("could not get artifactory version", "err", err)
		return nil, err
	}

	u, err := url.Parse(config.ArtifactoryURL)
	if err != nil {
		b.Backend.Logger().Warn("could not parse artifactory url", "url", u, "err", err)
		return nil, err
	}

	path := ""

	if newAccessReq == true {
		path = u.Scheme + "://" + u.Host + "/access/api/v1/tokens"
	} else {
		path = u.Scheme + "://" + u.Host + u.Path + "/api/security/token"
	}

	resp, err := b.performArtifactoryRequest(config, path, u.Host, values)
	if err != nil {
		b.Backend.Logger().Warn("error making  token request", "response", resp, "err", err)
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

func (b *backend) getSystemStatus(config adminConfiguration) (bool, error) {

	newAccessReq := false

	u, err := url.Parse(config.ArtifactoryURL)
	if err != nil {
		b.Backend.Logger().Warn("could not parse artifactory url", "url", u, "err", err)
		return newAccessReq, err
	}

	resp, err := b.performArtifactorySystemRequest(config, "/api/system/version", u.Host)
	if err != nil {
		b.Backend.Logger().Warn("error making system version request", "response", resp, "err", err)
		return newAccessReq, err
	}

	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got non-200 status code", "statusCode", resp.StatusCode)
		return newAccessReq, fmt.Errorf("could not get the sytem version: HTTP response %v", resp.StatusCode)
	}

	var systemVersion systemVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&systemVersion); err != nil {
		b.Backend.Logger().Warn("could not parse system version response", "response", resp, "err", err)
		return newAccessReq, err
	}

	v1, err := version.NewVersion(systemVersion.Version)
	v2, err := version.NewVersion("7.21.1")

	if v1.GreaterThan(v2) {
		newAccessReq = true
	}

	return newAccessReq, nil
}

func (b *backend) performArtifactorySystemRequest(config adminConfiguration, path, host string) (*http.Response, error) {
	if !strings.Contains(host, "myserver.com") && !isProxyExists() {
		conn, err := tls.Dial("tcp", host+":443", nil)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
	}

	u, err := url.ParseRequestURI(config.ArtifactoryURL + path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "art-secrets-plugin")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return b.httpClient.Do(req)
}

func (b *backend) performArtifactoryRequest(config adminConfiguration, path, host string, values url.Values) (*http.Response, error) {

	if !strings.Contains(path, "myserver.com") && !isProxyExists() {
		conn, err := tls.Dial("tcp", host+":443", nil)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
	}

	u, err := url.ParseRequestURI(path)
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

func (b *backend) performArtifactoryDelRequest(config adminConfiguration, host, path string) (*http.Response, error) {

	if !strings.Contains(path, "myserver.com") && !isProxyExists() {
		conn, err := tls.Dial("tcp", host+":443", nil)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
	}

	u, err := url.ParseRequestURI(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "art-secrets-plugin")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return b.httpClient.Do(req)
}

func isProxyExists() bool {
	_, p1 := os.LookupEnv("https_proxy")
	_, p2 := os.LookupEnv("HTTPS_PROXY")
	if p1 || p2 {
		return true
	}
	return false
}
