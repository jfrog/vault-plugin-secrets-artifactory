package artifactory

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/vault/sdk/helper/template"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/samber/lo"
)

const (
	defaultUserNameTemplate    string = `{{ printf "v-%s-%s" (.RoleName | truncate 24) (random 8) }}` // Docs indicate max length is 256
	grantTypeClientCredentials string = "client_credentials"
	grantTypeRefreshToken      string = "refresh_token"
)

var ErrIncompatibleVersion = errors.New("incompatible version")

type baseConfiguration struct {
	AccessToken                      string `json:"access_token"`
	ArtifactoryURL                   string `json:"artifactory_url"`
	UseExpiringTokens                bool   `json:"use_expiring_tokens,omitempty"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

func (b *backend) RevokeToken(config baseConfiguration, secret logical.Secret) error {
	tokenId := secret.InternalData["token_id"].(string)
	u, err := url.Parse(config.ArtifactoryURL)
	if err != nil {
		b.Logger().Error("could not parse artifactory url", "url", u, "err", err)
		return err
	}

	var resp *http.Response

	if b.useNewAccessAPI() {
		resp, err = b.performArtifactoryDelete(config, "/access/api/v1/tokens/"+tokenId)
		if err != nil {
			b.Logger().Error("error deleting access token", "tokenId", tokenId, "response", resp, "err", err)
			return err
		}
	} else {
		accessToken := secret.InternalData["access_token"].(string)
		values := url.Values{}
		values.Set("token", accessToken)

		resp, err = b.performArtifactoryPost(config, u.Path+"/artifactory/api/security/token/revoke", values)
		if err != nil {
			b.Logger().Error("error deleting token", "tokenId", tokenId, "response", resp, "err", err)
			return err
		}
	}
	//noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			b.Logger().Error("revokenToken could not read error response body", "err", err)
			return fmt.Errorf("could not parse response body. Err: %v", err)
		}
		b.Logger().Error("revokenToken got non-200 status code", "statusCode", resp.StatusCode, "body", string(body))
		return fmt.Errorf("could not revoke tokenID: %v - HTTP response %v", tokenId, body)
	}

	return nil
}

type CreateTokenRequest struct {
	GrantType             string `json:"grant_type,omitempty"`
	Username              string `json:"username,omitempty"`
	Scope                 string `json:"scope,omitempty"`
	ExpiresIn             int64  `json:"expires_in"`
	Refreshable           bool   `json:"refreshable,omitempty"`
	Description           string `json:"description,omitempty"`
	Audience              string `json:"audience,omitempty"`
	ForceRevocable        bool   `json:"force_revocable,omitempty"`
	IncludeReferenceToken bool   `json:"include_reference_token,omitempty"`
	RefreshToken          string `json:"refresh_token,omitempty"`
}

type createTokenErrorResponse struct {
	Errors []errorResponse `json:"errors"`
}

type TokenExpiredError struct{}

func (e *TokenExpiredError) Error() string {
	return "token has expired"
}

func (b *backend) CreateToken(config baseConfiguration, role artifactoryRole) (*createTokenResponse, error) {
	request := CreateTokenRequest{
		GrantType:             role.GrantType,
		Username:              role.Username,
		Scope:                 role.Scope,
		Audience:              role.Audience,
		Description:           role.Description,
		Refreshable:           role.Refreshable,
		IncludeReferenceToken: role.IncludeReferenceToken,
		RefreshToken:          role.RefreshToken,
	}

	return b.createToken(config, role.ExpiresIn, request)
}

func (b *backend) RefreshToken(config baseConfiguration, role artifactoryRole) (*createTokenResponse, error) {
	if role.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token supplied")
	}

	request := CreateTokenRequest{
		GrantType:    grantTypeRefreshToken,
		RefreshToken: role.RefreshToken,
	}

	return b.createToken(config, role.ExpiresIn, request)
}

func (b *backend) createToken(config baseConfiguration, expiresIn time.Duration, request CreateTokenRequest) (*createTokenResponse, error) {
	if request.GrantType == "client_credentials" && len(request.Username) == 0 {
		return nil, fmt.Errorf("empty username not allowed, possibly a template error")
	}

	// Artifactory will not let you revoke a token that has an expiry unless it also meets
	// criteria that can only be set in its configuration file. The version of Artifactory
	// I'm testing against will actually delete a token when you ask it to revoke by token_id,
	// but the token is still usable even after it's deleted. See RTFACT-15293.
	request.ExpiresIn = 0 // never expires

	if config.UseExpiringTokens && b.supportForceRevocable() && expiresIn > 0 {
		request.ExpiresIn = int64(expiresIn.Seconds())
		request.ForceRevocable = true
	}

	u, err := url.Parse(config.ArtifactoryURL)
	if err != nil {
		b.Logger().Error("could not parse artifactory url", "url", config.ArtifactoryURL, "err", err)
		return nil, err
	}

	path := ""

	if b.useNewAccessAPI() {
		path = "/access/api/v1/tokens"
	} else {
		path = u.Path + "/artifactory/api/security/token"
	}

	jsonReq, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := b.performArtifactoryPostWithJSON(config, path, jsonReq)
	if err != nil {
		b.Logger().Error("error making token request", "response", resp, "err", err)
		return nil, err
	}

	//noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		e := fmt.Errorf("could not create access token: HTTP response %v", resp.StatusCode)

		if resp.StatusCode == http.StatusUnauthorized {
			var errResp createTokenErrorResponse
			err := json.NewDecoder(resp.Body).Decode(&errResp)
			if err != nil {
				b.Logger().Error("could not parse error response", "response", resp, "err", err)
				return nil, fmt.Errorf("could not create access token. Err: %v", err)
			}

			errMessages := lo.Reduce(errResp.Errors, func(agg string, e errorResponse, _ int) string {
				return fmt.Sprintf("%s, %s", agg, e.Message)
			}, "")

			expiredTokenRe := regexp.MustCompile(`.*Invalid token, expired.*`)
			if expiredTokenRe.MatchString(errMessages) {
				return nil, &TokenExpiredError{}
			}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			b.Logger().Error("createToken could not read error response body", "err", err)
			return nil, fmt.Errorf("could not parse response body. Err: %v", e)
		}
		b.Logger().Error("createToken got non-200 status code", "statusCode", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("could not create access token. HTTP response: %s", body)
	}

	var createdToken createTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&createdToken); err != nil {
		b.Logger().Error("could not parse response", "response", resp, "err", err)
		return nil, fmt.Errorf("could not create access token. Err: %v", err)
	}

	return &createdToken, nil
}

// supportForceRevocable verifies whether or not the Artifactory version is 7.50.3 or higher.
// The access API changes in v7.50.3 to support force_revocable to allow us to set the expiration for the tokens.
// REF: https://www.jfrog.com/confluence/display/JFROG/JFrog+Platform+REST+API#JFrogPlatformRESTAPI-CreateToken
func (b *backend) supportForceRevocable() bool {
	return b.checkVersion("7.50.3")
}

// useNewAccessAPI verifies whether or not the Artifactory version is 7.21.1 or higher.
// The access API changed in v7.21.1
// REF: https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API#ArtifactoryRESTAPI-AccessTokens
func (b *backend) useNewAccessAPI() bool {
	return b.checkVersion("7.21.1")
}

// getVersion will fetch the current Artifactory version and store it in the backend
func (b *backend) getVersion(config baseConfiguration) (err error) {
	resp, err := b.performArtifactoryGet(config, "/artifactory/api/system/version")
	if err != nil {
		b.Logger().Error("error making system version request", "response", resp, "err", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Logger().Error("got non-200 status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("could not get the system version: HTTP response %v", resp.StatusCode)
	}

	var systemVersion systemVersionResponse
	if err = json.NewDecoder(resp.Body).Decode(&systemVersion); err != nil {
		b.Logger().Error("could not parse system version response", "response", resp, "err", err)
		return
	}
	b.version = systemVersion.Version
	return
}

// checkVersion will return a boolean and error to check compatibility before making an API call
// -- This was formerly "checkSystemStatus" but that was hard-coded, that method now calls this one
func (b *backend) checkVersion(ver string) (compatible bool) {
	v1, err := version.NewVersion(b.version)
	if err != nil {
		b.Logger().Error("could not parse Artifactory system version", "ver", b.version, "err", err)
		return
	}

	v2, err := version.NewVersion(ver)
	if err != nil {
		b.Logger().Error("could not parse provided version", "ver", ver, "err", err)
		return
	}

	if v1.GreaterThanOrEqual(v2) {
		compatible = true
	}

	return
}

// parseJWT will parse a JWT token string from Artifactory and return a *jwt.Token, err
func (b *backend) parseJWT(config baseConfiguration, token string) (jwtToken *jwt.Token, err error) {
	validate := true

	cert, err := b.getRootCert(config)
	if err != nil {
		if errors.Is(err, ErrIncompatibleVersion) {
			b.Logger().Error("outdated artifactory, unable to retrieve root cert, skipping token validation")
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

type TokenInfo struct {
	TokenID  string `json:"token_id"`
	Scope    string `json:"scope"`
	Username string `json:"username"`
	Expires  int64  `json:"expires"`
}

// getTokenInfo will parse the provided token to return useful information about it
func (b *backend) getTokenInfo(config baseConfiguration, token string) (info *TokenInfo, err error) {
	// Parse Current Token (to get tokenID/scope)
	jwtToken, err := b.parseJWT(config, token)
	if err != nil {
		return
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("error parsing claims in AccessToken")
	}

	sub := strings.Split(claims["sub"].(string), "/") // sub -> subject (jfac@01fr1x1h805xmg0t17xhqr1v7a/users/admin)

	info = &TokenInfo{
		TokenID:  claims["jti"].(string),     // jti -> JFrog Token ID
		Scope:    claims["scp"].(string),     // scp -> scope
		Username: strings.Join(sub[2:], "/"), // 3rd+ elements (incase username has / in it)
	}

	// exp -> expires at (unixtime) - may not be present
	switch exp := claims["exp"].(type) {
	case int64:
		info.Expires = exp
	case float64:
		info.Expires = int64(exp) // close enough this should be int64 anyhow
	case json.Number:
		v, err := exp.Int64()
		if err != nil {
			b.Logger().Error("error parsing token exp as json.Number", "err", err)
		}
		info.Expires = v
	}

	return
}

// getRootCert will return the Artifactory access root certificate's public key, for validating token signatures
func (b *backend) getRootCert(config baseConfiguration) (cert *x509.Certificate, err error) {
	// Verify Artifactory version is at 7.12.0 or higher, prior versions will not work
	// REF: https://jfrog.com/help/r/jfrog-rest-apis/get-root-certificate
	if !b.checkVersion("7.12.0") {
		return cert, ErrIncompatibleVersion
	}

	resp, err := b.performArtifactoryGet(config, "/access/api/v1/cert/root")
	if err != nil {
		b.Logger().Error("error requesting cert/root", "response", resp, "err", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.Logger().Error("got non-200 status code", "statusCode", resp.StatusCode)
		return cert, fmt.Errorf("could not get the certificate: HTTP response %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b.Logger().Error("error reading root cert response body", "err", err)
		return
	}

	// The certificate is base64 encoded DER
	binCert := make([]byte, len(body))
	n, err := base64.StdEncoding.Decode(binCert, body)
	if err != nil {
		b.Logger().Error("error decoding body", "err", err)
		return
	}

	cert, err = x509.ParseCertificate(binCert[0:n])
	if err != nil {
		b.Logger().Error("error parsing certificate", "err", err)
		return
	}
	return
}

type Feature struct {
	FeatureId string `json:"featureId"`
}

type Usage struct {
	ProductId string    `json:"productId"`
	Features  []Feature `json:"features"`
}

func (b *backend) sendUsage(config baseConfiguration, featureId string) {
	features := []Feature{
		{
			FeatureId: featureId,
		},
	}

	usage := Usage{
		productId,
		features,
	}

	jsonReq, err := json.Marshal(usage)
	if err != nil {
		b.Logger().Info("error marshalling call home request", "err", err)
		return
	}

	resp, err := b.performArtifactoryPostWithJSON(config, "artifactory/api/system/usage", jsonReq)
	if err != nil {
		b.Logger().Info("error making call home request", "response", resp, "err", err)
		return
	}

	//noinspection GoUnhandledErrorResult
	defer resp.Body.Close()
}

func (b *backend) performArtifactoryGet(config baseConfiguration, path string) (*http.Response, error) {
	u, err := parseURLWithDefaultPort(config.ArtifactoryURL)
	if err != nil {
		return nil, err
	}

	u.Path = path // replace any path in the URL with the provided path

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", productId)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return b.httpClient.Do(req)
}

// performArtifactoryPost will HTTP POST values to the Artifactory API.
func (b *backend) performArtifactoryPost(config baseConfiguration, path string, values url.Values) (*http.Response, error) {
	u, err := parseURLWithDefaultPort(config.ArtifactoryURL)
	if err != nil {
		return nil, err
	}

	// Replace URL Path
	u.Path = path

	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", productId)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return b.httpClient.Do(req)
}

// performArtifactoryPost will HTTP POST data to the Artifactory API.
func (b *backend) performArtifactoryPostWithJSON(config baseConfiguration, path string, postData []byte) (*http.Response, error) {
	u, err := parseURLWithDefaultPort(config.ArtifactoryURL)
	if err != nil {
		return nil, err
	}

	// Replace URL Path
	u.Path = path

	postDataBuf := bytes.NewBuffer(postData)
	req, err := http.NewRequest(http.MethodPost, u.String(), postDataBuf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", productId)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/json")

	return b.httpClient.Do(req)
}

// performArtifactoryDelete will HTTP DELETE to the Artifactory API.
// The path will be appended to the configured configured URL Path (usually /artifactory)
func (b *backend) performArtifactoryDelete(config baseConfiguration, path string) (*http.Response, error) {

	u, err := parseURLWithDefaultPort(config.ArtifactoryURL)
	if err != nil {
		return nil, err
	}

	// Replace URL Path
	u.Path = path

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", productId)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return b.httpClient.Do(req)
}

func parseURLWithDefaultPort(rawUrl string) (*url.URL, error) {
	urlParsed, err := url.ParseRequestURI(rawUrl)
	if err != nil {
		return nil, err
	}

	if urlParsed.Port() == "" {
		defaultPort, err := net.LookupPort("tcp", urlParsed.Scheme)
		if err != nil {
			return nil, err
		}
		urlParsed.Host = fmt.Sprintf("%s:%d", urlParsed.Host, defaultPort)
	}

	return urlParsed, nil
}

func testUsernameTemplate(testTemplate string) (up template.StringTemplate, err error) {
	up, err = template.NewTemplate(template.Template(testTemplate))
	if err != nil {
		return up, fmt.Errorf("username_template initialization error: %w", err)
	}
	_, err = up.Generate(UsernameMetadata{})
	if err != nil {
		return up, fmt.Errorf("username_template failed to generate username: %w", err)
	}
	return
}
