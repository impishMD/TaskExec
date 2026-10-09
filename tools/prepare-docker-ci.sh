#!/usr/bin/env bash
set -euo pipefail

# GitHub-hosted runners include shared Docker Hub credentials. If their token
# service is unavailable, public images can still be pulled anonymously.
# Never change a developer's local Docker credentials.
if [[ "${GITHUB_ACTIONS:-}" != true ]]; then
  echo 'This helper is only intended for GitHub Actions runners.' >&2
  exit 1
fi

image=alpine:3.24
if timeout 90 docker pull "$image"; then
  exit 0
fi

echo '::warning::Docker Hub pull failed with runner credentials; retrying public images anonymously.'
docker logout docker.io
for attempt in 1 2 3; do
  if timeout 90 docker pull "$image"; then
    exit 0
  fi
  if [[ "$attempt" != 3 ]]; then
    sleep "$((attempt * 5))"
  fi
done
echo '::error::Docker Hub is unavailable for anonymous pulls too.'
exit 1
