package artifactory

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/vault/sdk/logical"
)

var ErrIncompatibleVersion = errors.New("incompatible version")

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

	if newAccessReq {
		resp, err = b.performArtifactoryDelete(config, "/access/api/v1/tokens/"+tokenId)
		if err != nil {
			b.Backend.Logger().Warn("error deleting access token", "response", resp, "err", err)
			return err
		}

	} else {
		resp, err = b.performArtifactoryPost(config, u.Path+"/api/security/token/revoke", values)
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

	if newAccessReq {
		path = "/access/api/v1/tokens"
	} else {
		path = u.Path + "/api/security/token"
	}

	resp, err := b.performArtifactoryPost(config, path, values)
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

// getSystemStatus verifies whether or not the Artifactory version is 7.21.1 or higher.
// The access API changed in v7.21.1
// REF: https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API#ArtifactoryRESTAPI-AccessTokens
func (b *backend) getSystemStatus(config adminConfiguration) (bool, error) {
	return b.checkVersion(config, "7.21.1")
}

// checkVersion will return a boolean and error to check compatibilty before making an API call
// -- This was formerly "checkSystemStatus" but that was hard-coded, that method now calls this one
func (b *backend) checkVersion(config adminConfiguration, ver string) (compatible bool, err error) {
	resp, err := b.performArtifactoryGet(config, "/artifactory/api/system/version")
	if err != nil {
		b.Backend.Logger().Warn("error making system version request", "response", resp, "err", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got non-200 status code", "statusCode", resp.StatusCode)
		return compatible, fmt.Errorf("could not get the sytem version: HTTP response %v", resp.StatusCode)
	}

	var systemVersion systemVersionResponse
	if err = json.NewDecoder(resp.Body).Decode(&systemVersion); err != nil {
		b.Backend.Logger().Warn("could not parse system version response", "response", resp, "err", err)
		return
	}

	v1, err := version.NewVersion(systemVersion.Version)
	if err != nil {
		b.Backend.Logger().Warn("could not parse Artifactory system version", "ver", systemVersion.Version, "err", err)
		return
	}

	v2, err := version.NewVersion(ver)
	if err != nil {
		b.Backend.Logger().Warn("could not parse provided version", "ver", ver, "err", err)
		return
	}

	if v1.GreaterThanOrEqual(v2) {
		compatible = true
	}

	return
}

// parseJWT will parse a JWT token string from Artifactory and return a *jwt.Token, err
func (b *backend) parseJWT(config adminConfiguration, token string) (jwtToken *jwt.Token, err error) {
	validate := true

	cert, err := b.getRootCert(config)
	if err != nil {
		if errors.Is(err, ErrIncompatibleVersion) {
			b.Logger().Warn("outdated artifactory, unable to retrieve root cert, skipping token validation")
			validate = false
		} else {
			b.Logger().Error("error retrieving root cert", "err", err.Error())
			return
		}
	}

	// Parse Token
	if validate {
		jwtToken, err = jwt.Parse(token,
			func(token *jwt.Token) (interface{}, error) { return cert.PublicKey, nil },
			jwt.WithValidMethods([]string{"RS256"}))
		if err != nil {
			return
		}
		if !jwtToken.Valid {
			return
		}
	} else { // SKIP Validation
		// -- NOTE THIS IGNORES THE SIGNATURE, which is probably bad,
		//    but it is artifactory's job to validate the token, right?
		// p := jwt.Parser{}
		// token, _, err := p.ParseUnverified(oldAccessToken, jwt.MapClaims{})
		jwtToken, err = jwt.Parse(token, nil, jwt.WithoutClaimsValidation())
		if err != nil {
			return
		}
	}

	// If we got here, we should have a jwtToken and nil err
	return
}

// getRootCert will return the Artifactory access root certificate's public key, for validating token signatures
func (b *backend) getRootCert(config adminConfiguration) (cert *x509.Certificate, err error) {
	// Verify Artifactory version is at 7.12.0 or higher, prior versions will not work
	// REF: https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API#ArtifactoryRESTAPI-GetRootCertificate
	compatible, err := b.checkVersion(config, "7.12.0")
	if err != nil {
		b.Backend.Logger().Warn("could not get artifactory version", "err", err)
		return
	}
	if !compatible {
		return cert, ErrIncompatibleVersion
	}

	resp, err := b.performArtifactoryGet(config, "/access/api/v1/cert/root")
	if err != nil {
		b.Backend.Logger().Warn("error requesting cert/root", "response", resp, "err", err)
		return
	}

	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		b.Backend.Logger().Warn("got non-200 status code", "statusCode", resp.StatusCode)
		return cert, fmt.Errorf("could not get the certificate: HTTP response %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	// body, err := ioutil.ReadAll(resp.Body)  Go.1.15 and earlier
	if err != nil {
		b.Backend.Logger().Error("error reading root cert response body", "err", err)
		return
	}

	// The certificate is base64 encoded DER
	binCert := make([]byte, len(body))
	n, err := base64.StdEncoding.Decode(binCert, body)
	if err != nil {
		b.Backend.Logger().Error("error decoding body", "err", err)
		return
	}

	cert, err = x509.ParseCertificate(binCert[0:n])
	if err != nil {
		b.Backend.Logger().Error("error parsing certificate", "err", err)
		return
	}
	return
}

func (b *backend) performArtifactoryGet(config adminConfiguration, path string) (*http.Response, error) {
	u, err := url.ParseRequestURI(config.ArtifactoryURL)
	if err != nil {
		return nil, err
	}

	u.Path = path // replace any path in the URL with the provided path

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "art-secrets-plugin")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return b.httpClient.Do(req)
}

// performArtifactoryPost will HTTP POST values to the Artifactory API.
func (b *backend) performArtifactoryPost(config adminConfiguration, path string, values url.Values) (*http.Response, error) {

	u, err := url.ParseRequestURI(config.ArtifactoryURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "https" && !strings.Contains(u.Host, "myserver.com") && !isProxyExists() {
		conn, err := tls.Dial("tcp", u.Host, nil)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
	}

	// Replace URL Path
	u.Path = path

	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(values.Encode()))

	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return b.httpClient.Do(req)
}

// performArtifactoryDelete will HTTP DELETE to the Artifactory API.
//   The path will be appended to the configured configured URL Path (usually /artifactory)
func (b *backend) performArtifactoryDelete(config adminConfiguration, path string) (*http.Response, error) {

	u, err := url.ParseRequestURI(config.ArtifactoryURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "https" && !strings.Contains(u.Host, "myserver.com") && !isProxyExists() {
		conn, err := tls.Dial("tcp", u.Host, nil)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
	}

	// Replace URL Path
	u.Path = path

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
