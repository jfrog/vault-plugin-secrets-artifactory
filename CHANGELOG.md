## 0.2.2 (January 24, 2023)

BUG FIXES:

- Fix HTTP response body not closed before root certificate error is returned. PR: [#26](https://github.com/jfrog/artifactory-secrets-plugin/pull/28)

## 0.2.1 (January 11, 2023)

BUG FIXES:

- Fix HTTP response body not closed, thus leading to memory leak. PR: [#26](https://github.com/jfrog/artifactory-secrets-plugin/pull/26)

## 0.2.0 (November 30, 2022)

IMPROVEMENTS:

- Add support for rotating Artifactory admin token. Issue: [#14](https://github.com/jfrog/artifactory-secrets-plugin/issues/14) PR: [#17](https://github.com/jfrog/artifactory-secrets-plugin/pull/17)
