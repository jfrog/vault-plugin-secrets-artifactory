package artifactory

import (
	"context"
	"fmt"
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
	var warning string

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

	if ttl == 0 {
		if role.DefaultTTL > 0 {
			ttl = role.DefaultTTL
		} else if config.DefaultTTL > 0 {
			ttl = config.DefaultTTL
		}
	}

	// Check that the max_ttl is not greater than the backend configuration, role, or system-wide max_ttl. This is
	// done at create time since I'm guessing that's the most secure time to do it (ie, if the max_ttl for a backend
	// changes such that it's lower than the max_ttl for the role, we still enforce that here).
	if config.MaxTTL > 0 && maxTTL > config.MaxTTL {
		return logical.ErrorResponse("max_ttl cannot exceed max_ttl for backend"), nil
	}

	if role.MaxTTL > 0 && maxTTL > role.MaxTTL {
		return logical.ErrorResponse("max_ttl cannot exceed max_ttl for role"), nil
	}

	if b.Backend.System().MaxLeaseTTL() > 0 {
		if role.MaxTTL > 0 {
			if b.Backend.System().MaxLeaseTTL() > role.MaxTTL {
				return logical.ErrorResponse("max_ttl cannot exceed system max_ttl"), nil
			}
		} else {
			// If there's a system-max TTL and one wasn't specified, we set maxTTL to that system maximum here.
			maxTTL = b.Backend.System().MaxLeaseTTL()
		}
	}

	if maxTTL > 0 && ttl > maxTTL {
		warning = fmt.Sprintf("ttl (%v) lowered to max_ttl (%v)", ttl, maxTTL)
		ttl = maxTTL
	}

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

	if warning != "" {
		response.AddWarning(warning)
	}

	response.Secret.TTL = ttl
	response.Secret.MaxTTL = maxTTL

	// TODO do I need to fill all this in myself?
	response.Secret.LeaseOptions.Renewable = (resp.RefreshToken != "")
	response.Secret.LeaseOptions.MaxTTL = maxTTL
	response.Secret.LeaseOptions.TTL = ttl
	response.Secret.LeaseOptions.IssueTime = time.Now()

	return response, nil
}
