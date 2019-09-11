#!/usr/bin/env bash
set -xeuo pipefail

docker login -u ${DOCKER_LOGIN} cloud.canister.io:5000
docker-compose build clickhouse-flamegraph
docker-compose up -d clickhouse-flamegraph
sleep 1
WAIT="0"
while [[ $(docker-compose ps | grep clickhouse-flamegraph | wc -l) = "0" ]]; do
	echo "wait clickhouse-flamegraph running..."
	sleep 1
	WAIT=$[$WAIT + 1]
	if ["$WAIT" > "20"]; then
		echo "TIMEOUT"
		echo "docker publishing failed"
		exit 1
	fi
done
if [[ $(docker-compose ps | grep clickhouse-flamegraph | grep Exit | wc -l) != "0" ]]; then
	docker-compose logs clickhouse-flamegraph
	exit 1
fi

docker-compose down
docker-compose push clickhouse-flamegraph
echo "docker publishing done"
