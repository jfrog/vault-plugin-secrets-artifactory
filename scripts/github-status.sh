#!/usr/bin/env bash
set -eo pipefail

token=${1}
repo_path=${2}
sha=${3}

curl -L \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${token}"\
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/${repo_path}/statuses/${sha} \
  -d '{"state":"'"${STATE}"'","target_url":"'"${step_url}"'","description":"'"${DESCRIPTION}"'","context":"continuous-integration/jfrog-pipelines"}'