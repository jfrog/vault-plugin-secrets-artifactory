# Vault Artifactory Secrets Plugin

----------------------------------------------------------------

This plugin is now being actively maintained by JFrog Inc. Please refer to [CONTRIBUTING.md](CONTRIBUTING.md) for contributions and create github issues to ask for support
-----------------------------------------------------------------

![Build](https://github.com/idcmp/artifactory-secrets-plugin/workflows/Build/badge.svg)

This is a [HashiCorp Vault](https://www.vaultproject.io/) plugin which talks to JFrog Artifactory server and will
dynamically provision access tokens with specified scopes. This backend can be mounted multiple times
to provide access to multiple Artifactory servers.

Using this plugin, you can limit the accidental exposure window of Artifactory tokens; useful for continuous integration servers.

## Access Token Creation and Revoking

This backend creates access tokens in Artifactory using the admin credentials provided. Note that if you provide non-admin credentials, then the "username" must match the username of the credential owner.

> **Note**
> Prior to Artifactory 7.42.1, admin access token was created with the system token expiration (default to 1 hour) even when `expires_in` API field is set to 0. In 7.42.1, admin token expiration no longer constrained by system configuration and therefore can be set to non-expiring. See section ["Generate a Non-expiry Admin Token without Changing the Configuration"](https://www.jfrog.com/confluence/display/JFROG/Artifactory+Release+Notes#ArtifactoryReleaseNotes-Artifactory7.42.1Cloud) in the release note.
>
> Therefore if you created access token(s) with Artifactory prior to 7.42.1, the tokens will have a 1 year expiration time (or whatever value set in Artifactory configuration) and will become unusable silently when it expires.
>
> We suggest upgrade your Artifactory to 7.42.1 or later (if possible) and rotate your tokens to get new, non-expiring tokens. Or set reminders to ensure you rotate your tokens before expiration.

## Testing Locally

If you're compiling this yourself and want to do a local sanity test, you
can do something like:

In first terminal, build the plugin and start the local dev server:
```sh
make
```

In another terminal, setup the vault with values:
```sh
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=root
make setup
```

Once that's completed, in the same terminal:
```sh
make artifactory &  # Runs netcat returning a static JSON response
vault read artifactory/token/test
```

## Installation

### Using pre-built releases

You can find pre-built releases of the plugin [here][artreleases]. Once you have downloaded the latest archive corresponding to your target OS, uncompress it to retrieve the `artifactory` binary file.

### From Sources

If you prefer to build the plugin from sources, clone the GitHub repository locally and run the command `make build` from the root of the sources directory. Upon successful compilation, the resulting `artifactory` binary is stored in the `vault/plugins` directory.

## Configuration

Copy the plugin binary into a location of your choice; this directory must be specified as the [`plugin_directory`][vaultdocplugindir] in the Vault configuration file:

```hcl
plugin_directory = "path/to/plugin/directory"
```

Start a Vault server with this configuration file:

```sh
vault server -config=path/to/vault/config.hcl
```

Once the server is started, register the plugin in the Vault server's [plugin catalog][vaultdocplugincatalog]:

```sh
vault write sys/plugins/catalog/secret/artifactory \
    sha_256="$(sha256sum path/to/plugin/directory/artifactory | cut -d " " -f 1)" \
    command="artifactory"
```

> **Note**
> This checksum above is provided for illustration purpose and does not validate your binary. It should **not** be used for production environment. At minimum, you should use the checksum provided as [part of the release](https://github.com/jfrog/artifactory-secrets-plugin/releases).

You can now enable the Artifactory secrets plugin:

```sh
vault secrets enable artifactory
```

## Usage

### Artifactory

You will need the "admin" user's password (not an admin, but admin specifically).

1. Log into the Artifactory UI as "admin".
1. Under "Welcome, admin" (top right) go to "Edit Profile".
1. Create the Access Token that Vault will use to interact with Artifactory. In Artifactory 7.x this can be done in the UI Administration -> User Management -> Access Tokens -> Generate Token. (Scoped Token, User name: `admin`, Service: `Artifactory`, Expiration time: `Never`). Or use the [CreateToken REST API][artifactory-create-token]. See `get-access-key.sh` in [Terraform Artifactory Provider](https://github.com/jfrog/terraform-provider-artifactory/blob/master/scripts/get-access-key.sh).

Note that `username` must be `admin` otherwise you will not be able to specify different usernames for roles. Save the `access_token` from the JSON response as the environment variable `TOKEN`.

```sh
vault write artifactory/config/admin \
    url=https://artifactory.example.org/artifactory \
    access_token=$TOKEN
```

* Rotate the admin token, so that only vault knows it.

```sh
vault write -f artifactory/config/rotate
```

* Create a Role (scope for artifactory < 7.21.1)

```sh
vault write artifactory/roles/jenkins \
    username="example-service-jenkins" \
    scope="api:* member-of-groups:ci-server" \
    default_ttl=1h max_ttl=3h
```

* Create a role (scope for artifactory >= 7.21.1)

```sh
vault write artifactory/roles/jenkins \
    username="example-service-jenkins" \
    scope="applied-permissions/groups:automation " \
    default_ttl=1h max_ttl=3h
```

Also supports `grant_type=[Optional, default: "client_credentials"]`, and `audience=[Optional, default: *@*]` see [JFrog documentation][artifactory-create-token].

> **Note**
> There are some changes in the **scopes** supported in artifactory request >7.21. Please refer to the JFrog documentation for the same according to the artifactory version.

```sh
vault list artifactory/roles

Keys
----
jenkins
```

```sh
vault read artifactory/token/jenkins

Key                Value
---                -----
lease_id           artifactory/token/jenkins/25jYH8DjUU548323zPWiSakh
lease_duration     1h
lease_renewable    true
access_token       adsdgbtybbeeyh...
role               jenkins
scope              api:* member-of-groups:ci-server
```

## Development

1. Install Vault binary
```sh
brew tap hashicorp/tap
brew install hashicorp/tap/vault
```

1. Start Vault dev server
```sh
make start
```

1. In a separate shell, build plugin binary
```sh
make build
```

1. Export Vault url and enable plugin
```sh
export VAULT_ADDR='http://127.0.0.1:8200'
make enable
```

1. Export auth token and write it to the vault
```sh
export TOKEN=<Your Artifactory auth token>
vault write artifactory/config/admin \
    url=http://127.0.0.1:8200/artifactory \
    access_token=$TOKEN
```

## Issues

RTFACT-22477, proposing CIDR restrictions on the created access tokens.

[artreleases]: https://github.com/jfrog/artifactory-secrets-plugin/releases
[vaultdocplugindir]: https://www.vaultproject.io/docs/configuration/index.html#plugin_directory
[vaultdocplugincatalog]: https://www.vaultproject.io/docs/internals/plugins.html#plugin-catalog
[artifactory-create-token]: https://www.jfrog.com/confluence/display/JFROG/JFrog+Platform+REST+API#JFrogPlatformRESTAPI-CreateToken


## Contributors
See the [contribution guide](./CONTRIBUTING.md).

## License

Copyright (c) 2023 JFrog.

Apache 2.0 licensed, see [LICENSE][LICENSE] file.

[LICENSE]: ./LICENSE
