#!/usr/bin/env bash
set -xeuo pipefail
docker login -u ${DOCKER_LOGIN} docker.io
docker-compose build clickhouse-flamegraph
docker-compose push clickhouse-flamegraph
echo "docker publishing done"
