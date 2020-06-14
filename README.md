![Build](https://github.com/idcmp/artifactory-secrets-plugin/workflows/Build/badge.svg)

# Vault Artifactory Secrets Plugin

This is a [HashiCorp Vault](https://www.vaultproject.io/) plugin which talks to JFrog Artifactory server (5.0.0 or later) and will
dynamically provision access tokens with specified scopes. This backend can be mounted multiple times
to provide access to multiple Artifactory servers.

Using this plugin, you limit the accidental exposure window of Artifactory tokens; useful for continuous
integration servers.

## Usage

```bash
$ vault secrets enable artifactory

# Also supports max_ttl= and default_ttl=
$ vault write artifactory/config/admin \
               url=https://artifactory.example.org \
               access_token=0ab31978246345871028973fbcdeabcfadecbadef

# Also supports grant_type=, and audience= (see JFrog documentation)
$ vault write artifactory/roles/jenkins \
               username="example-service-jenkins" \
               scope="api:* member-of-groups:ci-server" \
               refreshable=true \
               default_ttl=1h max_ttl=3h 

$ vault list artifactory/roles
Keys
----
jenkins

$ vault write -force artifactory/token/jenkins 
Key                Value
---                -----
lease_id           artifactory/token/jenkins/25jYH8DjUU548323zPWiSakh
access_token       adsdgbtybbeeyh...
refreshable        true
role               jenkins
scope              api:* member-of-groups:ci-server
```

