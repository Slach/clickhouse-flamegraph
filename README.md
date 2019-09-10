# ClickHouse flamegraph
command line utility for visualizing clickhouse system.trace_log as flamegraph, 
based on https://gist.github.com/alexey-milovidov/92758583dd41c24c360fdb8d6a4da194

![Output example](docs/clickhouse-flamegraph.png?raw=1 "example SVG")
## Installation
todo

## Usage
```
USAGE:
   clickhouse-flamegraph [global options] command [command options] [arguments...]

GLOBAL OPTIONS:
   --width value                               width of image (default 1200) (default: 1200)
   --height value                              height of each frame (default 16) (default: 16)
   --flamegraph-script value                   path of flamegraph.pl. if not given, find the script from $PATH [$CH_FLAME_FLAMEGRAPH_SCRIPT]
   --output-dir value, -o value                distination path of grenerated flamegraphs files. default is ./clickouse-flamegraphs/ (default: "./clickouse-flamegraphs/") [$CH_FLAME_OUTPUT_DIR]
   --date-from value, --from value             filter system.trace_log from date in any parsable format, see https://github.com/araddon/dateparse (default: current time - 5 min) [$CH_FLAME_DATE_FROM]
   --date-to value, --to value                 filter system.trace_log to date in any parsable format, see https://github.com/araddon/dateparse (default: current time) [$CH_FLAME_DATE_TO]
   --query-filter value, --query-regexp value  filter system.query_log by any regexp, see https://github.com/google/re2/wiki/Syntax [$CH_FLAME_QUERY_FILTER]
   --clickhouse-dsn value, --dsn value         clickhouse connection string, see https://github.com/kshvakov/clickhouse#dsn (default: "tcp://localhost:9000?database=default") [$CH_FLAME_CLICKHOUSE_DSN]
   --output-format value, --format value       accept values: txt (see https://github.com/brendangregg/FlameGraph#2-fold-stacks), json (see https://github.com/spiermar/d3-flame-graph/#input-format,  (default: "txt") [$CH_FLAME_OUTPUT_FORMAT]
   --debug, --verbose                          show debug log [$CH_FLAME_DEBUG]
   --console                                   output logs to console format instead of json [$CH_FLAME_LOG_TO_CONSOLE]
   --help, -h                                  show help
   --version, -v                               print the version
```                         

## TODO
- implement json format and webhooks
- try implement interactive dashboard with http://dash.plot.ly
- try integrate with https://github.com/samber/grafana-flamegraph-panel
