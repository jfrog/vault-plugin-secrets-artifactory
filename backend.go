package artifactory

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/helper/template"
	"github.com/hashicorp/vault/sdk/logical"
)

var Version = "v1.0.0"
var productId = "vault-plugin-secrets-artifactory/" + Version[1:] // don't need the 'v' prefix in productId

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
	b := &backend{}

	up, err := testUsernameTemplate(defaultUserNameTemplate)
	if err != nil {
		return nil, err
	}
	b.usernameProducer = up

	b.Backend = &framework.Backend{
		Help:           strings.TrimSpace(artifactoryHelp),
		RunningVersion: Version,

		PathsSpecial: &logical.Paths{
			SealWrapStorage: []string{configAdminPath},
		},

		BackendType:    logical.TypeLogical,
		InitializeFunc: b.initialize,
		Invalidate:     b.invalidate,
	}
	b.Backend.Secrets = append(b.Backend.Secrets, b.secretAccessToken())
	b.Backend.Paths = append(b.Backend.Paths,
		b.pathListRoles(),
		b.pathRoles(),
		b.pathTokenCreate(),
		b.pathUserTokenCreate(),
		b.pathConfig(),
		b.pathConfigRotate(),
		b.pathConfigUserToken())

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

	b.InitializeHttpClient(&config.baseConfiguration)

	err = b.getVersion(config.baseConfiguration)
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

func (b *backend) InitializeHttpClient(config *baseConfiguration) {
	if config.BypassArtifactoryTLSVerification {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.BypassArtifactoryTLSVerification,
			},
		}

		b.httpClient = &http.Client{Transport: tr}
	} else {
		b.httpClient = http.DefaultClient
	}
}

// invalidate clears an existing client configuration in
// the backend
func (b *backend) invalidate(ctx context.Context, key string) {
	if key == "config" {
		b.reset()
	}
}

// reset clears any client configuration for a new
// backend to be configured
func (b *backend) reset() {
	b.configMutex.Lock()
	defer b.configMutex.Unlock()
	b.httpClient = nil
}

const artifactoryHelp = `
The Artifactory secrets backend provides Artifactory access tokens based on configured roles.
`
