# ClickHouse flamegraph
command line utility for visualizing clickhouse system.trace_log as flamegraph, 
thanks https://gist.github.com/alexey-milovidov/92758583dd41c24c360fdb8d6a4da194 for original idea

![Output example](docs/clickhouse-flamegraph.png?raw=1 "example SVG")

### Prepare Clickhouse server
- install clickhouse-server package version 19.14 or higher as described in [documentation](https://clickhouse.yandex/#quick-start)
- install clickhouse-common-static-dbg package
- enable query_log and sampling profiling in settings on each server in your cluster for example add following files:
####  /etc/clickhouse-server/users.d/profiling.xml
```xml
<yandex>
    <profiles>
        <!-- see details about profile name https://clickhouse.yandex/docs/en/operations/settings/settings_profiles/ and https://clickhouse.yandex/docs/en/operations/server_settings/settings/#default-profile -->
        <default>
            <log_queries>1</log_queries>
            <allow_introspection_functions>1</allow_introspection_functions>
            <!-- cluster wide 25 times per second sampling profiler -->
            <query_profiler_real_time_period_ns>40000000</query_profiler_real_time_period_ns>
            <query_profiler_cpu_time_period_ns>40000000</query_profiler_cpu_time_period_ns>
        </default>
    </profiles>
</yandex>
```

## Installation
currently clickhouse-flamegraph required perl and flamegraph.pl for correct work, you can just download latest packages from  https://github.com/Slach/clickhouse-flamegraph/releases

### Simplest way (but you should skip it if you case about security)
```bash
curl -sL https://raw.githubusercontent.com/Slach/clickhouse-flamegraph/master/install.sh | sudo bash
```

### Linux (deb or rpm based distributive, amd64 architecture)
```bash
PKG_MANAGER=$(command -v dpkg || command -v rpm)
PKG_EXT=$(if [[ "${PKG_MANAGER}" == "/usr/bin/dpkg" ]]; then echo "deb"; else echo "rpm"; fi)
cd $TEMP
echo "$(curl -sL https://github.com/Slach/clickhouse-flamegraph/releases/latest | grep href | grep -E "\\.rpm|\\.deb|\\.txt" | cut -d '"' -f 2)" | sed -e "s/^\\/Slach/https:\\/\\/github.com\\/Slach/" | wget -nv -c -i -
grep $PKG_EXT clickhouse-flamegraph_checksums.txt | sha256sum
${PKG_MANAGER} -i clickhouse-flamegraph*.${PKG_EXT}
```

### MacOS (64bit)
```bash
brew install wget
cd $TEMP
echo "$(curl -sL https://github.com/Slach/clickhouse-flamegraph/releases/latest | grep href | grep -E "darwin_amd64\\.tar\\.gz|\\.txt" | cut -d '"' -f 2)" | sed -e "s/^\\/Slach/https:\\/\\/github.com\\/Slach/" | wget -nv -c -i -
grep darwin_amd64.tar.gz clickhouse-flamegraph_checksums.txt | sha256sum
tar -xvfz -C /usr/bin clickhouse-flamegraph*darwin_amd64.tar.gz
```

### Windows (64bit) 
install CYGWIN https://cygwin.com/install.html 
from setup.exe install following packages:
  - wget
  - sha256sum
  - bash
run following script

```bash
cd $TEMP
echo "$(curl -sL https://github.com/Slach/clickhouse-flamegraph/releases/latest | grep href | grep -E "windows_amd64\\.tar\\.gz|\\.txt" | cut -d '"' -f 2)" | sed -e "s/^\\/Slach/https:\\/\\/github.com\\/Slach/" | wget -nv -c -i -
grep windows_amd64.tar.gz clickhouse-flamegraph_checksums.txt | sha256sum
tar -xvfz -C /usr/bin clickhouse-flamegraph*windows_amd64.tar.gz
```

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
   --output-format value, --format value       accept values: svg, txt (see https://github.com/brendangregg/FlameGraph#2-fold-stacks), json (see https://github.com/spiermar/d3-flame-graph/#input-format,  (default: "txt") [$CH_FLAME_OUTPUT_FORMAT]
   --debug, --verbose                          show debug log [$CH_FLAME_DEBUG]
   --console                                   output logs to console format instead of json [$CH_FLAME_LOG_TO_CONSOLE]
   --help, -h                                  show help
   --version, -v                               print the version
```                         

## TODO
- implement json format and webhooks
- try implement interactive dashboard with http://dash.plot.ly
- try integrate with https://github.com/samber/grafana-flamegraph-panel
