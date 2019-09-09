#!/bin/bash
# if no args passed to `docker run` or first argument start with `--`, then the user is passing clickhouse-server arguments
if [[ $# -lt 1 ]] || [[ "$1" == "--"* ]]; then
    exec /usr/bin/clickhouse-flamegraph "$@"
fi

# Otherwise, we assume the user want to run his own process, for example a `bash` shell to explore this image
exec "$@"