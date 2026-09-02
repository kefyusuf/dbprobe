package collectors

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const maxSchemaRiskRows = 500

func NewSchemaRisk(q Queryer, database string, limit int) []collector.Collector {
	limit = clamp(limit, 1, maxSchemaRiskRows)
	return []collector.Collector{
		primaryKeyCollector{q: q, database: database, limit: limit},
		autoIncrementCollector{q: q, database: database, limit: limit},
	}
}

type primaryKeyCollector struct {
	q        Queryer
	database string
	limit    int
}

func (c primaryKeyCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.schema.primary_keys",
		Requires: capability.New("schema.metadata"),
		Strategy: collector.StrategySnapshot,
	}
}

func (c primaryKeyCollector) Collect(ctx context.Context, _ collector.Request) ([]signal.Observation, error) {
	rows, err := c.q.Query(ctx, `
SELECT
    tables.TABLE_SCHEMA,
    tables.TABLE_NAME,
    CASE WHEN pk.TABLE_NAME IS NULL THEN 0 ELSE 1 END AS primary_key_present
FROM information_schema.TABLES AS tables
LEFT JOIN (
    SELECT DISTINCT TABLE_SCHEMA, TABLE_NAME
    FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_TYPE = 'PRIMARY KEY'
) AS pk
  ON pk.TABLE_SCHEMA = tables.TABLE_SCHEMA
 AND pk.TABLE_NAME = tables.TABLE_NAME
WHERE tables.TABLE_SCHEMA = ?
  AND tables.TABLE_TYPE = 'BASE TABLE'
ORDER BY tables.TABLE_SCHEMA, tables.TABLE_NAME
LIMIT ?`, c.database, c.limit+1)
	if err != nil {
		return nil, fmt.Errorf("collect primary-key coverage: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit)
	truncated := false
	for rows.Next() {
		var schema, table string
		var present int
		if err := rows.Scan(&schema, &table, &present); err != nil {
			return nil, fmt.Errorf("scan primary-key coverage: %w", err)
		}
		if len(observations) >= c.limit {
			truncated = true
			break
		}
		value := present == 1
		observations = append(observations, signal.Observation{
			Key:         "mysql.table.primary_key_present",
			Object:      object.Ref{Kind: "mysql.table", ID: schema + "." + table},
			Boolean:     &value,
			Exactness:   signal.ExactnessScraped,
			Sensitivity: signal.SensitivityMetadata,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate primary-key coverage: %w", err)
	}
	if truncated {
		return observations, fmt.Errorf("primary-key scan truncated after %d tables", c.limit)
	}
	return observations, nil
}

type autoIncrementCollector struct {
	q        Queryer
	database string
	limit    int
}

func (c autoIncrementCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.schema.auto_increment",
		Requires: capability.New("schema.metadata"),
		Strategy: collector.StrategySnapshot,
	}
}

func (c autoIncrementCollector) Collect(ctx context.Context, _ collector.Request) ([]signal.Observation, error) {
	rows, err := c.q.Query(ctx, `
SELECT
    columns.TABLE_SCHEMA,
    columns.TABLE_NAME,
    columns.COLUMN_NAME,
    columns.DATA_TYPE,
    columns.COLUMN_TYPE,
    COALESCE(tables.AUTO_INCREMENT, 0)
FROM information_schema.COLUMNS AS columns
JOIN information_schema.TABLES AS tables
  ON tables.TABLE_SCHEMA = columns.TABLE_SCHEMA
 AND tables.TABLE_NAME = columns.TABLE_NAME
WHERE columns.TABLE_SCHEMA = ?
  AND columns.EXTRA LIKE '%auto_increment%'
ORDER BY columns.TABLE_SCHEMA, columns.TABLE_NAME, columns.ORDINAL_POSITION
LIMIT ?`, c.database, c.limit+1)
	if err != nil {
		return nil, fmt.Errorf("collect auto-increment metadata: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*3)
	columnsSeen := 0
	truncated := false
	for rows.Next() {
		var schema, table, column, dataType, columnType string
		var nextValueText string
		if err := rows.Scan(&schema, &table, &column, &dataType, &columnType, &nextValueText); err != nil {
			return nil, fmt.Errorf("scan auto-increment metadata: %w", err)
		}
		if columnsSeen >= c.limit {
			truncated = true
			break
		}
		columnsSeen++
		nextValue, err := strconv.ParseFloat(nextValueText, 64)
		if err != nil {
			return nil, fmt.Errorf("parse auto-increment value: %w", err)
		}
		maxValue, known := autoIncrementMax(dataType, columnType)
		if !known {
			continue
		}
		currentValue := math.Max(0, nextValue-1)
		usageRatio := 0.0
		if maxValue > 0 {
			usageRatio = currentValue / maxValue
		}
		ref := object.Ref{Kind: "mysql.column", ID: schema + "." + table + "." + column}
		observations = append(observations,
			number("mysql.auto_increment.current_value", ref, currentValue, signal.UnitCount),
			number("mysql.auto_increment.max_value", ref, maxValue, signal.UnitCount),
			number("mysql.auto_increment.utilization_ratio", ref, usageRatio, signal.UnitRatio),
		)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate auto-increment metadata: %w", err)
	}
	if truncated {
		return observations, fmt.Errorf("auto-increment scan truncated after %d columns", c.limit)
	}
	return observations, nil
}

func autoIncrementMax(dataType, columnType string) (float64, bool) {
	unsigned := strings.Contains(strings.ToLower(columnType), "unsigned")
	switch strings.ToLower(dataType) {
	case "tinyint":
		if unsigned {
			return 255, true
		}
		return 127, true
	case "smallint":
		if unsigned {
			return 65535, true
		}
		return 32767, true
	case "mediumint":
		if unsigned {
			return 16777215, true
		}
		return 8388607, true
	case "int", "integer":
		if unsigned {
			return 4294967295, true
		}
		return 2147483647, true
	case "bigint":
		if unsigned {
			return 18446744073709551615.0, true
		}
		return 9223372036854775807.0, true
	default:
		return 0, false
	}
}
