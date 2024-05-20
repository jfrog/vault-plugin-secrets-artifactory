package artifactory

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const createUserTokenPath = "user_token/"

func (b *backend) pathUserTokenCreate() *framework.Path {
	return &framework.Path{
		Pattern: createUserTokenPath + framework.GenericNameWithAtRegex("username"),
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
			"refreshable": {
				Type:        framework.TypeBool,
				Default:     false,
				Description: `Optional. Defaults to 'false'. A refreshable access token gets replaced by a new access token, which is not what a consumer of tokens from this backend would be expecting; instead they'd likely just request a new token periodically. Set this to 'true' only if your usage requires this. See the JFrog Artifactory documentation on "Generating Refreshable Tokens" (https://jfrog.com/help/r/jfrog-platform-administration-documentation/generating-refreshable-tokens) for a full and up to date description.`,
			},
			"include_reference_token": {
				Type:        framework.TypeBool,
				Default:     false,
				Description: `Optional. Defaults to 'false'. Generate a Reference Token (alias to Access Token) in addition to the full token (available from Artifactory 7.38.10). A reference token is a shorter, 64-character string, which can be used as a bearer token, a password, or with the ״X-JFrog-Art-Api״ header. Note: Using the reference token might have performance implications over a full length token.`,
			},
			"use_expiring_tokens": {
				Type:        framework.TypeBool,
				Default:     false,
				Description: "Optional. If Artifactory version >= 7.50.3, set expires_in to max_ttl and force_revocable.",
			},
			"force_revocable": {
				Type:        framework.TypeBool,
				Default:     false,
				Description: "Optional. When set to true, we will add the 'force_revocable' flag to the token's extension. In addition, a new configuration has been added that sets the default for setting the 'force_revocable' default when creating a new token - the default of this configuration will be 'false' to ensure that the Circle of Trust remains in place.",
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
		HelpSynopsis:    `Create an Artifactory access token for the specified user.`,
		HelpDescription: `Provides optional parameters to override default values for the user_token/<user name> path`,
	}
}

func (b *backend) pathUserTokenCreatePerform(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()

	baseConfig := baseConfiguration{}

	adminConfig, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if adminConfig == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	go b.sendUsage(adminConfig.baseConfiguration, "pathUserTokenCreatePerform")

	baseConfig = adminConfig.baseConfiguration

	username := data.Get("username").(string)

	userTokenConfig, err := b.fetchUserTokenConfiguration(ctx, req.Storage, username)
	if err != nil {
		return nil, err
	}

	if userTokenConfig.baseConfiguration.AccessToken != "" {
		baseConfig.AccessToken = userTokenConfig.baseConfiguration.AccessToken
	}

	baseConfig.UseExpiringTokens = userTokenConfig.UseExpiringTokens
	if value, ok := data.GetOk("use_expiring_tokens"); ok {
		baseConfig.UseExpiringTokens = value.(bool)
	}
	if value, ok := data.GetOk("force_revocable"); ok {
		temp := value.(bool)
		baseConfig.ForceRevocable = &temp
	}

	role := artifactoryRole{
		GrantType:             grantTypeClientCredentials,
		Username:              username,
		Scope:                 "applied-permissions/user",
		Audience:              userTokenConfig.Audience,
		Refreshable:           userTokenConfig.Refreshable,
		IncludeReferenceToken: userTokenConfig.IncludeReferenceToken,
		Description:           userTokenConfig.DefaultDescription,
		RefreshToken:          userTokenConfig.RefreshToken,
	}

	logger := b.Logger().With("func", "pathUserTokenCreatePerform")

	maxLeaseTTL := b.Backend.System().MaxLeaseTTL()
	logger.Debug("initialize maxLeaseTTL to system value", "maxLeaseTTL", maxLeaseTTL)

	if value, ok := data.GetOk("max_ttl"); ok && value.(int) > 0 {
		logger.Debug("max_ttl is set", "max_ttl", value)
		maxTTL := time.Second * time.Duration(value.(int))

		// use override max TTL if set and is less than maxLeaseTTL
		if maxTTL != 0 && maxTTL < maxLeaseTTL {
			maxLeaseTTL = maxTTL
		}
	} else if userTokenConfig.MaxTTL > 0 && userTokenConfig.MaxTTL < maxLeaseTTL {
		logger.Debug("using user token config MaxTTL", "userTokenConfig.MaxTTL", userTokenConfig.MaxTTL)
		// use max TTL from user config if set and is less than system max lease TTL
		maxLeaseTTL = userTokenConfig.MaxTTL
	}
	logger.Debug("Max lease TTL (sec)", "maxLeaseTTL", maxLeaseTTL)

	ttl := b.Backend.System().DefaultLeaseTTL()
	if value, ok := data.GetOk("ttl"); ok && value.(int) > 0 {
		logger.Debug("ttl is set", "ttl", value)
		ttl = time.Second * time.Duration(value.(int))
	} else if userTokenConfig.DefaultTTL != 0 {
		logger.Debug("using user config DefaultTTL", "userTokenConfig.DefaultTTL", userTokenConfig.DefaultTTL)
		ttl = userTokenConfig.DefaultTTL
	}

	// cap ttl to maxLeaseTTL
	if maxLeaseTTL > 0 && ttl > maxLeaseTTL {
		logger.Debug("ttl is longer than maxLeaseTTL", "ttl", ttl, "maxLeaseTTL", maxLeaseTTL)
		ttl = maxLeaseTTL
	}
	logger.Debug("TTL (sec)", "ttl", ttl)

	// now ttl is determined, we set role.ExpiresIn so this value so expirable token has the correct expiration
	if baseConfig.UseExpiringTokens {
		role.ExpiresIn = ttl
	}

	if value, ok := data.GetOk("refreshable"); ok {
		role.Refreshable = value.(bool)
	}

	if value, ok := data.GetOk("audience"); ok {
		role.Audience = value.(string)
	}

	if value, ok := data.GetOk("include_reference_token"); ok {
		role.IncludeReferenceToken = value.(bool)
	}

	if value, ok := data.GetOk("description"); ok {
		role.Description = value.(string)
	}

	resp, err := b.CreateToken(baseConfig, role)
	if err != nil {
		if _, ok := err.(*TokenExpiredError); ok {
			logger.Info("access token expired. Attempt to refresh using the refresh token.")
			refreshResp, err := b.RefreshToken(baseConfig, role)
			if err != nil {
				return nil, fmt.Errorf("failed to refresh access token. err: %v", err)
			}
			logger.Info("access token refresh successful")

			userTokenConfig.AccessToken = refreshResp.AccessToken
			userTokenConfig.RefreshToken = refreshResp.RefreshToken
			b.storeUserTokenConfiguration(ctx, req, username, userTokenConfig)

			baseConfig.AccessToken = userTokenConfig.AccessToken
			role.RefreshToken = userTokenConfig.RefreshToken

			// try again after token was refreshed
			logger.Info("attempt to create user token again after access token refresh")
			resp, err = b.CreateToken(baseConfig, role)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	response := b.Secret(SecretArtifactoryAccessTokenType).Response(map[string]interface{}{
		"access_token":    resp.AccessToken,
		"refresh_token":   resp.RefreshToken,
		"expires_in":      resp.ExpiresIn,
		"scope":           resp.Scope,
		"token_id":        resp.TokenId,
		"username":        role.Username,
		"description":     role.Description,
		"reference_token": resp.ReferenceToken,
	}, map[string]interface{}{
		"access_token":    resp.AccessToken,
		"refresh_token":   resp.RefreshToken,
		"expires_in":      resp.ExpiresIn,
		"scope":           resp.Scope,
		"token_id":        resp.TokenId,
		"username":        role.Username,
		"reference_token": resp.ReferenceToken,
	})

	response.Secret.TTL = ttl
	response.Secret.MaxTTL = maxLeaseTTL

	return response, nil
}
