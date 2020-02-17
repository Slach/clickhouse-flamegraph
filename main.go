package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/araddon/dateparse"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "clickhouse-flamegraph"
	app.Usage = "visualize clickhouse system.trace_log as flamegraph, based on https://gist.github.com/alexey-milovidov/92758583dd41c24c360fdb8d6a4da194"
	app.ArgsUsage = ""
	app.HideHelp = false
	app.Version = "2019.9.5"
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
			Name:   "flamegraph-script",
			EnvVars: []string{"CH_FLAME_FLAMEGRAPH_SCRIPT"},
			Usage:  "path of flamegraph.pl. if not given, find the script from $PATH",
		},
		&cli.StringFlag{
			Name:   "output-dir",
			Aliases: []string{"o"},
			Value:  "./clickhouse-flamegraphs/",
			EnvVars: []string{"CH_FLAME_OUTPUT_DIR"},
			Usage:  "destination path of generated flamegraphs files",
		},
		&cli.StringFlag{
			Name:   "date-from",
			Aliases: []string{"from"},
			Usage:  "filter system.trace_log from date in any parsable format, see https://github.com/araddon/dateparse",
			EnvVars: []string{"CH_FLAME_DATE_FROM"},
			Value:  time.Now().Add(time.Duration(-5) * time.Minute).Format("2006-01-02 15:04:05 -0700"),
		},
		&cli.StringFlag{
			Name:   "date-to",
			Aliases: []string{"to"},
			Usage:  "filter system.trace_log to date in any parsable format, see https://github.com/araddon/dateparse",
			EnvVars: []string{"CH_FLAME_DATE_TO"},
			Value:  time.Now().Format("2006-01-02 15:04:05 -0700"),
		},
		&cli.StringFlag{
			Name:   "query-filter",
			Aliases: []string{"query-regexp"},
			Usage:  "filter system.query_log by any regexp, see https://github.com/google/re2/wiki/Syntax",
			EnvVars: []string{"CH_FLAME_QUERY_FILTER"},
			Value:  "",
		},
		&cli.StringFlag{
			Name:   "clickhouse-dsn",
			Aliases: []string{"dsn"},
			Usage:  "clickhouse connection string, see https://github.com/ClickHouse/clickhouse-go#dsn",
			EnvVars: []string{"CH_FLAME_CLICKHOUSE_DSN"},
			Value:  "tcp://localhost:9000?database=default",
		},
		&cli.StringFlag{
			Name:   "output-format",
			Aliases: []string{"format"},
			Usage:  "accept values: svg, txt (see https://github.com/brendangregg/FlameGraph#2-fold-stacks), json (see https://github.com/spiermar/d3-flame-graph/#input-format, ",
			EnvVars: []string{"CH_FLAME_OUTPUT_FORMAT"},
			Value:  "svg",
		},
		&cli.BoolFlag{
			Name:   "debug",
			Aliases: []string{"verbose"},
			Usage:  "show debug log",
			EnvVars: []string{"CH_FLAME_DEBUG"},
		},
		&cli.BoolFlag{
			Name:   "console",
			Usage:  "output logs to console format instead of json",
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
SELECT 
	query, query_id 
FROM system.query_log
WHERE {where}
`

	traceSQLTemplate = `
SELECT 
	query_id,
	count() AS samples, 
	arrayStringConcat(arrayReverse(arrayMap(x -> concat( demangle(addressToSymbol(x)), '#', addressToLine(x) ), trace)), ';') AS stack
FROM system.trace_log
WHERE {where}
GROUP BY query_id, trace
`
)

func run(c *cli.Context) error {
	stdlog.SetOutput(log.Logger)
	clickhouse.SetLogOutput(log.Logger)
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
		log.Fatal().Err(err).Msgf("invalid %s parameter = %s", paramName, c.String(paramName))
	}
	return parsedDate
}

func generate(c *cli.Context) error {
	queryFilter := c.String("query-filter")
	dsn := c.String("dsn")
	dateFrom := parseDate(c, "date-from")
	dateTo := parseDate(c, "date-to")

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		log.Fatal().Str("dsn", dsn).Err(err).Msg("Can't establishment clickhouse connection")
	} else {
		log.Info().Str("dsn", dsn).Msg("conected to clickhouse")
	}
	if _, err := db.Exec("SYSTEM FLUSH LOGS"); err != nil {
		log.Fatal().Err(err).Msg("SYSTEM FLUSH LOGS failed")
	}
	// create output-dir if not exits
	if _, err := os.Stat(c.String("output-dir")); os.IsNotExist(err) {
		os.MkdirAll(c.String("output-dir"), 0755)
	}
	// queryId -> stackFile descriptor
	stackFiles := make(map[string]*os.File, 256)
	where := " event_time >= ? AND event_time <= ?"
	where = applyQueryFilter(db, c, queryFilter, dateFrom, dateTo, where)
	stackSQL := formatSQLTemplate(traceSQLTemplate, map[string]interface{}{"where": where})
	stackArgs := []interface{}{dateFrom, dateTo}
	fetchQuery(db, stackSQL, stackArgs, func(r map[string]interface{}) error {
		fetchStack := func(queryId, stack string, samples uint64) {
			if _, exists := stackFiles[queryId]; !exists {
				stackFile := queryId
				if c.String("output-format") == "json" {
					stackFile += ".json"
				} else {
					stackFile += ".txt"
				}
				stackFile = filepath.Join(c.String("output-dir"), stackFile)
				if f, err := os.Create(stackFile); err != nil {
					log.Fatal().Err(err).Stack().Str("stackFile", stackFile).Send()
				} else {
					stackFiles[queryId] = f
				}
				if c.String("output-format") == "json" {
					if _, err := stackFiles[queryId].WriteString("[\n"); err != nil {
						log.Fatal().Err(err).Stack().Send()
					}
				}
			}
			outputFormat := "%s %d\n"
			if c.String("output-format") == "json" {
				outputFormat = " {\"stack\":\"%s\", \"Value\": %d},\n"
			}
			if _, err := stackFiles[queryId].WriteString(fmt.Sprintf(outputFormat, stack, samples)); err != nil {
				log.Fatal().Err(err).Stack().Send()
			}

		}
		queryId := r["query_id"].(string)
		stack := r["stack"].(string)
		samples := r["samples"].(uint64)
		fetchStack(queryId, stack, samples)
		fetchStack("global", stack, samples)
		return nil
	})

	for queryId, stackFile := range stackFiles {
		if c.String("output-format") == "json" {
			if _, err := stackFiles[queryId].WriteString("{}]\n"); err != nil {
				log.Fatal().Err(err).Stack().Send()
			}
		}
		if err := stackFile.Close(); err != nil {
			log.Fatal().Err(err).Str("stackFile", stackFile.Name()).Send()
		}
		if c.String("output-format") == "txt" || c.String("output-format") == "svg" {
			writeSVG(c, queryId, stackFile.Name())
		}
	}
	log.Info().Int("processedFiles", len(stackFiles)).Msg("done processing")
	return nil
}

func applyQueryFilter(db *sql.DB, c *cli.Context, queryFilter string, dateFrom time.Time, dateTo time.Time, traceWhere string) string {
	var queryIdSQL string
	var queryFilterArgs []interface{}
	if queryFilter != "" {
		if _, err := regexp.Compile(queryFilter); err != nil {
			log.Fatal().Err(err).Str("queryFilter", queryFilter).Msg("Invalid regexp")
		}
		queryIdSQL = formatSQLTemplate(
			queryIdSQLTemplate,
			map[string]interface{}{
				"where": "type = 1 AND match(query, ?) AND event_time >= ? AND event_time <= ?",
			},
		)
		queryFilterArgs = []interface{}{queryFilter, dateFrom, dateTo}
	} else {
		queryIdSQL = formatSQLTemplate(
			queryIdSQLTemplate,
			map[string]interface{}{
				"where": "type = 1 AND event_time >= ? AND event_time <= ?",
			},
		)
		queryFilterArgs = []interface{}{dateFrom, dateTo}

	}
	var queryIds = make([]string, 0, 128)
	fetchQuery(db, queryIdSQL, queryFilterArgs, func(r map[string]interface{}) error {
		queryId := r["query_id"].(string)
		query := r["query"].(string)
		sqlFile := filepath.Join(c.String("output-dir"), queryId+".sql")
		if err := ioutil.WriteFile(sqlFile, []byte(query), 0644); err != nil {
			log.Fatal().Err(err).Str("sqlFile", sqlFile)
		}
		queryIds = append(queryIds, queryId)
		return nil
	})
	if queryFilter != "" && len(queryIds) != 0 {
		traceWhere += " AND query_id IN ('" + strings.Join(queryIds, "','") + "') "
	}
	return traceWhere
}

func findFlameGraphScript(c *cli.Context) string {
	if script := c.String("flamegraph-script"); script != "" {
		log.Debug().Msgf("script: %s", script)
		if _, err := os.Stat(script); err == nil {
			return script
		}
	}

	if script, err := exec.LookPath("flamegraph.pl"); err == nil {
		return script
	}

	log.Fatal().Msg("flamegraph.pl is not found in $PATH")

	return ""
}

func writeSVG(c *cli.Context, queryId string, stackName string) {
	// @TODO DEV/2h advanced title generation logic
	title := fmt.Sprintf("Clickhouse queryId %s from %s to %s", queryId, c.String("date-from"), c.String("date-to"))
	args := []string{
		"--title", title,
		"--width", fmt.Sprintf("%d", c.Int("width")),
		"--height", fmt.Sprintf("%d", c.Int("height")),
		"--countname", "samples",
		"--nametype", "Stack",
		//"--colors", "aqua",
	}
	stackFile, err := os.Open(stackName)
	if err != nil {
		log.Fatal().Stack().Err(err).Str("stackFile", stackName)
	}
	script := findFlameGraphScript(c)
	cmd := exec.Command(script, args...)
	cmd.Stdin = stackFile
	cmd.Stderr = os.Stderr

	svg, err := cmd.Output()
	if err != nil {
		log.Fatal().Msgf("writeSVG: failed to run script %s : %s", script, err)
	}

	fileName := filepath.Join(c.String("output-dir"), queryId+".svg")
	if err := ioutil.WriteFile(fileName, svg, 0644); err != nil {
		log.Fatal().Err(err).Str("fileName", fileName).Msg("can't write to svg")
	}
	if err := stackFile.Close(); err != nil {
		log.Fatal().Err(err).Str("stackName", stackName).Stack().Send()
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

//fetchRowAsMap see https://kylewbanks.com/blog/query-result-to-map-in-golang
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
		if exception, is_exception := err.(*clickhouse.Exception); is_exception {
			log.Fatal().Err(err).Int32("code", exception.Code).Str("message", exception.Message).Str("stacktrace", exception.StackTrace).Send()
		} else {
			log.Fatal().Err(err).Str("sql", sql).Str("sqlArgs", fmt.Sprintf("%v", sqlArgs)).Msg("query error")
		}
	} else {
		log.Debug().Str("sql", sql).Str("sqlArgs", fmt.Sprintf("%v", sqlArgs)).Msg("query OK")
	}
	cols, _ := rows.Columns()
	for rows.Next() {
		r, err := fetchRowAsMap(rows, cols)
		if err != nil {
			log.Fatal().Err(err).Msg("fetch error")
		}
		if err := fetchCallback(r); err != nil {
			log.Fatal().Err(err).Msg("fetch error")
		}
	}
	if err := rows.Close(); err != nil {
		log.Fatal().Err(err).Interface("rows", rows)
	}
}
