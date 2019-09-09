package chpool

import (
	"bytes"
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"bitbucket.org/clickhouse_pro/components/cfg"

	"bitbucket.org/clickhouse_pro/components/stacktrace"
	// clickhouse driver just registered in database/sql
	_ "github.com/kshvakov/clickhouse"
	"github.com/rs/zerolog/log"
	"strings"
)

//TableDefinitionsType main structure type for CH.PRO Tools Clickhouse tables definitions
type TableDefinitionsType map[string]map[string]map[string]string

var clickhousePool map[string]*sql.DB
var clickhouseDeadPool map[string]*sql.DB
var clickhouseMutex = sync.Mutex{}

//InitClickhousePool create lazy connection for each config.Clickhouse.Host and run ping all hosts every minute goroutine checkClickhousePoolLive
func InitClickhousePool(config *cfg.ConfigType) {
	if config.Clickhouse.Hosts == nil || len(config.Clickhouse.Hosts) == 0 {
		log.Fatal().Msg("Empty clickhouse.hosts in settings")
	}
	clickhousePool = map[string]*sql.DB{}
	clickhouseDeadPool = map[string]*sql.DB{}

	for i, mainHost := range config.Clickhouse.Hosts {
		dsn := "tcp://%s/%s?username=%s&password=%s&block_size=%d&read_timeout=%d&write_timeout=%d&debug=%t"
		if len(config.Clickhouse.Hosts) > 1 {
			altHosts := ""
			for j, host := range config.Clickhouse.Hosts {
				if j != i {
					if altHosts == "" {
						altHosts += host
					} else {
						altHosts += "," + host
					}
				}
			}
			dsn += "&connection_open_strategy=in_order&alt_hosts=" + altHosts
		}
		dsn = fmt.Sprintf(
			dsn,
			mainHost,
			config.Clickhouse.Database,
			config.Clickhouse.Username,
			config.Clickhouse.Password,
			config.Clickhouse.BlockSize,
			config.Clickhouse.ReadTimeout,
			config.Clickhouse.WriteTimeout,
			config.Debug,
		)
		log.Debug().Msg(dsn)
		ch, err := sql.Open("clickhouse", dsn)
		if err != nil {
			log.Fatal().Err(err).Msg("InitClickhousePool Open error")
		}
		err = ch.Ping()
		if err != nil {
			log.Fatal().Err(err).Msg("InitClickhousePool Ping error")
		}
		clickhousePool[mainHost] = ch
	}

	go checkClickhousePoolLive()

	log.Debug().Msgf(
		"InitClickhousePool Success with clickhousePool=%v config.Clickhouse.Hosts=%v", clickhousePool, config.Clickhouse.Hosts,
	)

}

func checkClickhousePoolLive() {
	check := func() {
		livePool := map[string]*sql.DB{}
		deadPool := map[string]*sql.DB{}
		pingAllConnection := func(Pool map[string]*sql.DB) {
			for clickhouseHost, conn := range Pool {
				if err := conn.Ping(); err == nil {
					livePool[clickhouseHost] = conn
				} else {
					log.Warn().Str("clickhouseHost", clickhouseHost).Err(err).Msg("some Clickhouse hosts is down")
					deadPool[clickhouseHost] = conn
				}
			}
		}
		pingAllConnection(clickhouseDeadPool)
		pingAllConnection(clickhousePool)
		if len(livePool) < 1 {
			log.Fatal().Msg("All clickhouse hosts is Down")
		}
		clickhouseMutex.Lock()
		defer clickhouseMutex.Unlock()
		clickhouseDeadPool = deadPool
		clickhousePool = livePool
	}
	for {
		check()
		time.Sleep(60 * time.Second)
	}
}

//DetectAllowReplicated if current clickhouse configuration have available zookeeper host, we can use ReplicatedMergeTree engine for save data
func DetectAllowReplicated(config *cfg.ConfigType) {
	clickhouseMutex.Lock()
	defer clickhouseMutex.Unlock()
	for _, conn := range clickhousePool {
		if r, err := conn.Query("SELECT name FROM system.tables WHERE database='system' AND name='zookeeper'"); err != nil {
			log.Fatal().Str("where", "DetectAllowReplicated").Err(err)
		} else {
			replicatedCount := 0
			for r.Next() {
				var name string
				if err = r.Scan(&name); err != nil || (err == nil && name != "zookeeper") {
					config.Clickhouse.TableSuffix = "_local"
					config.Clickhouse.UseReplicated = false
					return
				}
				replicatedCount++
			}
			if replicatedCount == 0 {
				config.Clickhouse.TableSuffix = "_local"
				config.Clickhouse.UseReplicated = false
				return
			}
		}
	}
	config.Clickhouse.TableSuffix = "_replicated"
	config.Clickhouse.UseReplicated = true
}

