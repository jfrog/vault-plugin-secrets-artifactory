package artifactory

import (
	"context"
	"fmt"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"time"
)

func (b *backend) pathTokenCreate() *framework.Path {
	return &framework.Path{
		Pattern: "token/" + framework.GenericNameWithAtRegex("role"),
		Fields: map[string]*framework.FieldSchema{
			"role": {
				Type:        framework.TypeString,
				Description: "FIXME",
			},
			"ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `FIXME`,
			},
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `FIXME`,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathTokenCreatePerform,
			},
		},
		HelpSynopsis:    `FIXME`,
		HelpDescription: `FIXME`,
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
	TTL := time.Second * time.Duration(data.Get("ttl").(int))

	if TTL == 0 {
		if role.DefaultTTL > 0 {
			TTL = role.DefaultTTL
		} else if config.DefaultTTL > 0 {
			TTL = config.DefaultTTL
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

	if maxTTL > 0 && TTL > maxTTL {
		return logical.ErrorResponse("ttl cannot exceed max_ttl"), nil
	}

	fmt.Printf("maxttl: %v ttl: %v\n", maxTTL, TTL)

	resp, err := b.createToken(*config, role, TTL, maxTTL)
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

	response.Secret.TTL = TTL
	response.Secret.MaxTTL = maxTTL

	// TODO do I need to fill all this in myself?
	response.Secret.LeaseOptions.Renewable = (resp.RefreshToken != "")
	response.Secret.LeaseOptions.MaxTTL = maxTTL
	response.Secret.LeaseOptions.TTL = TTL
	response.Secret.LeaseOptions.IssueTime = time.Now()

	return response, nil
}
