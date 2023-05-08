package artifactory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathConfig() *framework.Path {
	return &framework.Path{
		Pattern: "config/admin",
		Fields: map[string]*framework.FieldSchema{
			"access_token": {
				Type:        framework.TypeString,
				Required:    true,
				Description: "Administrator token to access Artifactory",
			},
			"url": {
				Type:        framework.TypeString,
				Required:    true,
				Description: "Address of the Artifactory instance",
			},
			"username_template": {
				Type:        framework.TypeString,
				Description: "Optional. Vault Username Template for dynamically generating usernames.",
			},
			"use_expiring_tokens": {
				Type:        framework.TypeString,
				Description: "Optional. If Artifactory version >= 7.50.3, set expires_in to max_ttl and force_revocable.",
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigUpdate,
				Summary:  "Configure the Artifactory secrets backend.",
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathConfigDelete,
				Summary:  "Delete the Artifactory secrets configuration.",
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigRead,
				Summary:  "Examine the Artifactory secrets configuration.",
			},
		},
		HelpSynopsis: `Interact with the Artifactory secrets configuration.`,
		HelpDescription: `
Configure the parameters used to connect to the Artifactory server integrated with this backend. The two main
parameters are "url" which is the absolute URL to the Artifactory server. Note that "/api" is prepended by the
individual calls, so do not include it in the URL here.

The second is "access_token" which must be an access token powerful enough to generate the other access tokens you'll
be using. This value is stored seal wrapped when available. Once set, the access token cannot be retrieved, but the backend
will send a sha256 hash of the token so you can compare it to your notes. If the token is a JWT Access Token, it will return
additional information such as jfrog_token_id, username and scope.

An optional "username_template" parameter will override the built-in default username_template for dynamically generating
usernames if a static one is not provided.

No renewals or new tokens will be issued if the backend configuration (config/admin) is deleted.
`,
	}
}

type adminConfiguration struct {
	AccessToken       string `json:"access_token"`
	ArtifactoryURL    string `json:"artifactory_url"`
	UsernameTemplate  string `json:"username_template,omitempty"`
	UseExpiringTokens bool   `json:"use_expiring_tokens,omitempty"`
}

func (b *backend) pathConfigUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.configMutex.Lock()
	defer b.configMutex.Unlock()

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = &adminConfiguration{}
	}

	if val, ok := data.GetOk("url"); ok {
		config.ArtifactoryURL = val.(string)
		config.AccessToken = "" // clear access token if URL changes, requires setting access_token and url together for security reasons
	}
	if val, ok := data.GetOk("access_token"); ok {
		config.AccessToken = val.(string)
	}
	if val, ok := data.GetOk("username_template"); ok {
		config.UsernameTemplate = val.(string)
		up, err := testUsernameTemplate(config.UsernameTemplate)
		if err != nil {
			return logical.ErrorResponse("username_template error"), err
		}
		b.usernameProducer = up
	}

	if val, ok := data.GetOk("use_expiring_tokens"); ok {
		switch exp := val.(type) {
		case bool:
			config.UseExpiringTokens = exp
		case string:
			config.UseExpiringTokens, err = strconv.ParseBool(exp)
			if err != nil {
				return logical.ErrorResponse("error parsing use_expired_tokens string to bool"), err
			}
		}
	}

	if config.AccessToken == "" {
		return logical.ErrorResponse("access_token is required"), nil
	}

	if config.ArtifactoryURL == "" {
		return logical.ErrorResponse("url is required"), nil
	}

	go b.sendUsage(*config, "pathConfigRotateUpdate")

	err = b.getVersion(*config)
	if err != nil {
		return logical.ErrorResponse("Unable to get Artifactory Version, check url and access_token."), err
	}

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

func (b *backend) pathConfigDelete(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	b.configMutex.Lock()
	defer b.configMutex.Unlock()

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	go b.sendUsage(*config, "pathConfigDelete")

	if err := req.Storage.Delete(ctx, "config/admin"); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *backend) pathConfigRead(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	go b.sendUsage(*config, "pathConfigRead")

	// I'm not sure if I should be returning the access token, so I'll hash it.
	accessTokenHash := sha256.Sum256([]byte(config.AccessToken))

	configMap := map[string]interface{}{
		"access_token_sha256": fmt.Sprintf("%x", accessTokenHash[:]),
		"url":                 config.ArtifactoryURL,
		"version":             b.version,
	}

	// Optionally include username_template
	if len(config.UsernameTemplate) > 0 {
		configMap["username_template"] = config.UsernameTemplate
	}

	// Optionally include token info if it parses properly
	token, err := b.getTokenInfo(*config, config.AccessToken)
	if err != nil {
		b.Logger().Warn("Error parsing AccessToken: " + err.Error())
	} else {
		configMap["token_id"] = token.TokenID
		configMap["username"] = token.Username
		configMap["scope"] = token.Scope
		if token.Expires > 0 {
			configMap["exp"] = token.Expires
			tm := time.Unix(token.Expires, 0)
			configMap["expires"] = tm.Local()
		}
	}

	if b.supportForceRevocable() {
		configMap["use_expiring_tokens"] = config.UseExpiringTokens
	}

	return &logical.Response{
		Data: configMap,
	}, nil
}
