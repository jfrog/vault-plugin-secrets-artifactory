package artifactory

import (
	"context"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathTokenCreate() *framework.Path {
	return &framework.Path{
		Pattern: "token/" + framework.GenericNameWithAtRegex("role"),
		Fields: map[string]*framework.FieldSchema{
			"role": {
				Type:        framework.TypeString,
				Description: `Use the configuration of the specified role.`,
			},
			"ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Override the default TTL when issuing this access token. Cannot exceed smallest (system, backend, role, this request) maximum TTL.`,
			},
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Override the maximum TTL for this access token. Cannot exceed smallest (system, backend) maximum TTL.`,
			},
			"scope": {
				Type:        framework.TypeString,
				Description: `Override the scope for this access token.`,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathTokenCreatePerform,
			},
		},
		HelpSynopsis: `Create an Artifactory access token for the specified role.`,
		HelpDescription: `
Create an Artifactory access token using paramters from the specified role.

An optional 'ttl' parameter will override the role's 'default_ttl' parameter.

An optional 'max_ttl' parameter will override the role's 'max_ttl' parameter.
`,
	}
}

type systemVersionResponse struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

type createTokenResponse struct {
	TokenId        string `json:"token_id"`
	AccessToken    string `json:"access_token"`
	RefreshToken   string `json:"refresh_token"`
	ExpiresIn      int    `json:"expires_in"`
	Scope          string `json:"scope"`
	TokenType      string `json:"token_type"`
	ReferenceToken string `json:"reference_token"`
}

func (b *backend) pathTokenCreatePerform(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
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

	go b.sendUsage(config.baseConfiguration, "pathTokenCreatePerform")

	// Read in the requested role
	roleName := data.Get("role").(string)

	role, err := b.Role(ctx, req.Storage, roleName)
	if err != nil {
		return nil, err
	}

	if role == nil {
		return logical.ErrorResponse("no such role: %s", roleName), nil
	}

	// Define username for token by template if a static one is not set
	if len(role.Username) == 0 {
		role.Username, err = b.usernameProducer.Generate(UsernameMetadata{
			RoleName:    roleName,
			DisplayName: req.DisplayName,
		})
		if err != nil {
			return logical.ErrorResponse("error generating username from template"), err
		}
	}

	maxLeaseTTL := b.Backend.System().MaxLeaseTTL()
	b.Logger().Debug("initialize maxLeaseTTL to system value", "maxLeaseTTL", maxLeaseTTL)

	if value, ok := data.GetOk("max_ttl"); ok && value.(int) > 0 {
		b.Logger().Debug("max_ttl is set", "max_ttl", value)
		maxTTL := time.Second * time.Duration(value.(int))

		// use override max TTL if set and less than maxLeaseTTL
		if maxTTL > 0 || maxTTL < maxLeaseTTL {
			maxLeaseTTL = maxTTL
		}
	} else if role.MaxTTL > 0 && role.MaxTTL < maxLeaseTTL {
		b.Logger().Debug("using role MaxTTL", "role.MaxTTL", role.MaxTTL)
		maxLeaseTTL = role.MaxTTL
	}
	b.Logger().Debug("Max lease TTL (sec)", "maxLeaseTTL", maxLeaseTTL)

	ttl := b.Backend.System().DefaultLeaseTTL()
	if value, ok := data.GetOk("ttl"); ok && value.(int) > 0 {
		b.Logger().Debug("ttl is set", "ttl", value)
		ttl = time.Second * time.Duration(value.(int))
	} else if role.DefaultTTL != 0 {
		b.Logger().Debug("using role DefaultTTL", "role.DefaultTTL", role.DefaultTTL)
		ttl = role.DefaultTTL
	}

	// cap ttl to maxLeaseTTL
	if ttl > maxLeaseTTL {
		b.Logger().Debug("ttl is longer than maxLeaseTTL", "ttl", ttl, "maxLeaseTTL", maxLeaseTTL)
		ttl = maxLeaseTTL
	}
	b.Logger().Debug("TTL (sec)", "ttl", ttl)

	// Set the role.ExpiresIn based on maxLeaseTTL if use_expiring_tokens is set to tru in config
	// - This value will be passed to createToken and used as expires_in for versions of Artifactory 7.50.3 or higher
	if config.UseExpiringTokens {
		role.ExpiresIn = maxLeaseTTL
	}

	if config.AllowScopedTokens {
	  scope := data.Get("scope").(string)
	  if len(scope) != 0 {
	    match, _ := regexp.MatchString(`^applied-permissions/groups:.+$`, scope)
	    if !match {
	      return logical.ErrorResponse("provided scope is invalid"), errors.New("provided scope is invalid")
	    }
	    //use the overridden scope rather than role default
	    role.Scope = scope
	  }
	}

	resp, err := b.CreateToken(config.baseConfiguration, *role)
	if err != nil {
		return nil, err
	}

	response := b.Secret(SecretArtifactoryAccessTokenType).Response(map[string]interface{}{
		"access_token":    resp.AccessToken,
		"refresh_token":   resp.RefreshToken,
		"role":            roleName,
		"expires_in":      resp.ExpiresIn,
		"scope":           resp.Scope,
		"token_id":        resp.TokenId,
		"username":        role.Username,
		"reference_token": resp.ReferenceToken,
	}, map[string]interface{}{
		"role":            roleName,
		"access_token":    resp.AccessToken,
		"refresh_token":   resp.RefreshToken,
		"expires_in":      resp.ExpiresIn,
		"scope":           resp.Scope,
		"token_id":        resp.TokenId,
		"username":        role.Username,
		"reference_token": resp.ReferenceToken,
	})

	response.Secret.TTL = ttl
	response.Secret.MaxTTL = maxLeaseTTL

	return response, nil
}
