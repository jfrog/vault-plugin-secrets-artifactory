package artifactory

import (
	"context"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathListRoles() *framework.Path {
	return &framework.Path{
		Pattern: "roles/?$",
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathRoleList,
			},
		},
		HelpSynopsis: `List configured roles with this backend.`,
	}
}

func (b *backend) pathRoles() *framework.Path {
	return &framework.Path{
		Pattern: "roles/" + framework.GenericNameWithAtRegex("role"),
		Fields: map[string]*framework.FieldSchema{
			"role": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `The name of the role, must be conform to alphanumeric plus at, dash, and period.`,
			},
			"grant_type": {
				Type:        framework.TypeString,
				Description: `Optional. Defaults to 'client_credentials' when creating the access token. You likely don't need to change this.'`,
			},
			"username": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `Required. The username for which the access token is created. If the user does not exist, Artifactory will create a transient user. Note that non-admininstrative access tokens can only create tokens for themselves.`,
			},
			"scope": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `Required. See the JFrog Artifactory REST documentation on "Create Token" for a full and up to date description.`,
			},
			"refreshable": {
				Type:        framework.TypeBool,
				Required:    false,
				Default:     false,
				Description: `Optional; defaults to false. Will tell Artifactory that the token should be refreshable.`,
			},
			"audience": {
				Type:        framework.TypeString,
				Description: `Optional. See the JFrog Artifactory REST documentation on "Create Token" for a full and up to date description.`,
			},
			"default_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Default TTL for issued access tokens. If unset, uses the backend's default_ttl. Cannot exceed max_ttl.`,
			},
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: `Maximum TTL that an access token can be renewed for. If unset, uses the backend's max_ttl. Cannot exceed backend's max_ttl.`,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathRoleRead,
				Summary:  `Read information about the specified role.`,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRoleWrite,
				Summary:  `Write or overwrite information about the specified role.`,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathRoleDelete,
				Summary:  `Delete the specified role.`,
			},
		},
		HelpSynopsis: `Manage data related to roles used to issue Artifactory access tokens.`,
	}
}

type artifactoryRole struct {
	GrantType   string        `json:"grant_type"`
	Username    string        `json:"username,omitempty"`
	Scope       string        `json:"scope"`
	Refreshable bool          `json:"refreshable"`
	Audience    string        `json:"audience,omitempty"`
	DefaultTTL  time.Duration `json:"default_ttl,omitempty"`
	MaxTTL      time.Duration `json:"max_ttl,omitempty"`
}

func (b *backend) pathRoleList(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	b.rolesMutex.RLock()
	defer b.rolesMutex.RUnlock()

	entries, err := req.Storage.List(ctx, "roles/")
	if err != nil {
		return nil, err
	}

	if entries == nil {
		return logical.ErrorResponse("no roles found"), nil
	}

	return logical.ListResponse(entries), nil
}

func (b *backend) pathRoleWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.rolesMutex.Lock()
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()
	defer b.rolesMutex.Unlock()

	config, err := b.fetchAdminConfiguration(ctx, req.Storage)

	if err != nil {
		return nil, err
	}

	if config == nil {
		return logical.ErrorResponse("backend not configured"), nil
	}

	roleName := data.Get("role").(string)

	if roleName == "" {
		return logical.ErrorResponse("missing role"), nil
	}

	newRole := artifactoryRole{
		GrantType:   data.Get("grant_type").(string),
		Username:    data.Get("username").(string),
		Scope:       data.Get("scope").(string),
		Refreshable: data.Get("refreshable").(bool),
		Audience:    data.Get("audience").(string),
		DefaultTTL:  time.Second * time.Duration(data.Get("default_ttl").(int)),
		MaxTTL:      time.Second * time.Duration(data.Get("max_ttl").(int)),
	}

	if newRole.MaxTTL > 0 && newRole.DefaultTTL > newRole.MaxTTL {
		return logical.ErrorResponse("default_ttl cannot be longer than max_ttl"), nil
	}

	if config.MaxTTL > 0 && newRole.MaxTTL > config.MaxTTL {
		return logical.ErrorResponse("role max_ttl cannot be longer than backend max_ttl"), nil
	}

	if config.MaxTTL > 0 && newRole.DefaultTTL > config.MaxTTL {
		return logical.ErrorResponse("role default_ttl cannot be longer than backend max_ttl"), nil
	}

	if newRole.Scope == "" {
		return logical.ErrorResponse("missing scope"), nil
	}

	if newRole.Username == "" {
		return logical.ErrorResponse("missing username"), nil
	}

	entry, err := logical.StorageEntryJSON("roles/"+roleName, newRole)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *backend) pathRoleRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.rolesMutex.RLock()
	defer b.rolesMutex.RUnlock()

	roleName := data.Get("role").(string)

	if roleName == "" {
		return logical.ErrorResponse("missing role"), nil
	}

	entry, err := req.Storage.Get(ctx, "roles/"+roleName)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return logical.ErrorResponse("no such role"), nil
	}

	var role artifactoryRole

	if err := entry.DecodeJSON(&role); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: b.roleToMap(roleName, role),
	}, nil
}

func (b *backend) roleToMap(roleName string, role artifactoryRole) map[string]interface{} {
	return map[string]interface{}{
		"role":        roleName,
		"grant_type":  role.GrantType,
		"username":    role.Username,
		"scope":       role.Scope,
		"refreshable": role.Refreshable,
		"audience":    role.Audience,
		"default_ttl": role.DefaultTTL.Seconds(),
		"max_ttl":     role.MaxTTL.Seconds(),
	}
}

func (b *backend) pathRoleDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.rolesMutex.Lock()
	defer b.rolesMutex.Unlock()

	err := req.Storage.Delete(ctx, "roles/"+data.Get("role").(string))
	if err != nil {
		return nil, err
	}

	return nil, nil
}
