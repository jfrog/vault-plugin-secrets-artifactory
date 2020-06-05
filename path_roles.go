package artifactory

import (
	"context"
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
		HelpSynopsis:    `FIXME`,
		HelpDescription: `FIXME`,
	}
}

func (b *backend) pathRoles() *framework.Path {
	return &framework.Path{
		Pattern: "roles/" + framework.GenericNameWithAtRegex("role"),
		Fields: map[string]*framework.FieldSchema{
			"role": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `FIXME`,
			},
			"grant_type": {
				Type:        framework.TypeString,
				Description: `FIXME`,
			},
			"username": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `FIXME`,
			},
			"scope": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `FIXME`,
			},
			"refreshable": {
				Type:        framework.TypeBool,
				Required:    false,
				Default:     false,
				Description: `FIXME`,
			},
			"audience": {
				Type:        framework.TypeString,
				Description: `FIXME`,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathRoleRead,
				Summary:  `FIXME`,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRoleWrite,
				Summary:  `FIXME`,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathRoleDelete,
				Summary:  `FIXME`,
			},
		},
		HelpSynopsis:    `FIXME`,
		HelpDescription: `FIXME`,
	}
}

type artifactoryRole struct {
	GrantType   string `json:"grant_type"`
	Username    string `json:"username,omitempty"`
	Scope       string `json:"scope"`
	Refreshable bool   `json:"refreshable"`
	Audience    string `json:"audience,omitempty"`
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
	defer b.rolesMutex.Unlock()

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
