package artifactory

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

var runAcceptanceTests = os.Getenv("VAULT_ACC") != ""

// accTestEnv creates an object to store and track testing environment
// resources
type accTestEnv struct {
	AccessToken string
	URL         string

	Backend logical.Backend
	Context context.Context
	Storage logical.Storage
}

type testData map[string]interface{}

// createNewTestToken creates a new scoped token using the one from test environment
// so that the original token won't be revoked by the path config rotate test
func (e *accTestEnv) createNewTestToken(t *testing.T) (string, string) {
	config := adminConfiguration{
		baseConfiguration: baseConfiguration{
			AccessToken:    e.AccessToken,
			ArtifactoryURL: e.URL,
		},
	}

	role := artifactoryRole{
		GrantType: "client_credentials",
		Username:  "admin",
		Scope:     "applied-permissions/admin",
	}

	e.Backend.(*backend).InitializeHttpClient(&config)

	resp, err := e.Backend.(*backend).CreateToken(config.baseConfiguration, role)
	if err != nil {
		t.Fatal(err)
	}

	return resp.TokenId, resp.AccessToken
}

// createNewNonAdminTestToken creates a new "user" token using the one from test environment
// primarily used to fail tests
func (e *accTestEnv) createNewNonAdminTestToken(t *testing.T) (string, string) {
	config := adminConfiguration{
		baseConfiguration: baseConfiguration{
			AccessToken:    e.AccessToken,
			ArtifactoryURL: e.URL,
		},
	}

	role := artifactoryRole{
		GrantType: "client_credentials",
		Username:  "notTheAdmin",
		Scope:     "applied-permissions/groups:readers",
	}

	e.Backend.(*backend).InitializeHttpClient(&config)

	resp, err := e.Backend.(*backend).CreateToken(config.baseConfiguration, role)
	if err != nil {
		t.Fatal(err)
	}

	return resp.TokenId, resp.AccessToken
}

func (e *accTestEnv) revokeTestToken(t *testing.T, accessToken string, tokenID string) {
	config := baseConfiguration{
		AccessToken:    e.AccessToken,
		ArtifactoryURL: e.URL,
	}

	err := e.Backend.(*backend).RevokeToken(config, tokenID, accessToken)
	if err != nil {
		t.Fatal(err)
	}
}

func (e *accTestEnv) UpdatePathConfig(t *testing.T) {
	e.UpdateConfigAdmin(t, testData{
		"access_token":         e.AccessToken,
		"url":                  e.URL,
		"allow_scope_override": true,
	})
}

// UpdateConfigAdmin will send a POST/PUT to the /config/admin endpoint with testData (vault write artifactory/config/admin)
func (e *accTestEnv) UpdateConfigAdmin(t *testing.T, data testData) {
	resp, err := e.update(configAdminPath, data)
	assert.NoError(t, err)
	assert.Nil(t, resp)
}

