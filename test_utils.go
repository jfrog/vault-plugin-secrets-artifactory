package artifactory

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

func newTestClient(fn roundTripperFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
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

func tokenCreatedResponse(token string) roundTripperFunc {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString(token)),
		}, nil
	}
}

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