//DetectAllowDistributed if current clickhouse configuration have available cluster we can use Distributed engine for Read Data
func DetectAllowDistributed(config *cfg.ConfigType) {
	clickhouseMutex.Lock()
	defer clickhouseMutex.Unlock()
	if config.Clickhouse.ClusterName == "" {
		config.Clickhouse.UseDistributed = false
		return
	}
	for _, conn := range clickhousePool {
		if r, err := conn.Query("SELECT cluster FROM system.clusters WHERE cluster=?", config.Clickhouse.ClusterName); err != nil {
			log.Fatal().Str("where", "DetectAllowDistributed").Msgf("%v", err)
		} else {
			rowCount := 0
			for r.Next() {
				var clusterName string
				if err = r.Scan(&clusterName); err != nil || (err == nil && clusterName != config.Clickhouse.ClusterName) {
					config.Clickhouse.UseDistributed = false
					log.Debug().
						Err(err).
						Str("clusterName", clusterName).
						Str("config.Clickhouse.ClusterName", config.Clickhouse.ClusterName).
						Bool("config.Clickhouse.UseDistributed", config.Clickhouse.UseDistributed).
						Msg("clusterName error")
					return
				}
				rowCount++
			}
			if rowCount == 0 {
				config.Clickhouse.UseDistributed = false
				log.Debug().
					Err(err).
					Int("rowCount", rowCount).
					Str("config.Clickhouse.ClusterName", config.Clickhouse.ClusterName).
					Bool("config.Clickhouse.UseDistributed", config.Clickhouse.UseDistributed).
					Msg("clusterName not found")
				return
			}

		}
	}
	config.Clickhouse.UseDistributed = true

}

// CreateClickhouseDatabase create database if not exists
func CreateClickhouseDatabase(config *cfg.ConfigType) {
	var conn *sql.DB
	var err error
	var tx *sql.Tx

	// rollback if any query in transaction failed
	defer func() {
		rollbackAndExitOnFailure(err, tx)
	}()

	clickhouseMutex.Lock()
	defer clickhouseMutex.Unlock()

	for _, conn = range clickhousePool {
		tx, err = conn.Begin()
		if err != nil {
			return
		}
		if _, err = tx.Exec("CREATE DATABASE IF NOT EXISTS " + config.Clickhouse.Database); err != nil {
			log.Debug().Err(err).Msg("CREATE DATABASE IF NOT EXISTS " + config.Clickhouse.Database)
			return
		}
		if err = tx.Commit(); err != nil {
			return
		}
	}

}

// CreateClickhouseTables create tables and distributed tables in Clickhouse on All available Clickhouse Hosts
func CreateClickhouseTables(config *cfg.ConfigType, tableDefinitions TableDefinitionsType) {
	var engine string
	var conn *sql.DB
	var err error
	var tx *sql.Tx

	// rollback if any query in transaction failed
	defer func() {
		rollbackAndExitOnFailure(err, tx)
	}()

	clickhouseMutex.Lock()
	defer clickhouseMutex.Unlock()

	for tableName := range tableDefinitions {
		tableFullName := config.Clickhouse.TablePrefix + tableName + config.Clickhouse.TableSuffix

		if config.Clickhouse.UseReplicated {
			engine = fmt.Sprintf(
				"ReplicatedMergeTree('%s/tables/{layer}-{shard}/%s', '{replica}', %s, %s, (%s), 8192)",
				config.Zookeeper.Path,
				tableFullName,
				tableDefinitions[tableName]["key_definition"]["date_field"],
				tableDefinitions[tableName]["key_definition"]["sampling_key"],
				tableDefinitions[tableName]["key_definition"]["primary_key_fields"],
			)
		} else {
			engine = fmt.Sprintf(
				"MergeTree(%s, (%s), 8192)",
				tableDefinitions[tableName]["key_definition"]["date_field"],
				tableDefinitions[tableName]["key_definition"]["primary_key_fields"],
			)
		}
		tableSQL := getCreateTableSQL(tableName, config.Clickhouse.TablePrefix, config.Clickhouse.TableSuffix, engine, config, tableDefinitions)

		engine = fmt.Sprintf(
			"Distributed(%s, %s, %s, rand())",
			config.Clickhouse.ClusterName,
			config.Clickhouse.Database,
			tableFullName,
		)
		distributedTableSQL := getCreateTableSQL(tableName, config.Clickhouse.TablePrefix, "_distributed", engine, config, tableDefinitions)

		for _, conn = range clickhousePool {
			tx, err = conn.Begin()
			if err != nil {
				return
			}
			if config.Clickhouse.DropTable {
				if _, err = tx.Exec("DROP TABLE IF EXISTS " + config.Clickhouse.Database + "." + tableFullName); err != nil {
					log.Fatal().Err(err).Msg("DROP TABLE IF EXISTS " + config.Clickhouse.Database + "." + tableFullName)
					return
				}
			}

			if _, err = tx.Exec(tableSQL); err != nil {
				log.Debug().Err(err).Msg("CREATE TABLE IF NOT EXISTS " + config.Clickhouse.Database + "." + tableFullName)
				return
			}

			if _, err = tx.Exec(distributedTableSQL); err != nil {
				return
			}
			if err = tx.Commit(); err != nil {
				return
			}
		}
	}
}

