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
			"description": {
				Type:        framework.TypeString,
				Description: `Optional. Description for the user token.`,
			},
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Override the maximum TTL for this access token. Cannot exceed smallest (system, backend) maximum TTL.`,
			},
			"ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Override the default TTL when issuing this access token. Cannot exceed smallest (system, backend, this request) maximum TTL.`,
			},
			"user": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `The name of the user.`,
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

	go b.sendUsage(*config, "pathUserTokenCreatePerform")

	role := &artifactoryRole{
		GrantType: "client_credentials",
		Username:  data.Get("user").(string),
		Scope:     "applied-permissions/user",
	}

	if value, ok := data.GetOk("max_ttl"); ok {
		role.MaxTTL = time.Duration(value.(int)) * time.Second
	} else {
		role.MaxTTL = config.UserTokensMaxTTL
	}

	var ttl time.Duration
	if value, ok := data.GetOk("ttl"); ok {
		ttl = time.Second * time.Duration(value.(int))
	} else {
		ttl = role.DefaultTTL
	}

	maxLeaseTTL := b.Backend.System().MaxLeaseTTL()

	// Set the role.MaxTTL based on maxLeaseTTL
	// - This value will be passed to createToken and used as expires_in for versions of Artifactory 7.50.3 or higher
	if role.MaxTTL == 0 || role.MaxTTL > maxLeaseTTL {
		role.MaxTTL = maxLeaseTTL
	}

	if role.MaxTTL > 0 && ttl > role.MaxTTL {
		ttl = role.MaxTTL
	}

	if value, ok := data.GetOk("audience"); ok {
		role.Audience = value.(string)
	}

	if value, ok := data.GetOk("description"); ok {
		role.Description = value.(string)
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
