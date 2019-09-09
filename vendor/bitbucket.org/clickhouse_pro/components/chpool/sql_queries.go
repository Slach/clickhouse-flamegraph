package chpool

import (
	"database/sql"
	"fmt"
	"strings"

	"bitbucket.org/clickhouse_pro/components/cfg"

	"github.com/rs/zerolog/log"
	"sort"
)

func getCreateTableSQL(tableName string, prefix string, suffix string, engine string, config *cfg.ConfigType, tableDefinitions TableDefinitionsType) (q string) {
	tableDefinition, exists := tableDefinitions[tableName]
	if !exists {
		log.Fatal().Msg("Undefined table definition for " + tableName)
	}
	_, exists = tableDefinition["field_types"]
	if !exists {
		log.Fatal().Msg("Undefined field definition for " + tableName)
	}

	q = FormatSQLTemplate(
		"CREATE TABLE IF NOT EXISTS {db}.{table}",
		map[string]interface{}{"db": config.Clickhouse.Database, "table": prefix + tableName + suffix},
	)
	q += " ("
	fieldCount := len(tableDefinition["field_types"])

	var i int
	var fq string
	_, systemFieldExists := tableDefinition["system_field_types"]
	if systemFieldExists {
		fieldCount += len(tableDefinition["system_field_types"])
		i, fq = concatFieldTypes(tableDefinition["system_field_types"], fieldCount, i)
		q += fq
	}

	_, fq = concatFieldTypes(tableDefinition["field_types"], fieldCount, i)
	q += fq

	q += ") ENGINE=" + engine
	return q
}

func concatFieldTypes(fieldTypes map[string]string, fieldCount int, i int) (int, string) {
	q := ""
	fieldNames := make([]string, len(fieldTypes))
	j := 0
	for fieldName := range fieldTypes {
		fieldNames[j] = fieldName
		j++
	}
	sort.Strings(fieldNames)
	for _, fieldName := range fieldNames {
		fieldType := fieldTypes[fieldName]
		if lastIndex := strings.LastIndex(fieldName, ":"); lastIndex != -1 {
			q += strings.Title(fieldName[lastIndex+1:]) + " " + fieldType
		} else {
			q += fieldName + " " + fieldType
		}
		i++
		if i < fieldCount {
			q += ", "
		}
	}
	return i, q
}

// FormatSQLTemplate use simple {key_from_context} template syntax
func FormatSQLTemplate(sqlTemplate string, context map[string]interface{}) string {
	args, i := make([]string, len(context)*2), 0
	for k, v := range context {
		args[i] = "{" + k + "}"
		args[i+1] = fmt.Sprint(v)
		i += 2
	}
	return strings.NewReplacer(args...).Replace(sqlTemplate)
}

//FetchRowAsMap see https://kylewbanks.com/blog/query-result-to-map-in-golang
func FetchRowAsMap(rows *sql.Rows, cols []string) (m map[string]interface{}, err error) {
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