func rollbackAndExitOnFailure(err error, tx *sql.Tx) {
	if err != nil {
		log.Error().Err(err).Msg("CreateClickhouseTables error")
		if tx != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				log.Error().Err(rollbackErr).Msg("CreateClickhouseTables rollback error")
			}
		}
		stacktrace.DumpErrorStackAndExit(err)
	}
}

//GetRandomClickhouseConnection get random database/sql instance pointer from live clickhouse connections
func GetRandomClickhouseConnection() *sql.DB {
	clickhouseMutex.Lock()
	defer clickhouseMutex.Unlock()
	i := rand.Intn(len(clickhousePool))
	for k := range clickhousePool {
		if i == 0 {
			return clickhousePool[k]
		}
		i--
	}
	return nil
}

//GetRandomClickhouseHost return random hostname:port string from current live clickhouse connections pool
func GetRandomClickhouseHost() string {
	clickhouseMutex.Lock()
	defer clickhouseMutex.Unlock()

	i := rand.Intn(len(clickhousePool))
	for k := range clickhousePool {
		if i == 0 {
			return k
		}
		i--
	}
	return ""
}

//LoadGzipFileToTable load gzip file over http protocol into clickhouse in any format, data must be gzipped and prepared on disk in selected format
func LoadGzipFileToTable(config *cfg.ConfigType, gzFileName string, table string, format string, sqlFields string) {
	host := GetRandomClickhouseHost()
	host = host[:strings.LastIndex(host, ":")] + ":" + config.Clickhouse.HTTPPort
	tableName := config.Clickhouse.TablePrefix + table + config.Clickhouse.TableSuffix

	var resp *http.Response
	var req *http.Request
	//nolint: gas
	//all data here from trusted source
	query := fmt.Sprintf("INSERT INTO %s.%s(%s) FORMAT %s", config.Clickhouse.Database, tableName, sqlFields, format)
	log.Debug().Str("query", query).Str("gzFileName", gzFileName).Str("database.table", config.Clickhouse.Database+"."+tableName).Str("clickhouse_host", host).Msg("LoadGzipFileToTable begin")
	dataFile, err := os.Open(gzFileName)
	if err == nil {
		query = url.Values{"query": {query}, "user": {config.Clickhouse.Username}, "password": {config.Clickhouse.Password}}.Encode()
		req, err = http.NewRequest("POST", "http://"+host+"?"+query, dataFile)
		if err == nil {
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set("Content-Encoding", "gzip")
			client := http.Client{}
			resp, err = client.Do(req)
			if err == nil {
				buf := bytes.Buffer{}
				_, err = buf.ReadFrom(resp.Body)
				if err == nil {
					if resp.StatusCode != http.StatusOK {
						fmt.Println(buf.String())
						log.Fatal().Str("gzFileName", gzFileName).Msg("LoadGzipFileToTable error")
					} else {
						log.Debug().Str("gzFileName", gzFileName).Str("Response", buf.String()).Msg("LoadGzipFileToTable DONE")
					}
				} else {
					log.Fatal().Str("gzFileName", gzFileName).Err(err).Msg("LoadGzipFileToTable read Response Body error")
				}
				if err = resp.Body.Close(); err == nil {
					err = os.Remove(gzFileName)
				}
			}
		}
	}

	if err != nil {
		log.Error().Str("gzFileName", gzFileName).Err(err).Msg("LoadGzipFileToTable error")
		return
	}

}
