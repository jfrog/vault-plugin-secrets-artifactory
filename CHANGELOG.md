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
