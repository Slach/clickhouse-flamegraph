#!/usr/bin/env bash
set +x
echo "+ docker login -u XXXX -p XXXX docker.io"
docker login -u ${DOCKER_LOGIN} -p ${DOCKER_PASSWORD} docker.io
set -x

set -xeuo pipefail
docker compose build --pull clickhouse-flamegraph
docker compose push clickhouse-flamegraph
echo "docker publishing done"
