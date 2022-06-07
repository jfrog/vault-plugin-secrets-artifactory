package artifactory

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

type backend struct {
	*framework.Backend
	configMutex sync.RWMutex
	rolesMutex  sync.RWMutex
	httpClient  *http.Client
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

	b.Backend = &framework.Backend{
		Help: strings.TrimSpace(artifactoryHelp),

		PathsSpecial: &logical.Paths{
			SealWrapStorage: []string{"config/admin"},
		},

		BackendType: logical.TypeLogical,
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