// UpdateConfigAdmin will send a POST/PUT to the /config/user_token endpoint with testData (vault write artifactory/config/user_token)
func (e *accTestEnv) UpdateConfigUserToken(t *testing.T, username string, data testData) {
	path := configUserTokenPath
	if len(username) > 0 && !strings.HasSuffix(path, username) {
		path = fmt.Sprintf("%s/%s", path, username)
	}

	resp, err := e.update(path, data)
	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func (e *accTestEnv) ReadPathConfig(t *testing.T) {
	_ = e.ReadConfigAdmin(t)
}

// ReadConfigAdmin will send a GET to the /config/admin endpoint (vault read artifactory/config/admin)
func (e *accTestEnv) ReadConfigAdmin(t *testing.T) testData {
	resp, err := e.read(configAdminPath)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token_sha256"])
	return resp.Data
}

// ReadConfigUserToken will send a GET to the /config/user_token endpoint (vault read artifactory/config/user_token)
func (e *accTestEnv) ReadConfigUserToken(t *testing.T, username string) testData {
	path := configUserTokenPath
	if len(username) > 0 && !strings.HasSuffix(path, username) {
		path = fmt.Sprintf("%s/%s", path, username)
	}

	resp, err := e.read(path)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	return resp.Data
}

func (e *accTestEnv) DeletePathConfig(t *testing.T) {
	e.DeleteConfigAdmin(t)
}

// DeleteConfigAdmin will send a DELETE to the /config/admin endpoint (vault delete artifactory/config/admin)
func (e *accTestEnv) DeleteConfigAdmin(t *testing.T) {
	resp, err := e.Backend.HandleRequest(e.Context, &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      configAdminPath,
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}

// UpdateConfigRotate will send a POST/PUT to the /config/rotate endpoint with testData (vault write artifactory/config/rotate) and test for errors
func (e *accTestEnv) UpdateConfigRotate(t *testing.T, data testData) {
	resp, err := e.update("config/rotate", data)
	assert.NoError(t, err)
	assert.Nil(t, resp)
}

// read will send a GET  to "path"
func (e *accTestEnv) read(path string) (*logical.Response, error) {
	return e.Backend.HandleRequest(e.Context, &logical.Request{
		Operation: logical.ReadOperation,
		Path:      path,
		Storage:   e.Storage,
	})
}

// update will send a POST/PUT to "path" with testData
func (e *accTestEnv) update(path string, data testData) (*logical.Response, error) {
	return e.Backend.HandleRequest(e.Context, &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      path,
		Storage:   e.Storage,
		Data:      data,
	})
}

func (e *accTestEnv) CreatePathRole(t *testing.T) {
	roleData := map[string]interface{}{
		"role":                    "test-role",
		"username":                "admin",
		"scope":                   "applied-permissions/user",
		"audience":                "*@*",
		"refreshable":             true,
		"include_reference_token": true,
		"default_ttl":             30 * time.Minute,
		"max_ttl":                 45 * time.Minute,
	}

	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   e.Storage,
		Data:      roleData,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func (e *accTestEnv) CreatePathAdminRole(t *testing.T) {
	roleData := map[string]interface{}{
		"role":        "admin-role",
		"username":    "admin",
		"scope":       "applied-permissions/groups:admin",
		"audience":    "*@*",
		"default_ttl": 30 * time.Minute,
		"max_ttl":     45 * time.Minute,
	}

	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/admin-role",
		Storage:   e.Storage,
		Data:      roleData,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func (e *accTestEnv) ReadPathRole(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "roles/test-role",
		Storage:   e.Storage,
	})

	assert.NotNil(t, resp)
	assert.NoError(t, err)

	assert.EqualValues(t, "admin", resp.Data["username"])
	assert.EqualValues(t, "applied-permissions/user", resp.Data["scope"])
	assert.EqualValues(t, "*@*", resp.Data["audience"])
	assert.EqualValues(t, true, resp.Data["refreshable"])
	assert.EqualValues(t, true, resp.Data["include_reference_token"])
	assert.EqualValues(t, 30*time.Minute.Seconds(), resp.Data["default_ttl"])
	assert.EqualValues(t, 45*time.Minute.Seconds(), resp.Data["max_ttl"])
}

func (e *accTestEnv) DeletePathRole(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "roles/test-role",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func (e *accTestEnv) CreatePathToken(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token"])
	assert.NotEmpty(t, resp.Data["token_id"])
	assert.Equal(t, "admin", resp.Data["username"])
	assert.Equal(t, "test-role", resp.Data["role"])
	assert.Equal(t, "applied-permissions/user", resp.Data["scope"])
	assert.NotEmpty(t, resp.Data["refresh_token"])
	assert.NotEmpty(t, resp.Data["reference_token"])
}

func (e *accTestEnv) CreatePathToken_overrides(t *testing.T) {
	e.update(configAdminPath, map[string]interface{}{
		"use_expiring_tokens": true,
	})

	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/test-role",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"max_ttl": 60,
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token"])
	assert.NotEmpty(t, resp.Data["token_id"])
	assert.Equal(t, "admin", resp.Data["username"])
	assert.Equal(t, "test-role", resp.Data["role"])
	assert.Equal(t, "applied-permissions/user", resp.Data["scope"])
	assert.NotEmpty(t, resp.Data["refresh_token"])
	assert.NotEmpty(t, resp.Data["reference_token"])
	assert.Equal(t, 60, resp.Data["expires_in"])
}

func (e *accTestEnv) CreatePathScopedDownToken(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/admin-role",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"scope": "applied-permissions/groups:test-group",
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token"])
	assert.NotEmpty(t, resp.Data["token_id"])
	assert.Equal(t, "admin", resp.Data["username"])
	assert.Equal(t, "admin-role", resp.Data["role"])
	assert.Equal(t, "applied-permissions/groups:test-group", resp.Data["scope"])
}

func (e *accTestEnv) CreatePathScopedDownTokenBadScope(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "token/admin-role",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"scope": "blueberries?pancakes",
		},
	})

	assert.Error(t, err, "provided scope is invalid")
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Data["access_token"])
	assert.Empty(t, resp.Data["token_id"])
	assert.NotEqual(t, "admin", resp.Data["username"])
	assert.NotEqual(t, "admin-role", resp.Data["role"])
	assert.NotEqual(t, "applied-permissions/groups:test-group", resp.Data["scope"])
}

