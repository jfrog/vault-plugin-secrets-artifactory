#!/usr/bin/env bash
set -e

TOKEN=${1}
REPO_PATH=${2}
SHA=${3}

curl -L \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${TOKEN}"\
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/${REPO_PATH}/statuses/${SHA} \
  -d '{"state":"'"${STATE}"'","target_url":"'"${step_url}"'","description":"'"${DESCRIPTION}"'","context":"continuous-integration/jfrog-pipelines"}'