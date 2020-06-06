package artifactory

import (
	"context"
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
				Description: `FIXME`,
			},
		},

		Renew:  b.secretAccessTokenRenew,
		Revoke: b.secretAccessTokenRevoke,
	}
}

func (b *backend) secretAccessTokenRenew(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	resp := &logical.Response{Secret: req.Secret}

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	b.Backend.Logger().Warn("renewing is complicated", "lease options", req.Secret.LeaseOptions,
		"Max TTL", req.Secret.MaxTTL,
		"TTL", req.Secret.TTL)
	accessToken := req.Secret.InternalData["access_token"].(string)
	refreshToken := req.Secret.InternalData["refresh_token"].(string)
	if refreshToken == "" {
		return logical.ErrorResponse("token can not be refreshed"), nil
	}

	if _, err := b.refreshToken(*config, accessToken, refreshToken); err != nil {
		return nil, err
	}

	// FIXME how do I handle TTLs here?

	return resp, nil
}

func (b *backend) secretAccessTokenRevoke(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	if err := b.revokeToken(*config, *req.Secret); err != nil {
		return nil, err
	}

	return nil, nil
}
