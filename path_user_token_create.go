package artifactory

import (
	"context"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathUserTokenCreate() *framework.Path {
	return &framework.Path{
		Pattern: "user_token/" + framework.GenericNameWithAtRegex("user"),
		Fields: map[string]*framework.FieldSchema{
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
				Callback: b.pathUserTokenCreatePerform,
			},
		},
		HelpSynopsis: `Create an Artifactory access token for the specified user.`,
	}
}

func (b *backend) pathUserTokenCreatePerform(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	// TODO: This isn't documented AFAIK, will this new name be recognized?
	go b.sendUsage(*config, "pathUserTokenCreatePerform")

	// Read in the requested user name
	userName := data.Get("user").(string)
	// TODO: Revise this
	ttl := time.Duration( b.Backend.System().MaxLeaseTTL())

	// TODO: Create config/user_token path to be able to set TTL & enable this path

	role := &artifactoryRole{
		GrantType:  "client_credentials",
		Username:   userName,
		Scope:      "applied-permissions/user",
		Audience:   "*@*",
		DefaultTTL: ttl,
		MaxTTL: ttl,
	}

	resp, err := b.CreateToken(*config, *role)
	if err != nil {
		return nil, err
	}

	response := b.Secret(SecretArtifactoryAccessTokenType).Response(map[string]interface{}{
		"access_token": resp.AccessToken,
		"scope":        resp.Scope,
		"token_id":     resp.TokenId,
		"username":     role.Username,
	}, map[string]interface{}{
		"access_token": resp.AccessToken,
		"token_id":     resp.TokenId,
		"username":     role.Username,
	})

	response.Secret.TTL = ttl
	response.Secret.MaxTTL = role.MaxTTL

	return response, nil
}
