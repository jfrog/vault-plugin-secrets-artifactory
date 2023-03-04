#!/usr/bin/env bash
# Generate an admin access token
# Heavily borrowed from: https://github.com/jfrog/terraform-provider-artifactory/blob/master/scripts/get-access-key.sh

set -eu

function getAccessKey() {
  url=${1:-http://localhost:8082}
  USERNAME=${USERNAME:-admin}
  PASSWORD=${PASSWORD:-password}
  TOKEN_USERNAME=${TOKEN_USERNAME:-admin}
  echo "### Generate Admin Access Key ###" > /dev/stderr

  local cookies
  cookies=$(curl -s -c - "${url}/ui/api/v1/ui/auth/login?_spring_security_remember_me=false" \
                --header "accept: application/json, text/plain, */*" \
                --header "content-type: application/json;charset=UTF-8" \
                --header "x-requested-with: XMLHttpRequest" \
                -d '{"user":"'$USERNAME'","password":"'$PASSWORD'","type":"login"}' | grep TOKEN)

  local refresh_token
  refresh_token=$(echo "${cookies}" | grep REFRESHTOKEN | awk '{print $7 }')

  local access_token
  access_token=$(echo "${cookies}" | grep ACCESSTOKEN | awk '{print $7 }')

  local access_key
  local scoped_access_key
  access_key=$(curl -s -g --request GET "${url}/ui/api/v1/system/security/token?services[]=all" \
                --header "accept: application/json, text/plain, */*" \
                --header "x-requested-with: XMLHttpRequest" \
                --header "cookie: ACCESSTOKEN=${access_token}; REFRESHTOKEN=${refresh_token}")

  curl -s --location --request POST "${url}/access/api/v1/tokens" \
    --header "Authorization: Bearer ${access_key}" \
    --header "Content-Type: application/x-www-form-urlencoded" \
    --data-urlencode "expires_in=3600" \
    --data-urlencode "username=${TOKEN_USERNAME}" \
    --data-urlencode "scope=applied-permissions/admin" \
    --data-urlencode "description=Created_with_script_during_provisioning" | jq -er .access_token
  }

getAccessKey $*
