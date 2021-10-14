package artifactory

import (
	"context"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathTokenCreate() *framework.Path {
	return &framework.Path{
		Pattern: "token/" + framework.GenericNameWithAtRegex("role"),
		Fields: map[string]*framework.FieldSchema{
			"role": {
				Type:        framework.TypeString,
				Description: `Use the configuration of the specified role.`,
			},
			"ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Override the default TTL when issuing this access token. Cannot exceed smallest (system, backend, role, this request) maximum TTL.`,
			},
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Override the maximum TTL for this access token. Cannot exceed smallest (system, backend) maximum TTL.`,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathTokenCreatePerform,
			},
		},
		HelpSynopsis: `Create an Artifactory access token for the specified role.`,
	}
}

type systemVersionResponse struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

type createTokenResponse struct {
	TokenId     string `json:"token_id"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

func (b *backend) pathTokenCreatePerform(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.rolesMutex.RLock()
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()
	defer b.rolesMutex.RUnlock()

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	// Read in the requested role
	roleName := data.Get("role").(string)

	role, err := b.Role(ctx, req.Storage, roleName)

	if err != nil {
		return nil, err
	}

	if role == nil {
		return logical.ErrorResponse("no such role"), nil
	}

	var ttl time.Duration
	if value, ok := data.GetOk("ttl"); ok {
		ttl = time.Second * time.Duration(value.(int))
	} else {
		ttl = role.DefaultTTL
	}

	maxLeaseTTL := b.Backend.System().MaxLeaseTTL()
	maxTTL := role.MaxTTL

	if maxTTL == 0 {
		maxTTL = maxLeaseTTL
	} else if maxTTL > maxLeaseTTL {
		maxTTL = maxLeaseTTL
	}

	if maxTTL > 0 && ttl > maxTTL {
		ttl = maxTTL
	}

	resp, err := b.createToken(*config, *role)
	if err != nil {
		return nil, err
	}

	response := b.Secret(SecretArtifactoryAccessTokenType).Response(map[string]interface{}{
		"access_token": resp.AccessToken,
		"role":         roleName,
		"scope":        resp.Scope,
		"token_id":     resp.TokenId,
	}, map[string]interface{}{
		"role":         roleName,
		"access_token": resp.AccessToken,
		"token_id":     resp.TokenId,
	})

	response.Secret.TTL = ttl
	response.Secret.MaxTTL = maxTTL

	return response, nil
}
