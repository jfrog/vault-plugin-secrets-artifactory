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
				Description: `Optional. Defaults to using the username_template. The static username for which the access token is created. If the user does not exist, Artifactory will create a transient user. Note that non-administrative access tokens can only create tokens for themselves.`,
			},
			"scope": {
				Type:        framework.TypeString,
				Required:    true,
				Description: `Required. See the JFrog Artifactory REST documentation on "Create Token" for a full and up to date description.`,
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
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathRoleWrite,
				Summary:  `Write information about the specified role.`,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRoleWrite,
				Summary:  `Overwrite information about the specified role.`,
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
	GrantType   string        `json:"grant_type,omitempty"`
	Username    string        `json:"username,omitempty"`
	Scope       string        `json:"scope"`
	Audience    string        `json:"audience,omitempty"`
	Description string        `json:"description,omitempty"`
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

	createOperation := (req.Operation == logical.CreateOperation)

	role := &artifactoryRole{}

	if !createOperation {
		existingRole, err := b.Role(ctx, req.Storage, roleName)
		if err != nil {
			return nil, err
		}
		if existingRole != nil {
			role = existingRole
		}
	}

	if value, ok := data.GetOk("grant_type"); ok {
		role.GrantType = value.(string)
	}

	if value, ok := data.GetOk("username"); ok {
		role.Username = value.(string)
	}

	if value, ok := data.GetOk("scope"); ok {
		role.Scope = value.(string)
	}

	if value, ok := data.GetOk("audience"); ok {
		role.Audience = value.(string)
	}

	// Looking at database/path_roles.go, it doesn't do any validation on these values during role creation.
	if value, ok := data.GetOk("default_ttl"); ok {
		role.DefaultTTL = time.Duration(value.(int)) * time.Second
	}

	if value, ok := data.GetOk("max_ttl"); ok {
		role.MaxTTL = time.Duration(value.(int)) * time.Second
	}

	if role.Scope == "" {
		return logical.ErrorResponse("missing scope"), nil
	}

	entry, err := logical.StorageEntryJSON("roles/"+roleName, role)
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

	role, err := b.Role(ctx, req.Storage, roleName)
	if err != nil {
		return nil, err
	}

	if role == nil {
		return nil, nil
	}

	return &logical.Response{
		Data: b.roleToMap(roleName, *role),
	}, nil
}

func (b *backend) Role(ctx context.Context, storage logical.Storage, roleName string) (*artifactoryRole, error) {

	entry, err := storage.Get(ctx, "roles/"+roleName)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var role artifactoryRole

	if err := entry.DecodeJSON(&role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (b *backend) roleToMap(roleName string, role artifactoryRole) (roleMap map[string]interface{}) {
	roleMap = map[string]interface{}{
		"role":        roleName,
		"scope":       role.Scope,
		"default_ttl": role.DefaultTTL.Seconds(),
		"max_ttl":     role.MaxTTL.Seconds(),
	}

	// Optional Attributes
	if len(role.GrantType) > 0 {
		roleMap["grant_type"] = role.GrantType
	}
	if len(role.Username) > 0 {
		roleMap["username"] = role.Username
	}
	if len(role.Audience) > 0 {
		roleMap["audience"] = role.Audience
	}

	return
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
