package artifactory

import (
	"context"
	"errors"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathConfigRotate() *framework.Path {
	return &framework.Path{
		Pattern: "config/rotate",
		Fields: map[string]*framework.FieldSchema{
			"username": {
				Type:        framework.TypeString,
				Description: "Optional. Override Artifactory token username for new access token.",
			},
			"description": {
				Type:        framework.TypeString,
				Description: "Optional. Set Artifactory token description on new access token.",
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigRotateWrite,
				Summary:  "Rotate the Artifactory Access Token.",
			},
		},
		HelpSynopsis:    `Rotate the Artifactory Access Token.`,
		HelpDescription: `This will rotate the "access_token" used to access artifactory from this plugin. A new access token is created first then revokes the old access token.`,
	}
}

func (b *backend) pathConfigRotateWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.configMutex.Lock()
	defer b.configMutex.Unlock()

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	if config.AccessToken == "" {
		return logical.ErrorResponse("missing access token"), errors.New("missing access token")
	}

	go b.sendUsage(config.baseConfiguration, "pathConfigRotateWrite")

	oldAccessToken := config.AccessToken

	// Parse Current Token (to get tokenID/scope)
	token, err := b.getTokenInfo(config.baseConfiguration, oldAccessToken)
	if err != nil {
		return logical.ErrorResponse("error parsing existing access token: " + err.Error()), err
	}

	// Check for submitted username
	if val, ok := data.GetOk("username"); ok {
		token.Username = val.(string)
	}

	if len(token.Username) == 0 {
		token.Username = "admin-vault-secrets-artifactory" // default username if empty
	}

	// Create admin role for the new token
	role := &artifactoryRole{
		Username: token.Username,
		Scope:    token.Scope,
	}

	// Check for new description
	if val, ok := data.GetOk("description"); ok {
		role.Description = val.(string)
	} else {
		role.Description = "Rotated access token for artifactory-secrets plugin in Vault"
	}

	// Create a new token
	resp, err := b.CreateToken(config.baseConfiguration, *role)
	if err != nil {
		return logical.ErrorResponse("error creating new access token"), err
	}

	// Set new token and set revoke_on_delete to true
	config.AccessToken = resp.AccessToken
	b.Logger().With("func", "pathConfigRotateWrite").Info("set config.RevokeOnDelete to 'true'")
	config.RevokeOnDelete = true

	// Save new config
	entry, err := logical.StorageEntryJSON(configAdminPath, config)
	if err != nil {
		return nil, err
	}

	err = req.Storage.Put(ctx, entry)
	if err != nil {
		return nil, err
	}

	// Invalidate Old Token
	err = b.RevokeToken(config.baseConfiguration, token.TokenID, oldAccessToken)
	if err != nil {
		return logical.ErrorResponse("error revoking existing access token %s", token.TokenID), err
	}

	return nil, nil
}
