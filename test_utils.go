package artifactory

import (
	"context"
	"os"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/hashicorp/vault/sdk/logical"
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

// createNewTestToken creates a new scoped token using the one from test environment
// so that the original token won't be revoked by the path config rotate test
func (e *accTestEnv) createNewTestToken(t *testing.T) (string, string) {
	config := adminConfiguration{
		AccessToken:    e.AccessToken,
		ArtifactoryURL: e.URL,
	}

	role := artifactoryRole{
		GrantType: "client_credentials",
		Username:  "admin",
		Scope:     "applied-permissions/admin",
	}

	err := e.Backend.(*backend).getVersion(config)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := e.Backend.(*backend).CreateToken(config, role)
	if err != nil {
		t.Fatal(err)
	}

	return resp.TokenId, resp.AccessToken
}

func (e *accTestEnv) revokeTestToken(t *testing.T, accessToken string, tokenID string) {
	config := adminConfiguration{
		AccessToken:    e.AccessToken,
		ArtifactoryURL: e.URL,
	}

	err := e.Backend.(*backend).getVersion(config)
	if err != nil {
		t.Fatal(err)
	}

	secret := logical.Secret{
		InternalData: map[string]interface{}{
			"access_token": accessToken,
			"token_id":     tokenID,
		},
	}

	err = e.Backend.(*backend).RevokeToken(config, secret)
	if err != nil {
		t.Fatal(err)
	}
}

func (e *accTestEnv) UpdatePathConfig(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/admin",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"access_token": e.AccessToken,
			"url":          e.URL,
		},
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func (e *accTestEnv) ReadPathConfig(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "config/admin",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token_sha256"])
}

func (e *accTestEnv) DeletePathConfig(t *testing.T) {
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "config/admin",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func (e *accTestEnv) RotatePathConfig(t *testing.T) {
	// create new test token
	tokenID, accessToken := e.createNewTestToken(t)

	// setup new path configuration
	resp, err := e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/admin",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"access_token": accessToken,
			"url":          e.URL,
		},
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)

	// read back the path configuration and save the access token hash for comparison later
	resp, err = e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "config/admin",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token_sha256"])

	accessTokenHash := resp.Data["access_token_sha256"]

	// rotate the path config
	resp, err = e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/rotate",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)

	// read back the path configuration and assert access token hash is now different
	resp, err = e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "config/admin",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data["access_token_sha256"])
	assert.NotEqual(t, accessTokenHash, resp.Data["access_token_sha256"])

	// clean up path config
	resp, err = e.Backend.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "config/admin",
		Storage:   e.Storage,
	})

	assert.NoError(t, err)
	assert.Nil(t, resp)

	// revoke the test token
	e.revokeTestToken(t, accessToken, tokenID)
}

func (e *accTestEnv) CreatePathRole(t *testing.T) {
	roleData := map[string]interface{}{
		"role":        "test-role",
		"username":    "admin",
		"scope":       "applied-permissions/user",
		"audience":    "*@*",
		"default_ttl": 30 * time.Minute,
		"max_ttl":     45 * time.Minute,
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
}

func newAcceptanceTestEnv() (*accTestEnv, error) {
	ctx := context.Background()

	conf := &logical.BackendConfig{
		System: &logical.StaticSystemView{},
		Logger: logging.NewVaultLogger(log.Debug),
	}
	backend, err := Factory(ctx, conf)
	if err != nil {
		return nil, err
	}
	return &accTestEnv{
		AccessToken: os.Getenv("ARTIFACTORY_ACCESS_TOKEN"),
		URL:         os.Getenv("JFROG_URL"),
		Backend:     backend,
		Context:     ctx,
		Storage:     &logical.InmemStorage{},
	}, nil
}

const rootCert string = `MIIDHzCCAgegAwIBAgIQHC4IERZbTl67GGjV8KH04jANBgkqhkiG9w0BAQ` +
	`sFADA9MTswOQYDVQQDDDJKRnJvZyBUb2tlbiBJc3N1ZXIgamZhY0AwMWc1aGVrNmtiMjk1MjB` +
	`yYno3MXY5MWN3OTAgFw0yMjA2MTMxNTUxMjdaGA83MDAwMDEwMTAwMDAyN1owPTE7MDkGA1UE` +
	`AwwySkZyb2cgVG9rZW4gSXNzdWVyIGpmYWNAMDFnNWhlazZrYjI5NTIwcmJ6NzF2OTFjdzkwg` +
	`gEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCAArmgZSKRHWCOFKQy58EG/4soW93WoH` +
	`W5KDvuDfoJKkejD9nrdmRsDbw2wyKPfqgsFz63zdOI3mBGLRRUqHxrXQc6UNxWerYuLzfb/rg` +
	`gby6VzXHPGKft8eiO8w9TNMibf30MY/xFwmHWamECjZ5L9pTc8n1txizEPNW8farqQXXlli2N` +
	`PymEK/G3xW1QQWfThY5lMqTjvg6DYvB5ZQMbl853S+nsW10rWHSeFpnXFo46kNN5VaoXlJunZ` +
	`hPk3mm1rLIR6HLLeOPRSTIsVCqwhQbnRV84HZMVQnG9355L1EzbeEZAZjWC4r9hOmtyt4rcuq` +
	`dnYuGLR3Yw2cZEILKvAgMBAAGjGTAXMBUGBFUdEQEEDTALoAkGBFUdEQECAQAwDQYJKoZIhvc` +
	`NAQELBQADggEBAHblGVlZR9uyZN7sNpd7zDiVaoCJjuSFwmnEjrRqzMNxqqBixYXAb2LgeFya` +
	`MqLT0WEEB5v8BQL0FlsKPob9GpzMiLfFxhQGpR5K57nRlN5Qws+XWSCydi0tBAC5mHJea8VZB` +
	`j9REsFUEtgE7En2BDBRD/4DcM+d0bmyXh7GKYLoMcSEQJ+zpSJ4AwXraKKkcIwqcXMkNZhbMz` +
	`l/EyhwOsDvBRb1t0VJkrS9s01buqz+gkrPwm5+0+BhLxCfT1PP5DBhs72Pt/1UPOlDLPuf/AB` +
	`bZoWR2vqNvX+ia1bsAJvx56K1KkRSswhJOPCSWLnPcB/Eh6oWUY0dZQQN+5v6Hm8=`

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

	_, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/admin",
		Storage:   config.StorageView,
		Data:      adminConfig,
	})
	assert.NoError(t, err)

	return b, config
}
