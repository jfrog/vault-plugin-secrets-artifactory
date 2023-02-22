package artifactory

import (
	"context"
	"crypto/sha256"
	"fmt"

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
additional informaiton such as jfrog_token_id, username and scope.

No renewals or new tokens will be issued if the backend configuration (config/admin) is deleted.
`,
	}
}

type adminConfiguration struct {
	AccessToken    string `json:"access_token"`
	ArtifactoryURL string `json:"artifactory_url"`
}

func (b *backend) pathConfigUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.configMutex.Lock()
	defer b.configMutex.Unlock()

	config := &adminConfiguration{}
	config.AccessToken = data.Get("access_token").(string)
	config.ArtifactoryURL = data.Get("url").(string)

	if config.AccessToken == "" {
		return logical.ErrorResponse("access_token is required"), nil
	}

	if config.ArtifactoryURL == "" {
		return logical.ErrorResponse("url is required"), nil
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

	// I'm not sure if I should be returning the access token, so I'll hash it.
	accessTokenHash := sha256.Sum256([]byte(config.AccessToken))

	configMap := map[string]interface{}{
		"access_token_sha256": fmt.Sprintf("%x", accessTokenHash[:]),
		"url":                 config.ArtifactoryURL,
	}

	// Optionally include token info if it parses properly
	token, err := b.getAdminTokenInfo(*config)
	if err != nil {
		b.Logger().Warn("Error parsing AccessToken: " + err.Error())
	} else {
		configMap["jfrog_token_id"] = token["TokenID"]
		configMap["username"] = token["Username"]
		configMap["scope"] = token["Scope"]
	}

	return &logical.Response{
		Data: configMap,
	}, nil
}
