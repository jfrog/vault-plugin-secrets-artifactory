#!/usr/bin/env bash

vault write artifactory/config/admin url=$JFROG_URL use_expiring_tokens=true max_ttl=14400 default_ttl=3600

USER_TOKEN=$(curl -s -L "${JFROG_URL}/access/api/v1/tokens" -H 'Content-Type: application/json' -H "Authorization: Bearer ${JFROG_ACCESS_TOKEN}" --data-raw '{"grant_type":"client_credentials","username":"admin","scope":"applied-permissions/user applied-permissions/admin","refreshable":true,"audience":"*@*","expires_in":60,"force_revocable":false,"include_reference_token":false}')

USER_ACCESS_TOKEN=$(echo ${USER_TOKEN} | jq -r ".access_token")
echo "USER_ACCESS_TOKEN: ${USER_ACCESS_TOKEN}"

USER_REFRESH_TOKEN=$(echo ${USER_TOKEN} | jq -r ".refresh_token")
echo "USER_REFRESH_TOKEN: ${USER_REFRESH_TOKEN}"

vault write artifactory/config/user_token access_token=${USER_ACCESS_TOKEN} refresh_token=${USER_REFRESH_TOKEN} refreshable=true use_expiring_tokens=true max_ttl=14400 default_ttl=3600
vault read artifactory/config/user_token
vault read artifactory/user_token/test

date