package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/mailru/go-clickhouse/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "clickhouse-flamegraph"
	app.Usage = "visualize clickhouse system.trace_log as flamegraph, based on https://gist.github.com/alexey-milovidov/92758583dd41c24c360fdb8d6a4da194"
	app.ArgsUsage = ""
	app.HideHelp = false
	app.Version = "2023.0.1"
	app.Flags = []cli.Flag{
		&cli.IntFlag{
			Name:  "width",
			Value: 1200,
			Usage: "width of image (default 1200)",
		},
		&cli.IntFlag{
			Name:  "height",
			Value: 16,
			Usage: "height of each frame (default 16)",
		},
		&cli.StringFlag{
			Name:    "flamegraph-script",
			EnvVars: []string{"CH_FLAME_FLAMEGRAPH_SCRIPT"},
			Value:   "flamegraph.pl",
			Usage:   "path to script which run SVG flamegraph generation, can be passed without full path, will try to find the script from $PATH",
		},
		&cli.StringFlag{
			Name:    "output-dir",
			Aliases: []string{"o"},
			Value:   "./clickhouse-flamegraphs/",
			EnvVars: []string{"CH_FLAME_OUTPUT_DIR"},
			Usage:   "destination path of generated flamegraphs files",
		},
		&cli.StringFlag{
			Name:    "date-from",
			Aliases: []string{"from"},
			Usage:   "filter system.trace_log from date in any parsable format (see https://github.com/araddon/dateparse) or time duration (from current time)",
			EnvVars: []string{"CH_FLAME_DATE_FROM"},
			Value:   time.Now().Add(time.Duration(-5) * time.Minute).Format("2006-01-02 15:04:05 -0700"),
		},
		&cli.StringFlag{
			Name:    "date-to",
			Aliases: []string{"to"},
			Usage:   "filter system.trace_log to date in any parsable format or time duration (see https://github.com/araddon/dateparse) or time duration (from current time)",
			EnvVars: []string{"CH_FLAME_DATE_TO"},
			Value:   time.Now().Format("2006-01-02 15:04:05 -0700"),
		},
		&cli.StringFlag{
			Name:    "query-filter",
			Aliases: []string{"query-regexp"},
			Usage:   "filter system.query_log by any regexp, see https://github.com/google/re2/wiki/Syntax",
			EnvVars: []string{"CH_FLAME_QUERY_FILTER"},
			Value:   "",
		},
		&cli.StringSliceFlag{
			Name:    "query-ids",
			Aliases: []string{"query-id"},
			Usage:   "filter system.query_log by query_id field, comma separated list",
			EnvVars: []string{"CH_FLAME_QUERY_IDS"},
			Value:   cli.NewStringSlice(),
		},
		&cli.StringSliceFlag{
			Name:    "trace-types",
			Aliases: []string{"trace-type"},
			Usage:   "filter system.trace_log by trace_type field, comma separated list",
			EnvVars: []string{"CH_FLAME_TRACE_TYPES"},
			Value:   cli.NewStringSlice("Real", "CPU", "Memory", "MemorySample"),
		},
		&cli.StringFlag{
			Name:    "clickhouse-dsn",
			Aliases: []string{"dsn"},
			Usage:   "clickhouse connection string, see https://github.com/mailru/go-clickhouse#dsn",
			EnvVars: []string{"CH_FLAME_CLICKHOUSE_DSN"},
			Value:   "http://localhost:8123/default",
		},
		&cli.StringFlag{
			Name:    "clickhouse-cluster",
			Aliases: []string{"cluster"},
			Usage:   "clickhouse cluster name from system.clusters, all flame graphs will get from cluster() function, see https://clickhouse.com/docs/en/sql-reference/table-functions/cluster",
			EnvVars: []string{"CH_FLAME_CLICKHOUSE_CLUSTER"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "tls-certificate",
			Usage:   "X509 *.cer, *.crt or *.pem file for https connection, use only if tls_config exists in --dsn, see https://clickhouse.com/docs/en/operations/server-configuration-parameters/settings/#server_configuration_parameters-openssl for details",
			EnvVars: []string{"CH_FLAME_TLS_CERT"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "tls-key",
			Usage:   "X509 *.key file for https connection, use only if tls_config exists in --dsn",
			EnvVars: []string{"CH_FLAME_TLS_KEY"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "tls-ca",
			Usage:   "X509 *.cer, *.crt or *.pem file used with https connection for self-signed certificate, use only if tls_config exists in --dsn, see https://clickhouse.com/docs/en/operations/server-configuration-parameters/settings/#server_configuration_parameters-openssl for details",
			EnvVars: []string{"CH_FLAME_TLS_CA"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "output-format",
			Aliases: []string{"format"},
			Usage:   "accept values: svg, txt (see https://github.com/brendangregg/FlameGraph#2-fold-stacks), json (see https://github.com/spiermar/d3-flame-graph/#input-format, ",
			EnvVars: []string{"CH_FLAME_OUTPUT_FORMAT"},
			Value:   "svg",
		},
		&cli.BoolFlag{
			Name:    "normalize-query",
			Aliases: []string{"normalize"},
			Usage:   "group stack by normalized queries, instead of query_id, see https://clickhouse.com/docs/en/sql-reference/functions/string-functions/#normalized-query",
			EnvVars: []string{"CH_FLAME_NORMALIZE_QUERY"},
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"verbose"},
			Usage:   "show debug log",
			EnvVars: []string{"CH_FLAME_DEBUG"},
		},
		&cli.BoolFlag{
			Name:    "console",
			Usage:   "output logs to console format instead of json",
			EnvVars: []string{"CH_FLAME_LOG_TO_CONSOLE"},
		},
	}

	app.Action = run
	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("generation failed")
	}
}

var (
	queryIdSQLTemplate = `
SELECT DISTINCT hostName() AS host_name, {queryField}, {queryIdField} 
FROM {from}
WHERE {where}
`

	traceSQLTemplate = `
SELECT 
	hostName() AS host_name,
    {queryIdField},
	trace_type,
	sum(abs(size)) AS total_size,
	count() AS samples, 
	concat(
		multiIf( 
			position( toString(trace_type), 'Memory') > 0 AND sum(size) >= 0, 'allocate;',
			position( toString(trace_type), 'Memory') > 0 AND sum(size) < 0, 'free;',
			concat( toString(trace_type), ';')
		),
		arrayStringConcat(arrayReverse(arrayMap(x -> concat( demangle(addressToSymbol(x)), '#', addressToLine(x) ), trace)), ';')
	) AS stack
FROM {from}
WHERE {where}
GROUP BY host_name, query_id, trace_type, trace
SETTINGS allow_introspection_functions=1
`
)

func run(c *cli.Context) error {
	stdlog.SetOutput(log.Logger)
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	if c.Bool("verbose") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if c.Bool("console") {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	return generate(c)
}

func parseDate(c *cli.Context, paramName string) time.Time {
	var parsedDate time.Time
	var err error
	if parsedDate, err = dateparse.ParseAny(c.String(paramName)); err != nil {
		if duration, err := time.ParseDuration(c.String(paramName)); err != nil {
			log.Fatal().Err(err).Msgf("invalid %s parameter = %s", paramName, c.String(paramName))
		} else {
			parsedDate = time.Now().Add(-duration)
		}
	}
	return parsedDate
}

func prepareTLSConfig(dsn string, c *cli.Context) {
	if strings.Contains(dsn, "tls_config") {
		cfg, err := clickhouse.ParseDSN(dsn)
		if err != nil {
			log.Fatal().Stack().Err(errors.Wrap(err, "")).Send()
		}
		tlsConfig := &tls.Config{}
		if c.String("tls-ca") != "" {
			CA := x509.NewCertPool()
			severCert, err := ioutil.ReadFile(c.String("tls-ca"))
			if err != nil {
				log.Fatal().Stack().Err(errors.Wrap(err, "")).
					Str("tls-ca", c.String("tls-ca")).
					Str("tls-certificate", c.String("tls-certificate")).
					Str("tls-key", c.String("tls-key")).
					Send()
			}
			CA.AppendCertsFromPEM(severCert)
			tlsConfig.RootCAs = CA
		}
		if c.String("tls-certificate") != "" {
			cert, err := tls.LoadX509KeyPair(c.String("tls-certificate"), c.String("tls-key"))
			if err != nil {
				log.Fatal().Stack().Err(errors.Wrap(err, "")).
					Str("tls-ca", c.String("tls-ca")).
					Str("tls-certificate", c.String("tls-certificate")).
					Str("tls-key", c.String("tls-key")).
					Send()
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		if err := clickhouse.RegisterTLSConfig(cfg.TLSConfig, tlsConfig); err != nil {
			log.Fatal().Stack().Err(errors.Wrap(err, "")).Send()
		}
	}
}

func generate(c *cli.Context) error {
	queryFilter := c.String("query-filter")
	queryIds := c.StringSlice("query-ids")
	traceTypes := c.StringSlice("trace-types")
	dsn := c.String("dsn")
	dateFrom := parseDate(c, "date-from")
	dateTo := parseDate(c, "date-to")

	prepareTLSConfig(dsn, c)

	db := openDbConnection(dsn)
	checkClickHouseVersion(c, db)
	flushSystemLog(db)

	createOutputDir(c)
	// stackFiles format = outputDir/hostname/queryId.traceType.outExtension -> stackFile descriptor
	stackFiles := make(map[string]*os.File, 256)
	stackWhere := " trace_type IN ('" + strings.Join(traceTypes, "','") + "') AND event_time >= ? AND event_time <= ?"
	stackArgs := []interface{}{dateFrom, dateTo}
	stackWhere, stackArgs = applyQueryFilter(db, c, queryFilter, queryIds, dateFrom, dateTo, stackWhere, stackArgs)

	var queryIdField string
	if c.Bool("normalize-query") {
		queryIdField = "toString(normalizedQueryHash(q.query)) AS query_id"
	} else {
		queryIdField = "replaceAll(t.query_id,':','_') AS query_id"
	}

	var traceFrom string
	if c.String("clickhouse-cluster") != "" {
		traceFrom = "clusterAllReplicas('" + c.String("clickhouse-cluster") + "', system.trace_log) AS t"
		traceFrom += " ANY LEFT JOIN clusterAllReplicas('" + c.String("clickhouse-cluster") + "', system.query_log) AS q"
		traceFrom += " ON q.query_id=t.query_id"
	} else {
		traceFrom = "system.trace_log AS t ANY LEFT JOIN system.query_log AS q ON q.query_id=t.query_id"
	}

	stackSQL := formatSQLTemplate(traceSQLTemplate, map[string]interface{}{
		"where":        stackWhere,
		"from":         traceFrom,
		"queryIdField": queryIdField,
	})
	fetchQuery(db, stackSQL, stackArgs, func(r map[string]interface{}) error {
		fetchStack := func(hostName, queryId, stack, traceType string, totalSize, samples uint64) {
			stackFile := filepath.Join(c.String("output-dir"), hostName, queryId+"."+traceType)
			if c.String("output-format") == "json" {
				stackFile += ".json"
			} else {
				stackFile += ".txt"
			}
			if _, exists := stackFiles[stackFile]; !exists {
				if f, err := os.Create(stackFile); err != nil {
					log.Fatal().Stack().Err(errors.Wrap(err, "")).Str("stackFile", stackFile).Send()
				} else {
					stackFiles[stackFile] = f
				}
				if c.String("output-format") == "json" {
					if _, err := stackFiles[stackFile].WriteString("[\n"); err != nil {
						log.Fatal().Stack().Err(errors.Wrap(err, "")).Send()
					}
				}
			}
			outputFormat := "%s %d\n"
			if c.String("output-format") == "json" {
				outputFormat = " {\"stack\":\"%s\", \"Value\": %d},\n"
			}
			if strings.Contains(traceType, "Memory") {
				if _, err := stackFiles[stackFile].WriteString(fmt.Sprintf(outputFormat, stack, totalSize)); err != nil {
					log.Fatal().Stack().Err(errors.Wrap(err, "")).Send()
				}
			} else {
				if _, err := stackFiles[stackFile].WriteString(fmt.Sprintf(outputFormat, stack, samples)); err != nil {
					log.Fatal().Stack().Err(errors.Wrap(err, "")).Send()
				}
			}

		}

		hostName := r["host_name"].(string)
		queryId := r["query_id"].(string)
		stack := r["stack"].(string)
		traceType := r["trace_type"].(string)
		totalSize := r["total_size"].(uint64)
		samples := r["samples"].(uint64)

		if queryId != "" {
			fetchStack(hostName, queryId, stack, traceType, totalSize, samples)
		}
		fetchStack(hostName, "global", stack, traceType, totalSize, samples)
		return nil
	})

	for stackKey, stackFile := range stackFiles {
		fileName := strings.Split(filepath.Base(stackKey), ".")
		queryId := fileName[0]
		traceType := fileName[1]

		hostName := strings.Split(filepath.Dir(stackKey), string(filepath.Separator))[1]

		if c.String("output-format") == "json" {
			if _, err := stackFile.WriteString("{}]\n"); err != nil {
				log.Fatal().Stack().Err(errors.Wrap(err, "")).Send()
			}
		}
		if err := stackFile.Close(); err != nil {
			log.Fatal().Err(err).Str("stackFile", stackFile.Name()).Send()
		}
		if c.String("output-format") == "txt" || c.String("output-format") == "svg" {
			writeSVG(c, hostName, queryId, traceType, stackFile.Name())
		}
	}
	log.Info().Int("processedFiles", len(stackFiles)).Msg("done processing")
	return nil
}

func createOutputDir(c *cli.Context) {
	// create output-dir if not exits
	if _, err := os.Stat(c.String("output-dir")); os.IsNotExist(err) {
		if err := os.MkdirAll(c.String("output-dir"), 0755); err != nil {
			log.Fatal().Err(err).Str("output-dir", c.String("output-dir")).Msg("Failed create output-dir")
		}
	}
}

func flushSystemLog(db *sql.DB) {
	if _, err := db.Exec("SYSTEM FLUSH LOGS"); err != nil {
		log.Fatal().Stack().Err(errors.Wrap(err, "")).Msg("SYSTEM FLUSH LOGS failed")
	}
}

func parseClickhouseVersion(versionStr string) ([]int, error) {
	split := strings.Split(versionStr, ".")
	if len(split) < 2 {
		return nil, fmt.Errorf("can't parse clickhouse version: '%s'", versionStr)
	}
	version := make([]int, len(split))
	var err error
	for i := range split {
		if version[i], err = strconv.Atoi(split[i]); err != nil {
			break
		}
	}
	return version, err
}

func checkClickHouseVersion(c *cli.Context, db *sql.DB) {
	fetchQuery(db, "SELECT version() AS version", nil, func(r map[string]interface{}) error {
		version, err := parseClickhouseVersion(r["version"].(string))
		if err != nil {
			log.Fatal().Str("version", r["version"].(string)).Err(err)
		}
		if (version[0] == 20 && version[1] < 6) && c.Bool("normalize-query") {
			log.Fatal().Str("version", r["version"].(string)).Msg("normalize-query require ClickHouse server version 20.6+")
		}
		if version[0] < 20 || (version[0] == 20 && version[1] < 5) {
			log.Fatal().Str("version", r["version"].(string)).Msg("system.trace_log with trace_type require ClickHouse server version 20.5+")
		}
		return nil
	})
}

func openDbConnection(dsn string) *sql.DB {
	db, err := sql.Open("chhttp", dsn)
	if err != nil {
		log.Fatal().Str("dsn", dsn).Err(err).Msg("Can't establishment ClickHouse connection")
	} else {
		log.Info().Str("dsn", dsn).Msg("connected to ClickHouse")
	}
	return db
}

func addWhereArgs(where, addWhere string, args []interface{}, addArg interface{}) (string, []interface{}) {
	where += addWhere
	if addArg != nil {
		args = append(args, addArg)
	}
	return where, args
}

func applyQueryFilter(db *sql.DB, c *cli.Context, queryFilter string, queryIds []string, dateFrom time.Time, dateTo time.Time, stackWhere string, stackArgs []interface{}) (string, []interface{}) {
	var queryIdSQL string
	var queryIdWhere, queryLogTable string
	var queryField, queryIdField string
	var queryIdArgs []interface{}

	queryIdWhere = "event_time >= ? AND event_time <= ?"
	queryIdArgs = []interface{}{dateFrom, dateTo}

	if c.String("clickhouse-cluster") != "" {
		queryLogTable = "clusterAllReplicas('" + c.String("clickhouse-cluster") + "', system.query_log) AS q"
	} else {
		queryLogTable = "system.query_log AS q"
	}

	if c.Bool("normalize-query") {
		queryField = "normalizeQuery(q.query) AS query"
		queryIdField = "toString(normalizedQueryHash(q.query)) AS query_id"
	} else {
		queryField = "q.query"
		queryIdField = "q.query_id"
	}

	if queryFilter != "" {
		if _, err := regexp.Compile(queryFilter); err != nil {
			log.Fatal().Err(err).Str("queryFilter", queryFilter).Msg("Invalid regexp")
		}
		queryIdWhere, queryIdArgs = addWhereArgs(queryIdWhere, " AND match(query, ?) ", queryIdArgs, queryFilter)
		stackWhere, stackArgs = addWhereArgs(stackWhere, " AND match(query, ?) ", stackArgs, queryFilter)
	}
	if len(queryIds) != 0 {
		queryIdWhere, queryIdArgs = addWhereArgs(queryIdWhere, " AND query_id IN ('"+strings.Join(queryIds, "','")+"') ", queryIdArgs, nil)
		stackWhere, stackArgs = addWhereArgs(stackWhere, " AND query_id IN ('"+strings.Join(queryIds, "','")+"') ", stackArgs, nil)
	}

	queryIdSQL = formatSQLTemplate(
		queryIdSQLTemplate,
		map[string]interface{}{
			"where":        queryIdWhere,
			"from":         queryLogTable,
			"queryField":   queryField,
			"queryIdField": queryIdField,
		},
	)

	if queryIdSQL != "" {
		sqlFiles := 0
		fetchQuery(db, queryIdSQL, queryIdArgs, func(r map[string]interface{}) error {
			if err := os.MkdirAll(filepath.Join(c.String("output-dir"), r["host_name"].(string)), 0755); err != nil {
				log.Fatal().Stack().Err(errors.Wrap(err, "")).Str("sqlDir", filepath.Join(c.String("output-dir"), r["host_name"].(string))).Send()
			}
			sqlFile := filepath.Join(c.String("output-dir"), r["host_name"].(string), r["query_id"].(string)+".sql")
			if err := ioutil.WriteFile(sqlFile, []byte(r["query"].(string)), 0644); err != nil {
				log.Fatal().Stack().Err(errors.Wrap(err, "")).Str("sqlFile", sqlFile).Send()
			}
			sqlFiles++
			return nil
		})
		log.Info().Int("sqlFiles", sqlFiles).Msg("write .sql files")
	}
	return stackWhere, stackArgs
}

func findFlameGraphScript(c *cli.Context) string {
	script := c.String("flamegraph-script")
	if script != "" {
		log.Debug().Msgf("set flamegraph-script: %s", script)
		if _, err := os.Stat(script); err == nil {
			return script
		}

		var err error
		if script, err = exec.LookPath(script); err != nil {
			log.Fatal().Msgf("%s is not found in $PATH", script)
		}
	}
	return script
}

func writeSVG(c *cli.Context, hostName, queryId, traceType, stackName string) {
	// @TODO DEV/2h advanced title generation logic
	title := fmt.Sprintf("hostName %s queryId %s (%s) from %s to %s", hostName, queryId, traceType, c.String("date-from"), c.String("date-to"))
	countName := "samples"
	if strings.Contains(traceType, "Memory") {
		countName = "bytes"
	}
	args := []string{
		"--title", title,
		"--width", fmt.Sprintf("%d", c.Int("width")),
		"--height", fmt.Sprintf("%d", c.Int("height")),
		"--countname", countName,
		"--nametype", traceType,
		//"--colors", "aqua",
	}
	stackFile, err := os.Open(stackName)
	if err != nil {
		log.Fatal().Stack().Err(errors.Wrap(err, "")).Str("stackName", stackName).Send()
	}
	script := findFlameGraphScript(c)
	log.Debug().Str("script", script).Strs("args", args).Send()
	cmd := exec.Command(script, args...)
	cmd.Stdin = stackFile
	cmd.Stderr = os.Stderr

	svg, err := cmd.Output()
	if err != nil {
		log.Fatal().Msgf("writeSVG: failed to run script %s : %s", script, err)
	}

	fileName := filepath.Join(c.String("output-dir"), hostName, queryId+"."+traceType+".svg")
	if err := ioutil.WriteFile(fileName, svg, 0644); err != nil {
		log.Fatal().Err(err).Str("fileName", fileName).Msg("can't write to svg")
	}
	if err := stackFile.Close(); err != nil {
		log.Fatal().Stack().Err(errors.Wrap(err, "")).Str("stackName", stackName).Send()
	}
}

// formatSQLTemplate use simple {key_from_context} template syntax
func formatSQLTemplate(sqlTemplate string, context map[string]interface{}) string {
	args, i := make([]string, len(context)*2), 0
	for k, v := range context {
		args[i] = "{" + k + "}"
		args[i+1] = fmt.Sprint(v)
		i += 2
	}
	return strings.NewReplacer(args...).Replace(sqlTemplate)
}

// fetchRowAsMap see https://kylewbanks.com/blog/query-result-to-map-in-golang
func fetchRowAsMap(rows *sql.Rows, cols []string) (m map[string]interface{}, err error) {
	// Create a slice of interface{}'s to represent each column,
	// and a second slice to contain pointers to each item in the columns slice.
	columns := make([]interface{}, len(cols))
	columnPointers := make([]interface{}, len(cols))
	for i := range columns {
		columnPointers[i] = &columns[i]
	}

	// Scan the result into the column pointers...
	if err := rows.Scan(columnPointers...); err != nil {
		return nil, err
	}

	// Create our map, and retrieve the value for each column from the pointers slice,
	// storing it in the map with the name of the column as the key.
	m = make(map[string]interface{}, len(cols))
	for i, colName := range cols {
		val := columnPointers[i].(*interface{})
		m[colName] = *val
	}
	return m, nil
}

func fetchQuery(db *sql.DB, sql string, sqlArgs []interface{}, fetchCallback func(r map[string]interface{}) error) {
	rows, err := db.Query(sql, sqlArgs...)
	if err != nil {
		log.Fatal().Stack().Err(errors.Wrap(err, "")).Str("sql", sql).Str("sqlArgs", fmt.Sprintf("%v", sqlArgs)).Send()
	} else {
		log.Debug().Str("sql", sql).Str("sqlArgs", fmt.Sprintf("%v", sqlArgs)).Msg("query OK")
	}
	cols, _ := rows.Columns()
	for rows.Next() {
		r, err := fetchRowAsMap(rows, cols)
		if err != nil {
			log.Fatal().Stack().Err(errors.Wrap(err, "")).Msg("fetch error")
		}
		if err := fetchCallback(r); err != nil {
			log.Fatal().Stack().Err(errors.Wrap(err, "")).Msg("fetch error")
		}
	}
	if err := rows.Close(); err != nil {
		log.Fatal().Stack().Err(errors.Wrap(err, "")).Interface("rows", rows).Send()
	}
}
