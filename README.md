# Vault Artifactory Secrets Plugin

----------------------------------------------------------------

This plugin is now being actively maintained by JFrog Inc.Please refer to [CONTRIBUTING.md](CONTRIBUTING.md) for contributions and create github issues to ask for support

----------------------------------------------------------------

![Build](https://github.com/jfrog/artifactory-secrets-plugin/actions/workflows/build.yml/badge.svg)

This is a [HashiCorp Vault](https://www.vaultproject.io/) plugin which talks to JFrog Artifactory server and will
dynamically provision access tokens with specified scopes. This backend can be mounted multiple times
to provide access to multiple Artifactory servers.

Using this plugin, you can limit the accidental exposure window of Artifactory tokens; useful for continuous integration servers.

## Access Token Creation and Revoking

This backend creates access tokens in Artifactory using the admin credentials provided. Note that if you provide non-administrative credentials, then the "username" must match the username of the credential owner.

### Admin Token Expiration Notice

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

* Example:

```sh
vault write artifactory/config/admin username_template="v_{{.DisplayName}}_{{.RoleName}}_{{random 10}}_{{unix_time}}"
```

### Artifactory Version Detection

Some of the functionality of this plugin requires certain versions of Artifactory. For example, as of Artifactory 7.50.3, we set the `force_revocable` flag and set the expiration of the token to `max_ttl`.
If you have upgraded Artifactory after installing this plugin, and would like to take advantage of newer features, you can issue an empty write to the `artifactory/config/admin` endpoint to re-detect the version, or it will re-detect upon reload.

* Example:

```sh
vault write -f artifactory/config/admin
```

## Installation

### Using pre-built releases

You can find pre-built releases of the plugin [here][artreleases]. Once you have downloaded the latest archive corresponding to your target OS, unzip it to retrieve the `artifactory` binary file.

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
vault plugin register \
  -sha_256=$(sha256sum path/to/plugin/directory/artifactory | cut -d " " -f 1) \
  secret artifactory
```

* NOTE: you may need to also add arguments to the registration like `-args="-ca-cert ca.pem` or something insecure like: `-args="-tls-skip-verify"` depending on your environment. (see `./path/to/plugins/artifactory -help` for all the options)

> **Note**
> This inline checksum calculation above is provided for illustration purpose and does not validate your binary. It should **not** be used for production environment. At minimum, you should use the checksum provided as [part of the release](https://github.com/jfrog/artifactory-secrets-plugin/releases).

You can now enable the Artifactory secrets plugin:

```sh
vault secrets enable artifactory
```

## Usage

### Artifactory

1. Log into the Artifactory UI as an "admin".
1. Create the Access Token that Vault will use to interact with Artifactory. In Artifactory 7.x this can be done in the UI Administration (gear) -> User Management -> Access Tokens -> Generate Token.
    * Token Type: `Scoped Token`
    * Description: (optional) `Vault-artifactory-secrets-plugin` (NOTE: This will be lost on admin token rotation, because it is not part of the token)
    * Token Scope: `Admin` **(IMPORTANT)**
    * User name: `vault-admin` (for example)
    * Service: `Artifactory` (or you can leave it on "All")
    * Expiration time: `Never` (do not set the expiration time less than `7h`, since by default, it will not be revocable once the expiration is less than 6h)
1. Save the generated token as the environment variable `TOKEN`

Alternatives:

* Use the [CreateToken REST API][artifactory-create-token], and save the `access_token` from the JSON response as the environment variable `TOKEN`.
* Use [`getArtifactoryAdminToken.sh`](./scripts/getArtifactoryAdminToken.sh).

    ```sh
    export ARTIFACTORY_URL=https://artifactory.example.org
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

* OPTIONAL, but recommended: Rotate the admin token, so that only vault knows it.

```sh
vault write -f artifactory/config/rotate
```

**NOTE** some versions of artifactory (notably `7.39.10`) fail to rotate correctly. As noted above, we recommend being on `7.42.1` or higher. The token was indeed rotated, but as the error indicates, the old token could not be revoked.

* OPTIONAL: Check the results:

```sh
vault read artifactory/config/admin
```

Example output:

```console
Key                    Value
---                    -----
access_token_sha256    f4259e051d6f81732d65ffb648f09fc959411c2c8fcbdc9bffae2179021ccb91
scope                  applied-permissions/admin
token_id               4b7f6b11-069c-4c28-8090-37064808cb20
url                    http://localhost:8082
username               admin
version                7.55.2
```

* Create a role (scope for artifactory >= 7.21.1)

```sh
vault write artifactory/roles/jenkins \
    scope="applied-permissions/groups:automation " \
    default_ttl=1h max_ttl=3h
```

Also supports `grant_type=[Optional, default: "client_credentials"]`, and `audience=[Optional, default: *@*]` see [JFrog documentation][artifactory-create-token].

NOTE: By default, the username will be generated automatically using the template `v-(RoleName)-(random 8)` (i.e. `v-jenkins-x4mohTA8`). If you would prefer to have a static username (the same for every token), you can set `username=whatever-you-want`, but keep in mind that in a dynamic environment, someone or something using an old, expired token might cause a denial of service (too many failed logins) against users with the correct token.

<details>
<summary>CLICK for: Create a Role (scope for artifactory < 7.21.1)</summary>

```sh
vault write artifactory/roles/jenkins \
    username="example-service-jenkins" \
    scope="api:* member-of-groups:ci-server" \
    default_ttl=1h max_ttl=3h
```

</details>

> **Note**
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
access_token       eyJ2ZXIiOiIyIiw....
role               jenkins
scope              applied-permissions/groups:automation
token_id           06d962b2-63e2-4279-a25d-d2a9cab6507f
username           v-jenkins-x4mohTA8
```

## Development

### Testing Locally

If you're compiling this yourself and want to test locally, you will need a working docker environment. You will also need vault and golang installed, then you can follow the steps below.

* In first terminal, build the plugin and start the local dev server:

```sh
make
```

* In another terminal, setup a test artifactory instance.

```sh
make artifactory
```

* In the same terminal, setup artifactory-secrets-engine in vault with values:

```sh
export VAULT_ADDR=http://localhost:8200
export VAULT_token=root
make setup
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

----------------------------------------------------------------

#### Start Vault dev server

```sh
make start
```

----------------------------------------------------------------

#### Export Vault url and token

```sh
export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN=root
```

----------------------------------------------------------------

#### Build plugin binary

```sh
make build
```

----------------------------------------------------------------

#### Upgrade plugin binary

To build and upgrade the plugin without having to reconfigure it...

```sh
make upgrade
```

----------------------------------------------------------------

#### Create Test Artifactory

```sh
make artifactory
```

Set `ARTIFACTORY_VERSION` to a [specific self hosted version][artifactory-release-notes] to override the default.

Example:

```sh
make artifactory ARTIFACTORY_VERSION=7.49.10
```

NOTE: If you get a message like:

```console
make: Nothing to be done for `artifactory'.
```

This simply means that "make" thinks artifactory is already running due to the existence of the `./vault/artifactory.env` file.
If you want to run a different version, first use `make stop_artifactory`. If you stopped artifactory using other means (docker), then `rm vault/artifactory.env` manually.

----------------------------------------------------------------

#### Enable artifactory-secrets plugin

```sh
make enable
```

----------------------------------------------------------------

#### Disable plugin (unmount from vault)

```sh
make disable
```

NOTE: This is a good idea before stopping artifactory, especially if you plan to change versions of artifactory. Alternatively, just exit vault (Ctrl+c), and it will go back to default state.

----------------------------------------------------------------

#### Get ADMIN Artifactory token and write it to vault

```sh
make admin
```

NOTE: This following might be some useful environment variables:

> * `ARTIFACTORY_URL`
> * `ARTIFACTORY_USERNAME`
> * `ARTIFACTORY_PASSWORD`

For example:

```sh
ARTIFACTORY_URL=https://artifactory.example.org ARTIFACTORY_USERNAME=tommy ARTIFACTORY_PASSWORD='SuperSecret' make admin
```

If you already have a JFROG_ACCESS_TOKEN, you can skip straight to that too:

```sh
export ARTIFACTORY_URL=https://artifactory.example.com
export JFROG_ACCESS_TOKEN=(PASTE YOUR JFROG ADMIN TOKEN)
make admin
```

----------------------------------------------------------------

* Setup a "test" role, bound to the "readers" group

```sh
make testrole
```

----------------------------------------------------------------

## Issues

* RTFACT-22477 - proposing CIDR restrictions on the created access tokens.
* () - Artifactory 7.39.10 fails to revoke previous token during rotation. Recommend 7.42.1. or higher.

## Contributors

See the [contribution guide](./CONTRIBUTING.md).

## License

Copyright (c) 2023 JFrog.

Apache 2.0 licensed, see [LICENSE][LICENSE] file.

[LICENSE]: ./LICENSE
[artreleases]: https://github.com/jfrog/artifactory-secrets-plugin/releases
[vaultdocplugindir]: https://www.vaultproject.io/docs/configuration/index.html#plugin_directory
[vaultdocplugincatalog]: https://www.vaultproject.io/docs/internals/plugins.html#plugin-catalog
[artifactory-create-token]: https://www.jfrog.com/confluence/display/JFROG/JFrog+Platform+REST+API#JFrogPlatformRESTAPI-CreateToken
[vault-username-templating]: https://developer.hashicorp.com/vault/docs/concepts/username-templating
[artifactory-release-notes]: https://www.jfrog.com/confluence/display/JFROG/Artifactory+Release+Notes
