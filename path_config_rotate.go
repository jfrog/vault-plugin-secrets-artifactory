package artifactory

import (
	"context"

	"github.com/golang-jwt/jwt"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathConfigRotate() *framework.Path {
	return &framework.Path{
		Pattern: "config/rotate",
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigRotateWrite,
				Summary:  "Rotate the Artifactory Admin Token.",
			},
		},
		HelpSynopsis: `Rotate the Artifactory Admin Token.`,
		HelpDescription: `
This will rotate the "access_token" used to access artifactory from this plugin, and remove the old token.
`,
	}
}

func (b *backend) pathConfigRotateWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.configMutex.Lock()
	defer b.configMutex.Unlock()
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

	// Parse Current Token
	// -- NOTE THIS IGNORES THE SIGNATURE, which is probably bad,
	//    but it is not our job to validate the token.
	token, err := jwt.Parse(config.AccessToken, nil)
	if err != nil {
		return logical.ErrorResponse("error parsing existing AccessToken"), nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return logical.ErrorResponse("error parsing claims in existing AccessToken"), nil
	}
	oldTokenID := claims["jti"] // jti -> JFrog Token ID?

	// Create admin role for the new token
	role := &artifactoryRole{
		Username: "admin",
		Scope:    "applied-permissions/admin",
	}

	// Create a new token
	resp, err := b.createToken(*config, *role)
	if err != nil {
		return nil, err
	}

	// Set new token
	config.AccessToken = resp.AccessToken

	// Invalidate Old Token (TODO)
	err = b.revokeToken(*config, oldTokenID.(logical.Secret))
	if err != nil {
		return logical.ErrorResponse("error revoking existing AccessToken"), nil
	}

	// Save new config
	entry, err := logical.StorageEntryJSON("config/admin", config)
	if err != nil {
		return nil, err
	}

	err = req.Storage.Put(ctx, entry)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