func (e *accTestEnv) CreatePathUserToken(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      configUserTokenPath + "/admin",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"default_description":     "foo",
			"refreshable":             true,
			"include_reference_token": true,
			"use_expiring_tokens":     true,
			"default_ttl":             60,
		},
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)

	resp, err = e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      createUserTokenPath + "admin",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token"])
	assert.NotEmpty(t, resp.Data["token_id"])
	assert.Equal(t, "admin", resp.Data["username"])
	assert.Equal(t, "applied-permissions/user", resp.Data["scope"])
	assert.Equal(t, "foo", resp.Data["description"])
	assert.Equal(t, 60, resp.Data["expires_in"])
	assert.NotEmpty(t, resp.Data["refresh_token"])
	assert.NotEmpty(t, resp.Data["reference_token"])
}

func (e *accTestEnv) CreatePathUserToken_overrides(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      configUserTokenPath + "/admin",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"default_description": "foo",
			"default_ttl":         600,
		},
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)

	resp, err = e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      createUserTokenPath + "admin",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"description":             "buffalo",
			"refreshable":             true,
			"include_reference_token": true,
			"use_expiring_tokens":     true,
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token"])
	assert.NotEmpty(t, resp.Data["token_id"])
	assert.Equal(t, "admin", resp.Data["username"])
	assert.Equal(t, "applied-permissions/user", resp.Data["scope"])
	assert.Equal(t, "buffalo", resp.Data["description"])
	assert.Equal(t, 600, resp.Data["expires_in"])
	assert.NotEmpty(t, resp.Data["refresh_token"])
	assert.NotEmpty(t, resp.Data["reference_token"])
}

// Cleanup will delete the admin configuration and revoke the token
func (e *accTestEnv) Cleanup(t *testing.T) {
	data := e.ReadConfigAdmin(t)
	e.DeleteConfigAdmin(t)

	// revoke the test token
	e.revokeTestToken(t, e.AccessToken, data["token_id"].(string))
}

