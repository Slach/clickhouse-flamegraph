#!/usr/bin/env bash
set +x
echo "+ docker login -u XXXX -p XXXX docker.io"
docker login -u ${DOCKER_LOGIN} -p ${DOCKER_PASSWORD} docker.io
set -x

set -xeuo pipefail
 docker buildx build -f docker/clickhouse-flamegraph/Dockerfile --platform=linux/amd64,linux/arm64 --progress plain --pull --push --tag clickhousepro/clickhouse-flamegraph:latest .

echo "docker publishing done"
