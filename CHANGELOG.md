## 0.2.15 (April 18, 2023)

IMPROVEMENTS:

- Fix empty strings for optional attributes when reading roles. PR: [#66](https://github.com/jfrog/artifactory-secrets-plugin/pull/66)
- Fix inconsistent use of env vars for acceptance tests. PR: [#71](https://github.com/jfrog/artifactory-secrets-plugin/pull/71)

## 0.2.14 (April 18, 2023)

IMPROVEMENTS:

- Upgrade dependencies to latest version.
- Update Go minimum version to 1.18 (which we have been using for a while now).

PR: [#65](https://github.com/jfrog/artifactory-secrets-plugin/pull/65)

## 0.2.13 (March 30, 2023)

IMPROVEMENTS:

- Sign release checksums file with GPG key. Release also include public key for signature verification.

PR: [#54](https://github.com/jfrog/artifactory-secrets-plugin/pull/54)

## 0.2.12 (March 23, 2023)

IMPROVEMENTS:

- Plugin now reports its version to Vault server. You can see it with `vault plugin list` command.
- Remove version number from the binary file name (now `artifactory-secrets-plugin`, vs `artifactory-secrets-plugin_v0.2.6`) now that it registers as 'versioned' plugin with Vault server.
- Update README on how to register plugin to reflect this change of binary name.
- Update Makefile to use GoRelease (same as GitHub Action) to build binary for development process.

PR: [#53](https://github.com/jfrog/artifactory-secrets-plugin/pull/53)

## 0.2.11 (March 20, 2023)

IMPROVEMENTS:

- Switch to using POSTing JSON (instead of form) when creating token.
- `expires_in` and `force_revocable` fields are now opt-in.

Issue: [#50](https://github.com/jfrog/artifactory-secrets-plugin/pull/50) PR: [#52](https://github.com/jfrog/artifactory-secrets-plugin/pull/52)

## 0.2.10 (March 13, 2023)

BUG FIXES:

- Temporarily disable `force_revocable` due to revoke token failing. Issue: [#50](https://github.com/jfrog/artifactory-secrets-plugin/pull/50) PR: [#51](https://github.com/jfrog/artifactory-secrets-plugin/pull/51)

## 0.2.9 (March 13, 2023)

IMPROVEMENTS:

- Add support for [Vault Username Templating](https://developer.hashicorp.com/vault/docs/concepts/username-templating).
- Improve README.md
- Update Vault API and SDK packages to latest version.

PR: [#47](https://github.com/jfrog/artifactory-secrets-plugin/pull/47)

## 0.2.8 (March 13, 2023)

IMPROVEMENTS:

- Add support for `force_revocable` flag available in Artifactory 7.50.3+. PR: [#45](https://github.com/jfrog/artifactory-secrets-plugin/pull/45)

## 0.2.7 (February 27, 2023)

BUG FIXES:

- Fix revoke token error check only for HTTP status code 200. Now it errors only for status code >= 400. Also include token ID in logs and error message. PR: [#41](https://github.com/jfrog/artifactory-secrets-plugin/pull/41)

## 0.2.6 (February 23, 2023)

IMPROVEMENTS:

- Include additional token information when reading from config. PR: [#39](https://github.com/jfrog/artifactory-secrets-plugin/pull/39)

## 0.2.5 (February 22, 2023)

IMPROVEMENTS:

- Use username from current token for new token during rotation. PR: [#34](https://github.com/jfrog/artifactory-secrets-plugin/pull/34)
- Add env vars to make command `make setup` works. PR: [#37](https://github.com/jfrog/artifactory-secrets-plugin/pull/37)

## 0.2.4 (February 21, 2023)

IMPROVEMENTS:

- Update `golang.org/x/net` and `golang.org/x/crypto` modules to latest version. PR: [#32](https://github.com/jfrog/artifactory-secrets-plugin/pull/32) Dependabot alerts: [1](https://github.com/jfrog/artifactory-secrets-plugin/security/dependabot/1), [2](https://github.com/jfrog/artifactory-secrets-plugin/security/dependabot/2), [3](https://github.com/jfrog/artifactory-secrets-plugin/security/dependabot/3), [4](https://github.com/jfrog/artifactory-secrets-plugin/security/dependabot/4)

## 0.2.3 (January 31, 2023)

BUG FIXES:

- Fix breakage introduced in 0.2.0 where default port fallback was incorrectly handled. PR: [#29](https://github.com/jfrog/artifactory-secrets-plugin/pull/29)

## 0.2.2 (January 24, 2023)

BUG FIXES:

- Fix HTTP response body not closed before root certificate error is returned. PR: [#28](https://github.com/jfrog/artifactory-secrets-plugin/pull/28)

## 0.2.1 (January 11, 2023)

BUG FIXES:

- Fix HTTP response body not closed, thus leading to memory leak. PR: [#26](https://github.com/jfrog/artifactory-secrets-plugin/pull/26)

## 0.2.0 (November 30, 2022)

IMPROVEMENTS:

- Add support for rotating Artifactory admin token. Issue: [#14](https://github.com/jfrog/artifactory-secrets-plugin/issues/14) PR: [#17](https://github.com/jfrog/artifactory-secrets-plugin/pull/17)
