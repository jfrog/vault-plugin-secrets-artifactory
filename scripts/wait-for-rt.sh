#!/usr/bin/env bash

function waitForArtifactory() {
  local url=${1?You must supply the artifactory url}
  local url_ui=${2?You must supply the artifactory UI url}
  echo "### Wait for Artifactory to start ###" > /dev/stderr

  until curl -sf -o /dev/null -u admin:password ${url}/artifactory/api/system/ping/; do
      printf '.' > /dev/stderr
      sleep 4
  done
  echo "" > /dev/stderr

  echo "### Waiting for Artifactory UI to start ###" > /dev/stderr
  until curl -sf -o /dev/null -u admin:password ${url_ui}/ui/login/; do
      printf '.' > /dev/stderr
      sleep 4
  done
  echo "" > /dev/stderr
}