func newAcceptanceTestEnv() (*accTestEnv, error) {
	ctx := context.Background()

	conf := &logical.BackendConfig{
		System: &logical.StaticSystemView{
			DefaultLeaseTTLVal: 24 * time.Hour,      // 1 day
			MaxLeaseTTLVal:     30 * 24 * time.Hour, // 30 days
		},
		Logger: logging.NewVaultLogger(log.Debug),
	}
	backend, err := Factory(ctx, conf)
	if err != nil {
		return nil, err
	}
	return &accTestEnv{
		AccessToken: os.Getenv("JFROG_ACCESS_TOKEN"),
		URL:         os.Getenv("JFROG_URL"),
		Backend:     backend,
		Context:     ctx,
		Storage:     &logical.InmemStorage{},
	}, nil
}

// NewConfiguredAcceptanceTestEnv will return an *accTestEnv that is already configured (entry point for most tests)
func NewConfiguredAcceptanceTestEnv(t *testing.T) (e *accTestEnv) {
	e, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	// create new test token
	_, accessToken := e.createNewTestToken(t)

	// setup new path configuration
	e.UpdateConfigAdmin(t, testData{
		"access_token":                        accessToken,
		"url":                                 e.URL,
		"bypass_artifactory_tls_verification": false,
	})

	return
}

const rootCert string = `MIIDHzCCAgegAwIBAgIQceT9KWrOSzCrxJZC8H7SljANBgkqhkiG9w0BAQ` +
	`sFADA9MTswOQYDVQQDDDJKRnJvZyBUb2tlbiBJc3N1ZXIgamZhY0AwMWg0MjRodndweXR6azF` +
	`henhoNms4MDdlNTAgFw0yMzA2MjcyMzEzNThaGA83MDAwMDEwMTAwMDA1OFowPTE7MDkGA1UE` +
	`AwwySkZyb2cgVG9rZW4gSXNzdWVyIGpmYWNAMDFoNDI0aHZ3cHl0emsxYXp4aDZrODA3ZTUwg` +
	`gEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCr6k59JmMlWScx6ib++v83L5MKecadO8` +
	`awKfhzgBxPVbJHYyvBx8yl2MBaHu+5AqIxRvNUPaGIbb/reMuteLj+wKUFsAgYFdhKC6SHMxu` +
	`+nbDciinY6o2g4aVZnmY1YUAi7hsiK2xl2aXVVn/ErQg/Hd00KAlJjs9x56nmAhIubZdEdYQx` +
	`jdBHQweix3HwncqWwL0gDymeVcpLFh5WzNR7KSOPpWlqVVhEMkCndbzkj4wDLMAIJ4phTH71C` +
	`lc7W4/KLolRP8jOIuemvdoFc3uRqrPQRU3QsGWLdJ59mwob5L7gtGfOYbQLhifFoltygvIRg8` +
	`NBiByDAJiDkuTsDG4DAgMBAAGjGTAXMBUGBFUdEQEEDTALoAkGBFUdEQECAQAwDQYJKoZIhvc` +
	`NAQELBQADggEBAHQJYVNvBjThimrfIvHi6mTs5nZO2GNaNQsDkFtCEeRSBLSOj3xirT50r5oH` +
	`D2/0QlT7eKZ6t0tKmpl9STO58aKl2OHY2h517Q9i8ZJtYp7XMI3/bgxUfvgCet3l3LykT3alO` +
	`M0IzB2h3nxeMwcROA0BU8hoTPY4aHG5k4fqGGP5grtxVW+v/LFjmGMl3ymjWML4VpvE0SCFOZ` +
	`hM2/OCbLtGx5EO6gOReSUWcFvkxbkutRFQ+ZPakxSH01f5haPitNaT88GaaDXqKy/goYNh0V4` +
	`EJ3F6q2rXgsKUErCO7xSgSoIMOc0XBk+zPwGjg0Nb1sva3yTHXzEG+nAF1ttDrwo=`

