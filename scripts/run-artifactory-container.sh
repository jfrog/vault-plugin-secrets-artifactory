#!/usr/bin/env bash
## Heavily borrowed from: https://github.com/jfrog/terraform-provider-artifactory/tree/master/scripts
set -eo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" > /dev/null && pwd )"
source "${SCRIPT_DIR}/wait-for-rt.sh"

ARTIFACTORY_REPO="${ARTIFACTORY_REPO:-releases-docker.jfrog.io/jfrog}"
ARTIFACTORY_IMAGE="${ARTIFACTORY_IMAGE:-artifactory-jcr}"
ARTIFACTORY_VERSION=${ARTIFACTORY_VERSION:-$(awk -F: '/FROM/ {print $2}' ${SCRIPT_DIR}/Dockerfile)}

# Get actual version for latest
if [ $ARTIFACTORY_VERSION == "latest" ]; then
  # The code below should work, but JFrog's docker repo is not reporting all the tags properly
  # REPO_HOST=$(echo $ARTIFACTORY_REPO | cut -d/ -f1)
  # REPO_PATH=$(echo $ARTIFACTORY_REPO | cut -d/ -f2-)
  # curl -u anonymous: -sS "https://${REPO_HOST}/v2/${REPO_PATH}/${ARTIFACTORY_IMAGE}/tags/list?page_size=1000"
  # exit 1
  # ARTIFACTORY_VERSION=$(curl -u anonymous: -sS "https://${REPO_HOST}/v2/${REPO_PATH}/${ARTIFACTORY_IMAGE}/tags/list?page_size=1000" \
  #   | jq -er '.tags | map(select(. | test("^[0-9.]+"))) | sort_by(values | split(".") | map(tonumber)) | last')
  
  # Instead, lets parse the response from artifactory
  # Set the URL of the JFrog Artifactory self-hosted repository
  URL="https://releases.jfrog.io/artifactory/artifactory-pro/org/artifactory/pro/docker/jfrog-artifactory-pro/"
  # Get the list of available versions
  VERSIONS=$(curl -s ${URL} | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
  # Get the latest version
  ARTIFACTORY_VERSION=$(echo "${VERSIONS}" | sort -t. -k1,1n -k2,2n -k3,3n | tail -n1)
fi

echo "ARTIFACTORY_IMAGE=${ARTIFACTORY_IMAGE}" > /dev/stderr
echo "ARTIFACTORY_VERSION=${ARTIFACTORY_VERSION}" > /dev/stderr

if [ -f "${SCRIPT_DIR}/artifactory.lic" ]; then
  ARTIFACTORY_LICENSE="-v \"${SCRIPT_DIR}/artifactory.lic:/artifactory_extra_conf/artifactory.lic:ro\""
else
  ARTIFACTORY_LICENSE=""
fi

set -euf

CONTAINER_ID=$(docker run -i -t -d --rm ${ARTIFACTORY_LICENSE} -p8081:8081 -p8082:8082 -p8080:8080 \
  "${ARTIFACTORY_REPO}/${ARTIFACTORY_IMAGE}:${ARTIFACTORY_VERSION}")

export ARTIFACTORY_URL=http://localhost:8081
export ARTIFACTORY_UI_URL=http://localhost:8082

# Wait for Artifactory to start
waitForArtifactory "${ARTIFACTORY_URL}" "${ARTIFACTORY_UI_URL}"

# With this trick you can do $(./run-artifactory-container.sh) and it will directly be setup for you without the terminal output
echo "export ARTIFACTORY_CONTAINER_ID=${CONTAINER_ID}"
echo "export ARTIFACTORY_URL=\"${ARTIFACTORY_UI_URL}\""
