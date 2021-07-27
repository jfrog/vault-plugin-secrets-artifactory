package artifactory

import (
	"context"
	"fmt"

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
			"token_id": {
				Type:        framework.TypeString,
				Description: `Artifactory Access Token Id`,
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
	if len(warnings) > 0 {
		for _, warning := range warnings {
			resp.AddWarning(warning)
		}
	}

	resp.Secret.TTL = ttl

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
