# Local testing reference

Commands for Phase 4 of `release-version-bump`.

## Quick path (Makefile)

Terminal A:

```bash
export PATH="$HOME/bin:$PATH"
make          # fmt + build + unit tests + vault -dev
```

Terminal B:

```bash
export PATH="$HOME/bin:$PATH"
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=root

make artifactory
make setup
make admin
make usertoken    # uses user_token/test — needs Artifactory user `test`, or use admin manually
make testrole
```

Cleanup:

```bash
make stop_artifactory
# Ctrl+C Vault in Terminal A, or:
pkill -f 'vault server -dev'
```

## Manual path when Makefile token fetch hangs

1. Build plugin: `make build`
2. Start Artifactory: `make artifactory` → source `vault/artifactory.env`
3. Start Vault:

```bash
PLUGIN_DIR="$PWD/dist/vault-plugin-secrets-artifactory_$(go env GOOS)_$(go env GOARCH)_*"
# resolve exact dir:
PLUGIN_DIR=$(echo dist/vault-plugin-secrets-artifactory_*/ | head -1 | sed 's:/*$::')
vault server -dev -dev-root-token-id=root -dev-plugin-dir="$PLUGIN_DIR" -log-level=INFO
```

4. Register (version must match binary self-report, often from goreleaser tag):

```bash
export VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=root
SHA=$(shasum -a 256 "$PLUGIN_DIR/artifactory-secrets-plugin" | awk '{print $1}')
VER=<version-from-build>   # e.g. 1.8.10-dev+<hash>
vault plugin register -sha256="$SHA" -command=artifactory-secrets-plugin -version="$VER" secret artifactory
vault secrets enable -path=artifactory -plugin-version="$VER" artifactory
```

5. Access admin token from **inside** the container when host UI login times out:

```bash
CID=$(docker ps --format '{{.ID}} {{.Image}}' | awk '/artifactory/ {print $1; exit}')
# Login + scoped token via UI APIs on 127.0.0.1:8082 inside the container
# Keep any helper script untracked / delete after use
```

6. Smoke:

```bash
JFROG_URL=http://127.0.0.1:8082
vault write artifactory/config/admin url="$JFROG_URL" access_token="$JFROG_ACCESS_TOKEN"
vault read artifactory/config/admin
vault write -f artifactory/config/rotate
vault write artifactory/roles/test scope="applied-permissions/groups:readers" max_ttl=3h default_ttl=2h
vault read artifactory/token/test
vault read artifactory/user_token/admin   # existing user
```

7. Issued role token should `ping` Artifactory:

```bash
AT=$(vault read -field=access_token artifactory/token/test)
curl -sf -H "Authorization: Bearer $AT" "$JFROG_URL/artifactory/api/system/ping/"
```

## Interpreting failures

| Symptom | Likely cause |
|---------|----------------|
| `failed to create new token` on `user_token/test` | User `test` missing in Artifactory |
| `Invalid token, audience` | Using legacy Artifactory token instead of Access JWT |
| `checksums did not match` on reload | Rebuild changed binary; remount or restart Vault |
| UI login hang from host | Use in-container UI login; wait for frontend readiness |
| `go get -u all` fails on `armon/go-metrics` | Use Hashicorp `go-metrics` upgrades; avoid `all` |
