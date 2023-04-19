#!/bin/bash
set -e

############################################################################
# shell script for getting an admin access token from Artifactory          #
# NOTE: This shell script uses undocumented "UI" API Access                #
#                                                                          #
# Author: Tommy McNeely <Tommy.McNeely@davita.com>                         #
#                                                                          #
# WARNING: This script is designed as a test script!                       #
# Use otherwise at your own peril!                                         #
#                                                                          #
# Globals (env variables)                                                  #
#     JFROG_URL                - artifactory base url                      #
#     ARTIFACTORY_USERNAME     - artifactory admin username                #
#     ARTIFACTORY_PASSWORD     - artifactory admin password                #
#     TOKEN_USERNAME           - generated token username                  #
#     TOKEN_EXPIRY             - token expiration in hours                 #
#     TOKEN_DESCRIPTION        - token description (limit 1024)            #
############################################################################

# defaulted variables
JFROG_URL="${JFROG_URL:-http://localhost:8082}"
ARTIFACTORY_USERNAME="${ARTIFACTORY_USERNAME:-admin}"
ARTIFACTORY_PASSWORD="${ARTIFACTORY_PASSWORD:-password}"
TOKEN_USERNAME="${TOKEN_USERNAME:-admin-$(date '+%Y-%m-%d-%H%M%S')}"
TOKEN_DESCRIPTION="${TOKEN_DESCRIPTION:-generated with getArtifactoryAdminToken.sh}"
TOKEN_EXPIRY="${EXPIRY:-8}" # By default, token expiration under 6h are not revocable.

# login function
login() {
    curl "${JFROG_URL}/ui/api/v1/ui/auth/login" \
    --fail \
    --silent \
    --show-error \
    --location \
    --cookie-jar - \
    --header 'Accept: application/json, text/plain, */*' \
    --header 'Accept-Encoding: gzip, deflate, br' \
    --header 'Content-Type: application/json' \
    --header 'X-Requested-With: XMLHttpRequest' \
    --data-raw "{\"user\":\"${ARTIFACTORY_USERNAME}\",\"password\":\"${ARTIFACTORY_PASSWORD}\",\"type\":\"login\"}"
}

# function to get admin access token
getToken() {
    curl "${JFROG_URL}/ui/api/v1/access/token/scoped" \
    --request GET \
    --get \
    --fail \
    --silent \
    --show-error \
    --location \
    --globoff \
    --header 'Accept: application/json, text/plain, */*' \
    --header 'Accept-Encoding: gzip, deflate, br' \
    --header 'X-Requested-With: XMLHttpRequest' \
    --header 'Connection: keep-alive' \
    --header "cookie: ACCESSTOKEN=${cookie_access_token}; REFRESHTOKEN=${cookie_refresh_token}" \
    --data-urlencode "expiry=${TOKEN_EXPIRY}" \
    --data-urlencode "services[]=all" \
    --data-urlencode "scope=applied-permissions/admin" \
    --data-urlencode "username=${TOKEN_USERNAME}" \
    --data-urlencode "description=${TOKEN_DESCRIPTION}"
}

# login to artifactory
echo "Logging in to Artifactory (${JFROG_URL}) as ${ARTIFACTORY_USERNAME} ..." >&2
cookies=$(login) >&2 || {
    echo "Failed to login to Artifactory" >&2
    exit 1
}

# Parse Login Cookies
cookie_refresh_token=$(echo "${cookies}" | grep REFRESHTOKEN | awk '{print $7 }')
cookie_access_token=$(echo "${cookies}" | grep ACCESSTOKEN | awk '{print $7 }')

# set variable to output from getToken function call
echo "Generating artifactory admin access token." >&2
payload=$(getToken) || {
    echo "Failed to get admin access token" >&2
    exit 1
}


access_token=$(echo "$payload" | jq -r '.access_token') || {
    echo "Failed to parse access_token" >&2
    exit 1
}

echo $access_token
