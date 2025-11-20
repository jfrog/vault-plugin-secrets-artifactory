package artifactory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const SecretArtifactoryAccessTokenType = "artifactory_access_token"

func (b *backend) secretAccessToken() *framework.Secret {
	return &framework.Secret{
		Type: SecretArtifactoryAccessTokenType,
		Fields: map[string]*framework.FieldSchema{
			"access_token": {
				Type:        framework.TypeString,
				Description: `Artifactory Access Token`,
			},
			"refresh_token": {
				Type:        framework.TypeString,
				Description: `Artifactory Refresh Token`,
			},
			"reference_token": {
				Type:        framework.TypeString,
				Description: `Artifactory Reference Token`,
			},
			"token_id": {
				Type:        framework.TypeString,
				Description: `Artifactory Access Token Id`,
			},
			"username": {
				Type:        framework.TypeString,
				Description: `Artifactory Username for Token ID`,
			},
		},

		Renew:  b.secretAccessTokenRenew,
		Revoke: b.secretAccessTokenRevoke,
	}
}

func (b *backend) secretAccessTokenRenew(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	if !req.Secret.Renewable {
		return nil, fmt.Errorf("lease cannot be renewed")
	}

	var defaultTTL time.Duration
	var maxTTL time.Duration

	if rawRole, ok := req.Secret.InternalData["role"]; ok {
		// Role backed token
		role, err := b.Role(ctx, req.Storage, rawRole.(string))
		if err != nil {
			return nil, fmt.Errorf("error during renew: could not get role: %q", rawRole)
		}
		if role == nil {
			return nil, fmt.Errorf("error during renew: could not find role with name: %q", rawRole)
		}
		defaultTTL = role.DefaultTTL
		maxTTL = role.MaxTTL
	} else if rawUsername, ok := req.Secret.InternalData["username"]; ok {
		// User backed token
		userTokenConfig, err := b.fetchUserTokenConfiguration(ctx, req.Storage, rawUsername.(string))
		if err != nil {
			return nil, err
		}
		defaultTTL = userTokenConfig.DefaultTTL
		maxTTL = userTokenConfig.MaxTTL
	} else {
		return nil, fmt.Errorf("error during renew: token has got no role nor username")
	}

	ttl, warnings, err :=
		framework.CalculateTTL(b.System(), req.Secret.Increment, defaultTTL, 0, maxTTL, req.Secret.MaxTTL, req.Secret.IssueTime)
	if err != nil {
		return nil, err
	}

	resp := &logical.Response{Secret: req.Secret}

	if len(warnings) > 0 {
		for _, warning := range warnings {
			resp.AddWarning(warning)
		}
	}

	resp.Secret.TTL = ttl

	return resp, nil
}

func (b *backend) secretAccessTokenRevoke(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	logger := b.Logger().With("func", "secretAccessTokenRevoke")

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		logger.Debug("failed to fetch admin config", "err", err)
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	// logger.Debug("req", "Path", req.Path, "Secret.InternalData", req.Secret.InternalData)

	if config.AccessToken == "" {
		// check if this is admin token
		if strings.HasPrefix(req.Path, "token/") {
			return logical.ErrorResponse("admin access_token is not configured"), nil
		}

		// try to use user token
		if strings.HasPrefix(req.Path, "user_token/") {
			logger.Debug("admin access token is empty and request path is user_token")
			username := req.Secret.InternalData["username"].(string)
			userTokenConfig, err := b.fetchUserTokenConfiguration(ctx, req.Storage, username)
			if err != nil {
				logger.Debug("failed to fetch user config", "err", err)
				return nil, err
			}

			if userTokenConfig.AccessToken == "" {
				return logical.ErrorResponse("user access_token is not configured"), nil
			}

			config.AccessToken = userTokenConfig.AccessToken
		}
	}

	tokenId := req.Secret.InternalData["token_id"].(string)

	if err := b.RevokeToken(config.baseConfiguration, tokenId); err != nil {
		return logical.ErrorResponse("failed to revoke access token"), err
	}

	return nil, nil
}
