#!/usr/bin/env bash
set -xeuo pipefail
if [[ $# -lt 1 ]]; then
    echo "release.sh [major|minor|patch]"
    exit 1
fi
echo 1 > /proc/sys/vm/drop_caches
source .release_env
git config core.eol lf
git config core.autocrlf input
git config user.name "$GITHUB_LOGIN"
git config user.email "$GITHUB_EMAIL"
bump2version --verbose $1
goreleaser
bash -x ./docker/docker-publisher.sh
git push origin master