const jwtAccessToken string = `
	{
		"token_id" : "59e39159-19eb-463d-953d-1d6baf567db6",
		"access_token" : "eyJ2ZXIiOiIyIiwidHlwIjoiSldUIiwiYWxnIjoiUlMyNTYiLCJraW` +
	`QiOiJxdkhkX3lTNWlPQTlfQ3E5Z3BVSl9WdDBzYVhsTExhdWk2SzFrb291MEJzIn0.eyJ` +
	`leHQiOiJ7XCJyZXZvY2FibGVcIjpcInRydWVcIn0iLCJzdWIiOiJqZmFjQDAxZzVoZWs2` +
	`a2IyOTUyMHJiejcxdjkxY3c5XC91c2Vyc1wvYWRtaW4iLCJzY3AiOiJhcHBsaWVkLXBlc` +
	`m1pc3Npb25zXC9hZG1pbiIsImF1ZCI6IipAKiIsImlzcyI6ImpmYWNAMDFnNWhlazZrYj` +
	`I5NTIwcmJ6NzF2OTFjdzkiLCJleHAiOjE2ODY3ODA4MjgsImlhdCI6MTY1NTI0NDgyOCw` +
	`ianRpIjoiNTllMzkxNTktMTllYi00NjNkLTk1M2QtMWQ2YmFmNTY3ZGI2In0.IaWDbYM-` +
	`NkDA9KVkCHlYMJAOD0CvOH3Hq4t2P3YYm8B6G1MddH46VPKGPySr4st5KmMInfW-lmg6I` +
	`fXjVarlkJVT8AkiaTBOR7EJFC5kqZ80OHOtYKusIHZx_7aEuDC6f9mijwuxz5ERd7WmYn` +
	`Jn3hOwLd7_94hScX9gWfmYcT3xZNjTS48BmXOqPyXu-XtfZ9K-X9zQNtHv6j9qFNtwwTf` +
	`v9GN8wnwTJ-e4xpginFQh-9YETaWUVtvOsm2-VtM5vDsszYtg8FM-Bz3JFNqJTFlvDs75` +
	`ATmHEjwoCIa7Vzg_GqAgFFRrW3SYwW3GpPyk8vJT9xLmEBBwVUVl2Ngjdw",
		"expires_in" : 31536000,
		"scope" : "applied-permissions/admin",
		"token_type" : "Bearer"
	}`

// Literally https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API#ArtifactoryRESTAPI-CreateToken
const canonicalAccessToken = `{
   "access_token":   "eyXsdgbtybbeeyh...",
   "expires_in":    0,
   "scope":         "api:* member-of-groups:example",
   "token_type":    "Bearer",
   "refresh_token": "fgsfgsdugh8dgu9s8gy9hsg..."
}`

const artVersion = `{
    "version": "7.19.10",
    "revision": "71910900",
    "license": "05179b957028fa9aa1ceb88da6519a245e55b9fc5"
}`

func makeBackend(t *testing.T) (*backend, *logical.BackendConfig) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, err := Backend(config)
	if err != nil {
		t.Fatal(err)
	}

	if err := b.Setup(context.Background(), config); err != nil {
		t.Fatal(err)
	}

	return b, config
}

func configuredBackend(t *testing.T, adminConfig map[string]interface{}) (*backend, *logical.BackendConfig) {

	b, config := makeBackend(t)
	t.Logf("b.System().MaxLeaseTTL(): %v\n", b.System().MaxLeaseTTL())
	b.InitializeHttpClient(&adminConfiguration{})

	_, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      configAdminPath,
		Storage:   config.StorageView,
		Data:      adminConfig,
	})
	assert.NoError(t, err)

	return b, config
}

func mockArtifactoryUsageVersionRequests(version string) {
	versionString := version
	if len(version) == 0 {
		versionString = artVersion
	}

	httpmock.RegisterResponder(
		http.MethodPost,
		"http://myserver.com:80/artifactory/api/system/usage",
		httpmock.NewStringResponder(200, ""))
	httpmock.RegisterResponder(
		http.MethodGet,
		"http://myserver.com:80/artifactory/api/system/version",
		httpmock.NewStringResponder(200, versionString))
}
