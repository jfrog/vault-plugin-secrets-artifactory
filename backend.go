package artifactory

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/helper/template"
	"github.com/hashicorp/vault/sdk/logical"
)

type backend struct {
	*framework.Backend
	configMutex      sync.RWMutex
	rolesMutex       sync.RWMutex
	httpClient       *http.Client
	usernameProducer template.StringTemplate
	version          string
}

// UsernameMetadata defines the metadata that a user_template can use to dynamically create user account in Artifactory
type UsernameMetadata struct {
	DisplayName string
	RoleName    string
}

// Factory configures and returns Artifactory secrets backends.
func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	if conf == nil {
		return nil, fmt.Errorf("configuration passed into backend is nil")
	}

	b, err := Backend(conf)
	if err != nil {
		return nil, err
	}

	if err := b.Backend.Setup(ctx, conf); err != nil {
		return nil, err
	}

	return b, nil
}
func Backend(_ *logical.BackendConfig) (*backend, error) {
	b := &backend{
		httpClient: http.DefaultClient,
	}

	up, err := testUsernameTemplate(defaultUserNameTemplate)
	if err != nil {
		return nil, err
	}
	b.usernameProducer = up

	b.Backend = &framework.Backend{
		Help: strings.TrimSpace(artifactoryHelp),

		PathsSpecial: &logical.Paths{
			SealWrapStorage: []string{"config/admin"},
		},

		BackendType:    logical.TypeLogical,
		InitializeFunc: b.initialize,
	}
	b.Backend.Secrets = append(b.Backend.Secrets, b.secretAccessToken())
	b.Backend.Paths = append(b.Backend.Paths,
		b.pathListRoles(),
		b.pathRoles(),
		b.pathTokenCreate(),
		b.pathConfig(),
		b.pathConfigRotate())

	return b, nil
}

// initialize will initialize the backend configuration
func (b *backend) initialize(ctx context.Context, req *logical.InitializationRequest) error {
	config, err := b.fetchAdminConfiguration(ctx, req.Storage)
	if err != nil {
		return err
	}

	if config == nil {
		return nil
	}

	err = b.getVersion(*config)
	if err != nil {
		return err
	}

	if len(config.UsernameTemplate) != 0 {
		up, err := testUsernameTemplate(config.UsernameTemplate)
		if err != nil {
			return err
		}
		b.usernameProducer = up
	}

	return nil
}

// fetchAdminConfiguration will return nil,nil if there's no configuration
func (b *backend) fetchAdminConfiguration(ctx context.Context, storage logical.Storage) (*adminConfiguration, error) {
	var config adminConfiguration

	// Read in the backend configuration
	entry, err := storage.Get(ctx, "config/admin")
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	if err := entry.DecodeJSON(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

const artifactoryHelp = `
The Artifactory secrets backend provides Artifactory access tokens based on configured roles.
`
