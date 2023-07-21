package artifactory

import (
	"context"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathUserTokenCreate() *framework.Path {
	return &framework.Path{
		Pattern: "user_token/" + framework.GenericNameWithAtRegex("username"),
		Fields: map[string]*framework.FieldSchema{
			"username": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `The username of the user.`,
			},
			"description": {
				Type:        framework.TypeString,
				Description: `Optional. Description for the user token.`,
			},
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Optional. Override the maximum TTL for this access token. Cannot exceed smallest (system, mount, backend) maximum TTL.`,
			},
			"ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Optional. Override the default TTL when issuing this access token. Cappaed at the smallest maximum TTL (system, mount, backend, request).`,
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

	userTokenConfig, err := b.fetchUserTokenConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	role := &artifactoryRole{
		GrantType:   "client_credentials",
		Username:    data.Get("username").(string),
		Scope:       "applied-permissions/user",
		MaxTTL:      b.Backend.System().MaxLeaseTTL(),
		Description: userTokenConfig.DefaultDescription,
	}

	if userTokenConfig.MaxTTL != 0 && userTokenConfig.MaxTTL < role.MaxTTL {
		role.MaxTTL = userTokenConfig.MaxTTL
	}

	if value, ok := data.GetOk("max_ttl"); ok {
		value := time.Second * time.Duration(value.(int))
		if value != 0 && value < role.MaxTTL {
			role.MaxTTL = value
		}
	}

	var ttl time.Duration
	if value, ok := data.GetOk("ttl"); ok {
		ttl = time.Second * time.Duration(value.(int))
	} else if userTokenConfig.DefaultTTL != 0 {
		ttl = userTokenConfig.DefaultTTL
	} else {
		ttl = b.Backend.System().DefaultLeaseTTL()
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
		"description":  role.Description,
	}, map[string]interface{}{
		"access_token": resp.AccessToken,
		"token_id":     resp.TokenId,
		"username":     role.Username,
	})

	response.Secret.TTL = ttl
	response.Secret.MaxTTL = role.MaxTTL

	return response, nil
}
