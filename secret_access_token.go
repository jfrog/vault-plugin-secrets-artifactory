package artifactory

import (
	"context"
	"fmt"
	"strings"

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

	role, err := b.Role(ctx, req.Storage, req.Secret.InternalData["role"].(string))
	if err != nil {
		return nil, fmt.Errorf("error during renew: could not get role: %q", req.Secret.InternalData["role"])
	}
	if role == nil {
		return nil, fmt.Errorf("error during renew: could not find role with name: %q", req.Secret.InternalData["role"])
	}

	ttl, warnings, err :=
		framework.CalculateTTL(b.System(), req.Secret.Increment, role.DefaultTTL, 0, role.MaxTTL, req.Secret.MaxTTL, req.Secret.IssueTime)
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

	// logger.Debug("request", "Path", req.Path, "Secret.InternalData", req.Secret.InternalData)

	if config.AccessToken == "" {
		if strings.Contains(req.Path, "token/") {
			return logical.ErrorResponse("admin access_token is not configured"), nil
		}

		// try to use user token
		if strings.Contains(req.Path, "user_token/") {
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
