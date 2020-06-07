package artifactory

import (
	"context"
	"crypto/sha256"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"time"
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
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: "Maximum duration any lease issued by this backend can be.",
				Default:     time.Duration(1 * time.Hour),
			},
			"default_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: "Default TTL when no other TTL is specified",
				Default:     time.Duration(1 * time.Hour),
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigUpdate,
				Summary:  "FIXME",
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathConfigDelete,
				Summary:  "FIXME",
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigRead,
				Summary:  "FIXME",
			},
		},
		HelpSynopsis:    `FIXME`,
		HelpDescription: `FIXME`,
	}
}

type adminConfiguration struct {
	AccessToken    string        `json:"access_token"`
	ArtifactoryURL string        `json:"artifactory_url"`
	MaxTTL         time.Duration `json:"max_ttl"`
	DefaultTTL     time.Duration `json:"default_ttl"`
}

func (b *backend) pathConfigUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {

	b.configMutex.Lock()
	defer b.configMutex.Unlock()

	config := &adminConfiguration{}
	config.AccessToken = data.Get("access_token").(string)
	config.ArtifactoryURL = data.Get("url").(string)
	config.MaxTTL = time.Second * time.Duration(data.Get("max_ttl").(int))
	config.DefaultTTL = time.Second * time.Duration(data.Get("default_ttl").(int))

	if config.AccessToken == "" {
		return logical.ErrorResponse("access_token is required"), nil
	}

	if config.ArtifactoryURL == "" {
		return logical.ErrorResponse("url is required"), nil
	}

	if config.MaxTTL > 0 && config.DefaultTTL > config.MaxTTL {
		return logical.ErrorResponse("default_ttl cannot be longer than max_ttl"), nil
	}

	if b.Backend.System().MaxLeaseTTL() > 0 {
		if config.MaxTTL > b.Backend.System().MaxLeaseTTL() {
			return logical.ErrorResponse("max_ttl exceeds system max_ttl"), nil
		}
		if config.DefaultTTL > b.Backend.System().MaxLeaseTTL() {
			return logical.ErrorResponse("default_ttl exceeds system max_ttl"), nil
		}
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

	return logical.ErrorResponse("FIXME"), nil
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
	accessTokenHash := sha256.Sum224([]byte(config.AccessToken))

	configMap := map[string]interface{}{
		"access_token_sha256": accessTokenHash,
		"url":                 config.ArtifactoryURL,
		"default_ttl":         config.DefaultTTL.Seconds(),
		"max_ttl":             config.MaxTTL.Seconds(),
	}

	return &logical.Response{
		Data: configMap,
	}, nil
}
