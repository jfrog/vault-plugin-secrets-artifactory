# Vault Artifactory Secrets Plugin

This plugin is actively maintained by JFrog Inc. Please refer to [CONTRIBUTING.md](CONTRIBUTING.md) for contributions and [create GitHub issues](https://github.com/jfrog/vault-plugin-secrets-artifactory/issues/new/choose) to ask for feature requests and support.

Contact [JFrog Support](https://jfrog.com/support/) for urgent, time sensitive issues.

----------------------------------------------------------------

This is a [HashiCorp Vault](https://www.vaultproject.io/) secret plugin which talks to JFrog Artifactory server and will
dynamically provision access tokens with specified scopes. This backend can be mounted multiple times
to provide access to multiple Artifactory servers.

Using this plugin, you can limit the accidental exposure window of Artifactory tokens; useful for continuous integration servers.

## Access Token Creation and Revoking

This backend creates access tokens in Artifactory using the admin credentials provided. Note that if you provide non-administrative credentials, then the "username" must match the username of the credential owner.

Visit [JFrog Help Center](https://jfrog.com/help/r/jfrog-platform-administration-documentation/introduction-to-access-tokens) for more information on Access Tokens.

### Admin Token Expiration Notice

> [!IMPORTANT]
> Prior to Artifactory 7.42.1, admin access token was created with the system token expiration (default to 1 year) even when `expires_in` API field is set to `0`. In 7.42.1, admin token expiration no longer constrained by system configuration and therefore can be set to non-expiring.
> See section ["Generate a Non-expiry Admin Token without Changing the Configuration"](https://www.jfrog.com/confluence/display/JFROG/Artifactory+Release+Notes#ArtifactoryReleaseNotes-Artifactory7.42.1Cloud) in the release note.
>
> Therefore if you created access token(s) with Artifactory prior to 7.42.1, the tokens will have a 1 year expiration time (or whatever value is set in the Artifactory configuration) and will become unusable silently when it expires.
>
> We suggest upgrading your Artifactory to 7.42.1 or later (if possible) and rotate your tokens to get new, non-expiring tokens. Or set reminders to ensure you rotate your tokens before expiration.
>
> It should also be noted that some "scripts" used to create an admin token may default to expiration in `1h`, so it is best to **rotate** the admin token immediately, to ensure it doesn't expire unexpectedly.
>
> If you are using v0.2.9 or later, you can check if your admin token has an expiration using `vault read artifactory/config/admin`. If the exp/expires fields are not present, your token has no expiration set.

### Dynamic Usernames

Previous versions of this plugin required a static `username` associated to the roles. This is still supported for backwards compatibility, but you can now use a dynamically generated username, based on [Vault Username Templates][vault-username-templating]. The generated tokens will be associated to a username generated from the template `v-{{.RoleName}}-{{Random 8}})` (`v-jenkins-x4mohTA8`), by default. You can change this template by specifying a `username_template=` option to the `/artifactory/config/admin` endpoint. The "scope" in the role should be `applied-permissions/groups:(list-of-groups)`, since `applied-permissions/user` would require the username to exist ahead of time. The user will not show in the Users list, but will be dynamically created during the scope of the token. The username still needs to be compliant with [artifactory requirements][artifactory-create-token] (less than 255 characters). It will be converted to lowercase by the API.

Example:

```sh
vault write artifactory/config/admin username_template="v_{{.DisplayName}}_{{.RoleName}}_{{random 10}}_{{unix_time}}"
```

### Expiring Tokens

By default, the Vault generated Artifactory tokens will not show an expiration date, which means that Artifactory will not
automatically revoke them. Vault will revoke the token when its lease expires due to logout or timeout (ttl/max_ttl). The reason
for this is because of the [default Revocable/Persistency Thresholds][artifactory-token-thresholds] in Artifactory. If you would
like the artifactory token itself to show an expiration, and you are using Artifactory v7.50.3 or higher, you can write
`use_expiring_tokens=true` to the `/artifactory/config/admin` path. This will set the `force_revocable=true` parameter and
set `expires_in` to either max lease TTL or role's `max_ttl`, whichever is lower, when a token is created, overriding the default
thresholds mentioned above.

Example:

```sh
vault write artifactory/config/admin use_expiring_tokens=true
```

Example Token Output:

```console
$ ACCESS_TOKEN=$(vault read -field access_token artifactory/token/test)
$ jwt decode $ACCESS_TOKEN

Token header
------------
{
"typ": "JWT",
"alg": "RS256",
"kid": "nxB2_1jNkYS5oYsl6nbUaaeALfKpfBZUyP0SW3txYUM"
}

Token claims
------------
{
"aud": "*@*",
"exp": 1678913614,
"ext": "{\"revocable\":\"true\"}",
"iat": 1678902814,
"iss": "jfac@01gvgpzpv8jytn0fvq41wb1srj",
"jti": "e39cec86-069c-4b75-8897-c2bf05dc8354",
"scp": "applied-permissions/groups:readers",
"sub": "jfac@01gvgpzpv8jytn0fvq41wb1srj/userv-test-p9nprfwr"
}
```

### Artifactory Version Detection

Some of the functionality of this plugin requires certain versions of Artifactory. For example, as of Artifactory 7.50.3, we can optionally set the `force_revocable` flag and set the expiration of the token to `max_ttl`.

If you have upgraded Artifactory after installing this plugin, and would like to take advantage of newer features, you can issue an empty write to the `artifactory/config/admin` endpoint to re-detect the version, or it will re-detect upon reload.

Example:

```sh
vault write -f artifactory/config/admin
```

## Installation

### Using pre-built releases

You can find pre-built releases of the plugin [here][artreleases] and download the latest binary file corresponding to your target OS.

### From Sources

If you prefer to build the plugin from sources, clone the GitHub repository locally and run the command `make build` from the root of the sources directory.

See [Local Development Prerequisites](#local-development-prerequisites) section for pre-requisites.

Upon successful compilation, the resulting `artifactory-secrets-plugin` binary is stored in the `dist/vault-plugin-secrets-artifactory_<OS architecture>` directory.

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
vault plugin register \
  -sha256=$(sha256sum path/to/plugin/directory/artifactory | cut -d " " -f 1) \
  -command=artifactory-secrets-plugin \
  secret artifactory
```

> [!NOTE]
> you may need to also add arguments to the registration like `-args="-ca-cert ca.pem` or something insecure like: `-args="-tls-skip-verify"` depending on your environment. (see `./path/to/plugins/artifactory -help` for all the options)

> [!CAUTION]
> This inline checksum calculation above is provided for illustration purpose and does not validate your binary. It should **not** be used for production environment. Instead you should use the checksum provided as [part of the release](https://github.com/jfrog/vault-plugin-secrets-artifactory/releases). See [How to verify binary checksums](#how-to-verify-binary-checksums) section.

You can now enable the Artifactory secrets plugin:

```sh
vault secrets enable artifactory
```

When upgrading, please refer to the [Vault documentation](https://developer.hashicorp.com/vault/docs/upgrading/plugins) for detailed instructions.


### How to verify binary checksums

Checksums for each binary are provided in the `artifactory-secrets-plugin_<version>_checksums.txt` file. It is signed with the public key [`vault-plugin-secrets-artifactory-public-key.asc`](vault-plugin-secrets-artifactory-public-key.asc) which creates the signature file `artifactory-secrets-plugin_<version>_checksums.txt.sig`.

If the public key is not in your GPG keychain, import it:
```sh
gpg --import artifactory-secrets-plugin-public-key.asc
```

Then verify the checksums file signature:

```sh
gpg --verify artifactory-secrets-plugin_<version>_checksums.txt.sig
```

You should see something like the following:
```sh
gpg: assuming signed data in 'artifactory-secrets-plugin_0.2.17_checksums.txt'
gpg: Signature made Mon May  8 14:22:12 2023 PDT
gpg:                using RSA key ED4FF1CD6C2318B470A33A1659FE1520A4A355CD
gpg: Good signature from "Alex Hung <alexh@jfrog.com>" [ultimate]
```

With the checksums file verified, you can now safely use the SHA256 checkum inside as part of the Vault plugin registration (vs calling `sha256sum`).

### Artifactory

1. Log into the Artifactory UI as an "admin".
1. Create the Access Token that Vault will use to interact with Artifactory. In Artifactory 7.x this can be done in the UI Administration (gear) -> User Management -> Access Tokens -> Generate Token.
    * Token Type: `Scoped Token`
    * Description: (optional) `vault-plugin-secrets-artifactory` (NOTE: This will be lost on admin token rotation, because it is not part of the token)
    * Token Scope: `Admin` **(IMPORTANT)**
    * User name: `vault-admin` (for example)
    * Service: `Artifactory` (or you can leave it on "All")
    * Expiration time: `Never` (do not set the expiration time less than `7h`, since by default, it will not be revocable once the expiration is less than 6h)
1. Save the generated token as the environment variable `TOKEN`

Alternatives:

* Use the [CreateToken REST API][artifactory-create-token], and save the `access_token` from the JSON response as the environment variable `TOKEN`.
* Use [`getArtifactoryAdminToken.sh`](./scripts/getArtifactoryAdminToken.sh).

    ```sh
    export JFROG_URL=https://artifactory.example.org
    export ARTIFACTORY_USERNAME=admin
    export ARTIFACTORY_PASSWORD=password
    TOKEN=$(scripts/getArtifactoryAdminToken.sh)
    ```

### Vault

```sh
vault write artifactory/config/admin \
    url=https://artifactory.example.org \
    access_token=$TOKEN
```

**OPTIONAL**, but recommended: Rotate the admin token, so that only Vault knows it.

```sh
vault write -f artifactory/config/rotate
```

> [!NOTE]
> some versions of artifactory (notably `7.39.10`) fail to rotate correctly. As noted above, we recommend being on `7.42.1` or higher. The token was indeed rotated, but as the error indicates, the old token could not be revoked.

**ALSO** If you want to change the username for the admin token (tired of it just being "admin"?) or set a "Description" on the token, those parameters are optionally available on the `artifactory/config/rotate` endpoint.

```sh
vault write artifactory/config/rotate username="new-username" description="A token used by vault-secrets-engine on our vault server"`
```

#### Bypass TLS connection verification with Artifactory

To bypass TLS connection verification with Artifactory, set `bypass_artifactory_tls_verification` to `true`, e.g.

```sh
vault write artifactory/config/admin \
    url=https://artifactory.example.org \
    access_token=$TOKEN \
    bypass_artifactory_tls_verification=true
```

OPTIONAL: Check the results:

```sh
vault read artifactory/config/admin
```

Example output:

```console
Key                                 Value
---                                 -----
access_token_sha256                 74834a86b2082750201e2a1e520f21f7bfc7d4026e5bd2b075ca2d0699b7c4e3
bypass_artifactory_tls_verification false
scope                               applied-permissions/admin
token_id                            db0002b0-af08-486c-bbad-b255a3cc7b31
url                                 http://localhost:8082
use_expiring_tokens                 false
username                            vault-admin
version                             7.55.6
```

#### Use expiring tokens

To enable creation of token that expires using TTL (system default, system max TTL, or config overrides), set `use_expiring_tokens` to `true`, e.g.

```sh
vault write artifactory/config/admin \
    url=https://artifactory.example.org \
    access_token=$TOKEN \
    use_expiring_tokens=true
```

## Usage

Create a role (scope for artifactory >= 7.21.1)

```sh
vault write artifactory/roles/jenkins \
    scope="applied-permissions/groups:automation " \
    default_ttl=3600 max_ttl=10800
```

Also supports `grant_type=[Optional, default: "client_credentials"]`, and `audience=[Optional, default: *@*]` see [JFrog documentation][artifactory-create-token].

> [!NOTE]
> By default, the username will be generated automatically using the template `v-(RoleName)-(random 8)` (i.e. `v-jenkins-x4mohTA8`). If you would prefer to have a static username (the same for every token), you can set `username=whatever-you-want`, but keep in mind that in a dynamic environment, someone or something using an old, expired token might cause a denial of service (too many failed logins) against users with the correct token.

<details>
<summary>CLICK for: Create a Role (scope for artifactory < 7.21.1)</summary>

```sh
vault write artifactory/roles/jenkins \
    username="example-service-jenkins" \
    scope="api:* member-of-groups:ci-server" \
    default_ttl=1h max_ttl=3h
```

</details>

> [!NOTE]
> There are some changes in the **scopes** supported in artifactory request >7.21. Please refer to the JFrog documentation for the same according to the artifactory version.

```sh
vault list artifactory/roles
```

Example Output:

```console
Keys
----
jenkins
```

```sh
vault read artifactory/token/jenkins
```

Example output (token truncated):

```console
Key                Value
---                -----
lease_id           artifactory/token/jenkins/9hHxV1NlyLzPgmNIzjssRCa9
lease_duration     1h
lease_renewable    true
access_token       eyJ2ZXIiOiIyIiw...
role               jenkins
scope              applied-permissions/groups:automation
token_id           06d962b2-63e2-4279-a25d-d2a9cab6507f
username           v-jenkins-x4mohTA8
```

### User Token Path

User tokens may be obtained from the `/artifactory/user_token/<user-name>` endpoint. This is useful in conjunction with [ACL Policy Path Templating](https://developer.hashicorp.com/vault/tutorials/policies/policy-templating) to allow users authenticated to Vault to obtain API tokens in Artfactory for their own account. Be careful to ensure that Vault authentication methods & policies align with user account names in Artifactory.

For example the following policy allows users authenticated to the `azure-ad-oidc` authentication mount to obtain a token for Artifactory for themselves, assuming the `upn` metadata is populated in Vault during authentication.

```
path "artifactory/user_token/{{identity.entity.aliases.azure-ad-oidc.metadata.upn}}" {
  capabilities = [ "read" ]
}
```

Default values for the token's `access_token`, `description`, `ttl`, `max_ttl`, `audience`, `refreshable`, `include_reference_token`, and `use_expiring_tokens` may be configured at the `/artifactory/config/user_token` or `/artifactory/config/user_token/<user-name>` path.

`access_token` field allows the use of user's identity token in place of the admin access token from the `/artifactory/config/admin` path, enabling creating access token scoped to that user only.

TTL rules follow Vault's [general cases](https://developer.hashicorp.com/vault/docs/concepts/tokens#the-general-case) and [token hierarchy](https://developer.hashicorp.com/vault/docs/concepts/tokens#token-hierarchies-and-orphan-tokens). The desired lease TTL will be determined by the most specific TTL value specified with the request ttl parameter being highest precedence, followed by the plugin configuration, secret mount tuning, or system default ttl. The maximum TTL value allowed is limited to the lowest value of the `max_ttl` setting set on the system, secret mount tuning, plugin configuration, or the specific request.

Example Token Configuration:

```console
vault write artifactory/config/user_token \
  default_description="Generated by Vault" \
  max_ttl=604800 \
  default_ttl=86400
```

```console
$ vault read artifactory/config/user_token
Key                        Value
---                        -----
audience                   n/a
default_description        Generated by Vault
default_ttl                24h
include_reference_token    true
max_ttl                    168h
refreshable                true
scope                      applied-permissions/user
token_id                   8df5dd21-31ae-4062-bbe5-580a607f5645
username                   vault-admin
```

Example Usage:
```console
$ vault read artifactory/user_token/admin description="Dev Desktop"
Key                Value
---                -----
lease_id           artifactory/user_token/admin/4UhTThCwctPGX0TYXeoyoVEt
lease_duration     24h
lease_renewable    true
access_token       eyJ2Z424242424...
description        Dev Desktop
reference_token    cmVmdGtu...
refresh_token      629299be-...
scope              applied-permissions/user
token_id           3c6b2e63-87dc-4d26-9698-ffdfb282a6ee
username           admin
```

## References

### Admin Config

| Command | Path |
| ------- | ---- |
| write   | artifactory/config/admin |
| read    | artifactory/config/admin |
| delete  | artifactory/config/admin |

Configure the parameters used to connect to the Artifactory server integrated with this backend.

The two main parameters are `url` which is the absolute URL to the Artifactory server. Note that `/artifactory/api`
is prepended by the individual calls, so do not include it in the URL here.

The second is `access_token` which must be an access token enough permissions to generate the other access tokens you'll
be using. This value is stored seal wrapped when available. Once set, the access token cannot be retrieved, but the backend
will send a sha256 hash of the token so you can compare it to your notes. If the token is a JWT Access Token, it will return
additional information such as `jfrog_token_id`, `username` and `scope`.

An optional `username_template` parameter will override the built-in default username_template for dynamically generating
usernames if a static one is not provided.

An optional `bypass_artifactory_tls_verification` parameter will enable bypassing the TLS connection verification with Artifactory.

No renewals or new tokens will be issued if the backend configuration (config/admin) is deleted.

#### Parameters

* `url` (string) - Address of the Artifactory instance, e.g. https://my.jfrog.io
* `access_token` (stirng) - Administrator token to access Artifactory
* `username_template` (string) - Optional. Vault Username Template for dynamically generating usernames.
* `use_expiring_tokens` (boolean) - Optional. If Artifactory version >= 7.50.3, set `expires_in` to `max_ttl` (admin token) or `ttl` (user token) and `force_revocable = true`. Default to `false`.
* `bypass_artifactory_tls_verification` (boolean) - Optional. Bypass certification verification for TLS connection with Artifactory. Default to `false`.

#### Example

```console
vault write artifactory/config/admin url=$JFROG_URL \
  access_token=$JFROG_ACCESS_TOKEN \
  username_template="v_{{.DisplayName}}_{{.RoleName}}_{{random 10}}_{{unix_time}}" \
  use_expiring_tokens=true \
  bypass_artifactory_tls_verification=true
```

### User Token Config

| Command | Path |
| ------- | ---- |
| write   | artifactory/user_config |
| read    | artifactory/user_config |
| write   | artifactory/user_config/:username |
| read    | artifactory/user_config/:username |

Configures default values for the `user_token/:user-name` path. The optional `username` field allows the configuration to be set for specific username.

#### Parameters

* `access_token` (stirng) - Optional. User identity token to access Artifactory. If `username` is not set then this token will be used for *all* users.
* `refresh_token` (string) - Optional. Refresh token for the user access token. If `username` is not set then this token will be used for *all* users.
* `audience` (string) - Optional. See the JFrog Platform REST documentation on [Create Token](https://jfrog.com/help/r/jfrog-rest-apis/create-token) for a full and up to date description. Service ID must begin with valid JFrog service type. Options: jfrt, jfxr, jfpip, jfds, jfmc, jfac, jfevt, jfmd, jfcon, or *. For instructions to retrieve the Artifactory Service ID see this [documentation](https://jfrog.com/help/r/jfrog-rest-apis/get-service-id)
* `refreshable` (boolean) - Optional. A refreshable access token gets replaced by a new access token, which is not what a consumer of tokens from this backend would be expecting; instead they'd likely just request a new token periodically. Set this to `true` only if your usage requires this. See the JFrog Platform documentation on [Generating Refreshable Tokens](https://jfrog.com/help/r/jfrog-platform-administration-documentation/generating-refreshable-tokens) for a full and up to date description. Defaults to `false`. 
* `include_reference_token` (boolean) - Optional. Generate a Reference Token (alias to Access Token) in addition to the full token (available from Artifactory 7.38.10). A reference token is a shorter, 64-character string, which can be used as a bearer token, a password, or with the `X-JFrog-Art-Api`header. Note: Using the reference token might have performance implications over a full length token. Defaults to `false`. 
* `use_expiring_tokens` (boolean) - Optional. If Artifactory version >= 7.50.3, set `expires_in` to `ttl` and `force_revocable = true`. Defaults to `false`. 
* `default_ttl` (int64) - Optional. Default TTL for issued user access tokens. If unset, uses the backend's `default_ttl`. Cannot exceed `max_ttl`. 
* `default_description` (string) - Optional. Default token description to set in Artifactory for issued user access tokens.

#### Examples

```console
# Set user token configuration for ALL users
vault write artifactory/config/user_token \
  access_token="eyJ2Z...3sT9r6nA" \
  refresh_token="4ab...471" \
  default_ttl=60s

vault read artifactory/config/user_token

# Set user token configuration for 'myuser' user
vault write artifactory/config/user_token/myuser \
  access_token="eyJ2Z...3sT9r6nA" \
  refresh_token="4ab...471" \
  audience="jfrt@* jfxr@*"

vault read artifactory/config/user_token/myuser

vault delete artifactory/config/user_token/myuser
```

### Role

| Command | Path |
| ------- | ---- |
| write   | artifactory/role/:rolename |
| patch   | artifactory/role/:rolename |
| read    | artifactory/role/:rolename |
| delete  | artifactory/role/:rolename |

#### Parameters

* `grant_type` (stirng) - Optional. Defaults to `client_credentials` when creating the access token. You likely don't need to change this.
* `username` (string) - Optional. Defaults to using the username_template. The static username for which the access token is created. If the user does not exist, Artifactory will create a transient user. Note that non-administrative access tokens can only create tokens for themselves.
* `scope` (string) - Space-delimited list. See the JFrog Artifactory REST documentation on ["Create Token"](https://jfrog.com/help/r/jfrog-rest-apis/create-token) for a full and up to date description.
* `refreshable` (boolean) - Optional. A refreshable access token gets replaced by a new access token, which is not what a consumer of tokens from this backend would be expecting; instead they'd likely just request a new token periodically. Set this to `true` only if your usage requires this. See the JFrog Platform documentation on [Generating Refreshable Tokens](https://jfrog.com/help/r/jfrog-platform-administration-documentation/generating-refreshable-tokens) for a full and up to date description. Defaults to `false`.
* `audience` (string) - Optional. See the JFrog Platform REST documentation on [Create Token](https://jfrog.com/help/r/jfrog-rest-apis/create-token) for a full and up to date description. Service ID must begin with valid JFrog service type. Options: jfrt, jfxr, jfpip, jfds, jfmc, jfac, jfevt, jfmd, jfcon, or *. For instructions to retrieve the Artifactory Service ID see this [documentation](https://jfrog.com/help/r/jfrog-rest-apis/get-service-id)
* `include_reference_token` (boolean) - Optional. Generate a Reference Token (alias to Access Token) in addition to the full token (available from Artifactory 7.38.10). A reference token is a shorter, 64-character string, which can be used as a bearer token, a password, or with the `X-JFrog-Art-Api`header. Note: Using the reference token might have performance implications over a full length token. Defaults to `false`.
* `default_ttl` (int64) - Default TTL for issued user access tokens. If unset, uses the backend's `default_ttl`. Cannot exceed `max_ttl`.
* `max_ttl` (int64) - Maximum TTL that an access token can be renewed for. If unset, uses the backend's `max_ttl`. Cannot exceed backend's `max_ttl`. 

#### Examples

```console
vault write artifactory/roles/test \
  scope="applied-permissions/groups:readers applied-permissions/groups:ci" \
  max_ttl=3h \
  default_ttl=2h

vault read artifactory/roles/test

vault delete artifactory/roles/test
```

### Admin Token

| Command | Path |
| ------- | ---- |
| read   | artifactory/token/:rolename |

Create an Artifactory access token using paramters from the specified role.

#### Parameters

* `ttl` (int64) - Optional. Override the default TTL when issuing this access token. Cannot exceed smallest (system, backend, role, this request) maximum TTL.
* `max_ttl` (int64) - Optional. Override the maximum TTL for this access token. Cannot exceed smallest (system, backend) maximum TTL.

#### Examples

```console
vault read artifactory/token/test \
  ttl=30m \
  max_ttl=1h
```

### Rotate Admin Token

| Command | Path |
| ------- | ---- |
| write   | artifactory/config/rotate |

This will rotate the `access_token` used to access artifactory from this plugin. A new access token is created first then revokes the old access token.

#### Examples

```console
vault write artifactory/config/rotate
```

### User Token

| Command | Path |
| ------- | ---- |
| read    | artifactory/user_token/:username |

Provides optional parameters to override default values for the user_token/:username path

#### Parameters

* `description` (string) - Optional. Override the token description to set in Artifactory for issued user access tokens.
* `refreshable` (boolean) - Optional. Override the `refreshable` for this access token. Defaults to `false`.
* `include_reference_token` (boolean) - Optional. Override the `include_reference_token` for this access token. Defaults to `false`.
* `use_expiring_tokens` (boolean) - Optional. Override the `use_expiring_tokens` for this access token. If Artifactory version >= 7.50.3, set `expires_in` to `ttl` and `force_revocable = true`. Defaults to `false`. 
* `ttl` (int64) - Optional. Override the default TTL when issuing this access token. Cannot exceed smallest (system, backend, role, this request) maximum TTL.
* `max_ttl` (int64) - Optional. Override the maximum TTL for this access token. Cannot exceed smallest (system, backend) maximum TTL.

#### Examples

```console
vault read artifactory/user_token/test_user \
  description="Refreshable token for Test user"
  refreshable=true \
  include_reference_token=true \
  use_expiring_tokens=true
```

## Development

### Local Development Prerequisites

* Vault
  * <https://developer.hashicorp.com/vault/docs/install>
  * `brew install vault`
* Golang
  * <https://go.dev/doc/install>
  * `brew install golang`
* GoReleaser - Used during the build process
  *  <https://goreleaser.com/install/>
  * `brew install goreleaser`
* Docker - To run a test Artifactory instance (very useful for testing)
  * <https://docs.docker.com/get-docker/>
  * `brew install docker --cask`

### Testing Locally

If you're compiling this yourself and want to test locally, you will need a working Docker environment. You will also need Vault cli and Golang installed, then you can follow the steps below.

* In first terminal, build the plugin and start the local dev server:

```sh
make
```

* In another terminal, setup a test artifactory instance.

```sh
make artifactory
```

* In the same terminal, setup `artifactory-secrets-engine` in vault with values:

```sh
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=root
make setup
```

* In the same terminal, you can configure and generate an admin access token:

```sh
make admin
```

Generate an user token:

```sh
make usertoken
```

NOTE: Each time you rebuild (`make`), vault will restart, so you will need to run `make setup` again, since vault is in dev mode.

* Once you are done testing, you can destroy the local artifactory instance:

```sh
make stop_artifactory
```

### Other Local Development Details

This section is informational, and is not intended as a step-by-step. If you really want the gory details, checkout [the `Makefile`](./Makefile)

#### Install Vault binary

* You can follow the [Installing Vault](https://developer.hashicorp.com/vault/docs/install) instructions.
* Alternatively, if you are on MacOS, and have HomeBrew, you can use that:

```sh
brew tap hashicorp/tap
brew install hashicorp/tap/vault
```

#### Start Vault dev server

```sh
make start
```

#### Export Vault url and token

```sh
export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN=root
```

#### Build plugin binary

```sh
make build
```

#### Upgrade plugin binary

To build and upgrade the plugin without having to reconfigure it...

```sh
make upgrade
```

#### Create Test Artifactory

```sh
make artifactory
```

Set `ARTIFACTORY_VERSION` to a [specific self hosted version][artifactory-release-notes] to override the default.

Example:

```sh
make artifactory ARTIFACTORY_VERSION=7.49.10
```

> [!NOTE]
> If you get a message like:
>
>```console
>make: Nothing to be done for `artifactory'.
>```
>
>This simply means that "make" thinks artifactory is >already running due to the existence of the `./vault/>artifactory.env` file.
>
>If you want to run a different version, first use `make >stop_artifactory`. If you stopped artifactory using other >means (docker), then `rm vault/artifactory.env` manually.


#### Register artifactory-secrets plugin with Vault server

If you didn't run `make upgrade` (i.e. just `make build`), then you need to register the newly built plugin with the Vault server.

```sh
make register
```

#### Enable artifactory-secrets plugin

```sh
make enable
```

#### Disable plugin (unmount from vault)

```sh
make disable
```

> [!NOTE]
> This is a good idea before stopping artifactory, especially if you plan to change versions of artifactory. Alternatively, just exit vault (Ctrl+c), and it will go back to default state.

#### Get ADMIN Artifactory token and write it to vault

```sh
make admin
```

NOTE: This following might be some useful environment variables:

> * `JFROG_URL`
> * `ARTIFACTORY_USERNAME`
> * `ARTIFACTORY_PASSWORD`

For example:

```sh
JFROG_URL=https://artifactory.example.org ARTIFACTORY_USERNAME=tommy ARTIFACTORY_PASSWORD='SuperSecret' make admin
```

If you already have a `JFROG_ACCESS_TOKEN``, you can skip straight to that too:

```sh
export JFROG_URL=https://artifactory.example.com
export JFROG_ACCESS_TOKEN=(PASTE YOUR JFROG ADMIN TOKEN)
make admin
```

* Setup a "test" role, bound to the "readers" group

```sh
make testrole
```

#### Run Acceptance Tests

```sh
make acceptance
```

This requires the following:
* A running Artifactory instance
* Env vars `JFROG_URL` and `JFROG_ACCESS_TOKEN` for the running Artifactory instance be set

## Issues

* RTFACT-22477 - proposing CIDR restrictions on the created access tokens.
* () - Artifactory 7.39.10 fails to revoke previous token during rotation. Recommend 7.42.1. or higher.

## Contributors

See the [contribution guide](./CONTRIBUTING.md).

## License

Copyright (c) 2024 JFrog.

Apache 2.0 licensed, see [LICENSE][LICENSE] file.

[LICENSE]: ./LICENSE
[artreleases]: https://github.com/jfrog/vault-plugin-secrets-artifactory/releases
[vaultdocplugindir]: https://www.vaultproject.io/docs/configuration/index.html#plugin_directory
[vaultdocplugincatalog]: https://www.vaultproject.io/docs/internals/plugins.html#plugin-catalog
[artifactory-create-token]: https://www.jfrog.com/confluence/display/JFROG/JFrog+Platform+REST+API#JFrogPlatformRESTAPI-CreateToken
[vault-username-templating]: https://developer.hashicorp.com/vault/docs/concepts/username-templating
[artifactory-release-notes]: https://www.jfrog.com/confluence/display/JFROG/Artifactory+Release+Notes
[artifactory-token-thresholds]: https://www.jfrog.com/confluence/display/JFROG/Access+Tokens#AccessTokens-UsingtheRevocableandPersistencyThresholds
