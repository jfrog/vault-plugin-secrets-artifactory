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
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathTokenCreatePerform,
			},
		},
		HelpSynopsis: `Create an Artifactory access token for the specified role.`,
	}
}

type createTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
}

func (b *backend) pathTokenCreatePerform(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.rolesMutex.RLock()
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()
	defer b.rolesMutex.RUnlock()

	var role artifactoryRole

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	// Read in the requested role
	roleName := data.Get("role").(string)
	entry, err := req.Storage.Get(ctx, "roles/"+roleName)

	if err != nil {
		return nil, err
	}

	if entry == nil {
		return logical.ErrorResponse("no such role"), nil
	}

	if err := entry.DecodeJSON(&role); err != nil {
		return nil, err
	}

	maxTTL := time.Second * time.Duration(data.Get("max_ttl").(int))
	ttl := time.Second * time.Duration(data.Get("ttl").(int))

	maxTTL, ttl, warnings := b.calculateTTLs(config, role, maxTTL, ttl)

	resp, err := b.createToken(*config, role, ttl, maxTTL)
	if err != nil {
		return nil, err
	}

	response := b.Secret(SecretArtifactoryAccessTokenType).Response(map[string]interface{}{
		"access_token": resp.AccessToken,
		"role":         roleName,
		"scope":        resp.Scope,
		"refreshable":  (resp.RefreshToken != ""),
		"expires_in":   resp.ExpiresIn,
	}, map[string]interface{}{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
	})

	if len(warnings) > 0 {
		response.Warnings = warnings
	}

	response.Secret.TTL = ttl
	response.Secret.MaxTTL = 555 * time.Second

	// TODO do I need to fill all this in myself?
	response.Secret.LeaseOptions.Renewable = (resp.RefreshToken != "")
	response.Secret.LeaseOptions.MaxTTL = maxTTL
	response.Secret.LeaseOptions.TTL = ttl
	response.Secret.LeaseOptions.IssueTime = time.Now()

	return response, nil
}

func (b *backend) calculateTTLs(backend *adminConfiguration, role artifactoryRole, requestMaxTTL, requestTTL time.Duration) (time.Duration, time.Duration, []string) {

	var maxTTL, ttl time.Duration
	var warnings []string

	var maxTTLWarning string

	// Figure out the most desirable max_ttl
	if requestMaxTTL > 0 {
		maxTTL = requestMaxTTL
	} else if role.MaxTTL > 0 {
		maxTTL = role.MaxTTL
	} else if backend.MaxTTL > 0 {
		maxTTL = backend.MaxTTL
	} else if b.Backend.System().MaxLeaseTTL() > 0 {
		maxTTL = b.Backend.System().MaxLeaseTTL()
	}

	// Lower it to the lowest reasonable value, I could do some loop here, but I want a nice
	// warning message.
	if b.Backend.System().MaxLeaseTTL() > 0 && maxTTL > b.Backend.System().MaxLeaseTTL() {
		maxTTLWarning = "max_ttl lowered to system max_ttl"
		maxTTL = b.Backend.System().MaxLeaseTTL()
	}

	if backend.MaxTTL > 0 && maxTTL > backend.MaxTTL {
		maxTTLWarning = "max_ttl lowered to backend max_ttl"
		maxTTL = backend.MaxTTL
	}

	if role.MaxTTL > 0 && maxTTL > role.MaxTTL {
		maxTTLWarning = "max_ttl lowered to role max_ttl"
		maxTTL = role.MaxTTL
	}

	// I don't think this is actually possible.
	if requestMaxTTL > 0 && maxTTL > requestMaxTTL {
		maxTTL = requestMaxTTL
	}

	if maxTTLWarning != "" {
		warnings = append(warnings, maxTTLWarning)
	}

	// Figure out the most desirable default_ttl
	if requestTTL > 0 {
		ttl = requestTTL
	} else if role.DefaultTTL > 0 {
		ttl = role.DefaultTTL
	} else if backend.DefaultTTL > 0 {
		ttl = backend.DefaultTTL
	} else if b.Backend.System().DefaultLeaseTTL() > 0 {
		ttl = b.Backend.System().DefaultLeaseTTL()
	}

	if ttl > maxTTL {
		warnings = append(warnings, "ttl lowered to max_ttl")
		ttl = maxTTL

	}

	return maxTTL, ttl, warnings
}
