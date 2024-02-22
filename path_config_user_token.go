package artifactory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const configUserTokenPath = "config/user_token"

func (b *backend) pathConfigUserToken() *framework.Path {
	return &framework.Path{
		Pattern: fmt.Sprintf("%s(?:/%s)?", configUserTokenPath, framework.GenericNameWithAtRegex("username")),
		Fields: map[string]*framework.FieldSchema{
			"username": {
				Type:        framework.TypeString,
				Description: `Optional. The username of the user. If not specified, the configuration will apply to *all* users.`,
			},
			"access_token": {
				Type:        framework.TypeString,
				Description: "Optional. User identity token to access Artifactory. If `username` is not set then this token will be used for *all* users.",
			},
			"refresh_token": {
				Type:        framework.TypeString,
				Description: "Optional. Refresh token for the user access token. If `username` is not set then this token will be used for *all* users.",
			},
			"audience": {
				Type:        framework.TypeString,
				Description: `Optional. See the JFrog Artifactory REST documentation on "Create Token" for a full and up to date description.`,
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
			"default_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Optional. Default TTL for issued user access tokens. If unset, uses the backend's default_ttl. Cannot exceed max_ttl.`,
			},
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Optional. Maximum TTL that a user access token can be renewed for. If unset, uses the backend's max_ttl. Cannot exceed backend's max_ttl.`,
			},
			"default_description": {
				Type:        framework.TypeString,
				Description: `Optional. Default token description to set in Artifactory for issued user access tokens.`,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigUserTokenUpdate,
				Summary:  "Configure the Artifactory secrets configuration for user token.",
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigUserTokenRead,
				Summary:  "Examine the Artifactory secrets configuration for user token.",
			},
		},
		HelpSynopsis:    `Configuration for issuing user tokens.`,
		HelpDescription: `Configures default values for the user_token/<user name> path. The optional 'username' field allows the configuration to be set for each username.`,
	}
}

type userTokenConfiguration struct {
	baseConfiguration
	RefreshToken          string        `json:"refresh_token"`
	Audience              string        `json:"audience,omitempty"`
	Refreshable           bool          `json:"refreshable,omitempty"`
	IncludeReferenceToken bool          `json:"include_reference_token,omitempty"`
	DefaultTTL            time.Duration `json:"default_ttl,omitempty"`
	MaxTTL                time.Duration `json:"max_ttl,omitempty"`
	DefaultDescription    string        `json:"default_description,omitempty"`
}

// fetchAdminConfiguration will return nil,nil if there's no configuration
func (b *backend) fetchUserTokenConfiguration(ctx context.Context, storage logical.Storage, username string) (*userTokenConfiguration, error) {
	// If username is not empty, then append to the path to fetch username specific configuration
	path := configUserTokenPath
	if len(username) > 0 && !strings.HasSuffix(path, username) {
		path = fmt.Sprintf("%s/%s", path, username)
	}

	// Read in the backend configuration
	b.Logger().Info("fetching user token configuration", "path", path)
	entry, err := storage.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return &userTokenConfiguration{}, nil
	}

	var config userTokenConfiguration
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (b *backend) storeUserTokenConfiguration(ctx context.Context, req *logical.Request, username string, userTokenConfig *userTokenConfiguration) error {
	// If username is not empty, then append to the path to fetch username specific configuration
	path := configUserTokenPath
	if len(username) > 0 && !strings.HasSuffix(path, username) {
		path = fmt.Sprintf("%s/%s", path, username)
	}

	entry, err := logical.StorageEntryJSON(path, userTokenConfig)
	if err != nil {
		return err
	}

	b.Logger().Info("saving user token configuration", "path", path)
	err = req.Storage.Put(ctx, entry)
	if err != nil {
		return err
	}

	return nil
}

func (b *backend) pathConfigUserTokenUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.configMutex.Lock()
	defer b.configMutex.Unlock()

	adminConfig, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if adminConfig == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	go b.sendUsage(adminConfig.baseConfiguration, "pathConfigUserTokenUpdate")

	username := ""
	if val, ok := data.GetOk("username"); ok {
		username = val.(string)
	}

	userTokenConfig, err := b.fetchUserTokenConfiguration(ctx, req.Storage, username)
	if err != nil {
		return nil, err
	}

	if userTokenConfig.ArtifactoryURL == "" {
		userTokenConfig.ArtifactoryURL = adminConfig.ArtifactoryURL
	}

	if val, ok := data.GetOk("access_token"); ok {
		userTokenConfig.AccessToken = val.(string)
	} else {
		userTokenConfig.AccessToken = adminConfig.AccessToken
	}

	if val, ok := data.GetOk("refresh_token"); ok {
		userTokenConfig.RefreshToken = val.(string)
	}

	if val, ok := data.GetOk("audience"); ok {
		userTokenConfig.Audience = val.(string)
	}

	if val, ok := data.GetOk("refreshable"); ok {
		userTokenConfig.Refreshable = val.(bool)
	}

	if val, ok := data.GetOk("include_reference_token"); ok {
		userTokenConfig.IncludeReferenceToken = val.(bool)
	}

	if val, ok := data.GetOk("use_expiring_tokens"); ok {
		userTokenConfig.UseExpiringTokens = val.(bool)
	} else {
		userTokenConfig.UseExpiringTokens = adminConfig.UseExpiringTokens
	}

	if val, ok := data.GetOk("default_ttl"); ok {
		userTokenConfig.DefaultTTL = time.Duration(val.(int)) * time.Second
	}

	if val, ok := data.GetOk("max_ttl"); ok {
		userTokenConfig.MaxTTL = time.Duration(val.(int)) * time.Second
	}

	if val, ok := data.GetOk("default_description"); ok {
		userTokenConfig.DefaultDescription = val.(string)
	}

	err = b.storeUserTokenConfiguration(ctx, req, username, userTokenConfig)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *backend) pathConfigUserTokenRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()

	adminConfig, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if adminConfig == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	go b.sendUsage(adminConfig.baseConfiguration, "pathConfigUserTokenRead")

	username := ""
	if val, ok := data.GetOk("username"); ok {
		username = val.(string)
	}

	userTokenConfig, err := b.fetchUserTokenConfiguration(ctx, req.Storage, username)
	if err != nil {
		return nil, err
	}

	accessTokenHash := sha256.Sum256([]byte(userTokenConfig.AccessToken))
	refreshTokenHash := sha256.Sum256([]byte(userTokenConfig.RefreshToken))

	configMap := map[string]interface{}{
		"access_token_sha256":     fmt.Sprintf("%x", accessTokenHash[:]),
		"refresh_token_sha256":    fmt.Sprintf("%x", refreshTokenHash[:]),
		"audience":                userTokenConfig.Audience,
		"refreshable":             userTokenConfig.Refreshable,
		"include_reference_token": userTokenConfig.IncludeReferenceToken,
		"use_expiring_tokens":     userTokenConfig.UseExpiringTokens,
		"default_ttl":             userTokenConfig.DefaultTTL.Seconds(),
		"max_ttl":                 userTokenConfig.MaxTTL.Seconds(),
		"default_description":     userTokenConfig.DefaultDescription,
	}

	// Optionally include token info if it parses properly
	token, err := b.getTokenInfo(adminConfig.baseConfiguration, userTokenConfig.AccessToken)
	if err != nil {
		b.Logger().Warn("Error parsing AccessToken", "err", err.Error())
	} else {
		configMap["token_id"] = token.TokenID
		configMap["username"] = token.Username
		configMap["scope"] = token.Scope
		if token.Expires > 0 {
			configMap["exp"] = token.Expires
			configMap["expires"] = time.Unix(token.Expires, 0).Local()
		}
	}

	return &logical.Response{
		Data: configMap,
	}, nil
}
