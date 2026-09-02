package collectors

import (
	"context"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const queryDigestSQL = `SELECT
  SCHEMA_NAME,
  DIGEST,
  COALESCE(DIGEST_TEXT, ''),
  COUNT_STAR,
  SUM_TIMER_WAIT,
  SUM_ROWS_EXAMINED,
  SUM_ROWS_SENT,
  SUM_NO_INDEX_USED,
  SUM_NO_GOOD_INDEX_USED,
  SUM_CREATED_TMP_DISK_TABLES,
  SUM_ERRORS,
  SUM_WARNINGS
FROM performance_schema.events_statements_summary_by_digest
WHERE SCHEMA_NAME = ? AND DIGEST IS NOT NULL
ORDER BY SUM_TIMER_WAIT DESC
LIMIT ?`

const maxDigestTextBytes = 2048

type queryCollector struct {
	query    Queryer
	database string
	limit    int
}

func NewQueries(query Queryer, database string, limit int) collector.Collector {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	return &queryCollector{query: query, database: database, limit: limit}
}

func (c *queryCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.queries",
		Requires: []capability.Capability{"workload.query_summary"},
		Produces: []signal.Key{
			"core.query.calls",
			"mysql.query.digest_text",
			"mysql.query.total_latency_ms",
			"mysql.query.rows_examined",
			"mysql.query.rows_sent",
			"mysql.query.no_index_used",
			"mysql.query.no_good_index_used",
			"mysql.query.temp_disk_tables",
			"mysql.query.errors",
			"mysql.query.warnings",
		},
		Strategy: collector.StrategyCounter,
	}
}

func (c *queryCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, queryDigestSQL, c.database, c.limit)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.queries: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*10)
	for rows.Next() {
		var schemaName, digest, digestText string
		var calls, timerPS, rowsExamined, rowsSent string
		var noIndex, noGoodIndex, tempDisk, errorsCount, warningsCount string
		if err := rows.Scan(
			&schemaName,
			&digest,
			&digestText,
			&calls,
			&timerPS,
			&rowsExamined,
			&rowsSent,
			&noIndex,
			&noGoodIndex,
			&tempDisk,
			&errorsCount,
			&warningsCount,
		); err != nil {
			return nil, fmt.Errorf("scan mysql.queries: %w", err)
		}

		ref := object.Ref{Kind: "mysql.query", ID: schemaName + ":" + digest}
		addNumber := func(key signal.Key, raw string, unit signal.Unit, scale float64) error {
			value, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return fmt.Errorf("parse %s for %s: %w", key, ref.ID, err)
			}
			observation := signal.NumberObservation(key, ref, value/scale, unit, signal.ExactnessCumulative, signal.SensitivityMetadata, req.CollectedAt)
			observation.Source = "performance_schema.events_statements_summary_by_digest"
			observations = append(observations, observation)
			return nil
		}

		for _, metric := range []struct {
			key   signal.Key
			raw   string
			unit  signal.Unit
			scale float64
		}{
			{"core.query.calls", calls, signal.UnitCount, 1},
			{"mysql.query.total_latency_ms", timerPS, signal.UnitMilliseconds, 1e9},
			{"mysql.query.rows_examined", rowsExamined, signal.UnitCount, 1},
			{"mysql.query.rows_sent", rowsSent, signal.UnitCount, 1},
			{"mysql.query.no_index_used", noIndex, signal.UnitCount, 1},
			{"mysql.query.no_good_index_used", noGoodIndex, signal.UnitCount, 1},
			{"mysql.query.temp_disk_tables", tempDisk, signal.UnitCount, 1},
			{"mysql.query.errors", errorsCount, signal.UnitCount, 1},
			{"mysql.query.warnings", warningsCount, signal.UnitCount, 1},
		} {
			if err := addNumber(metric.key, metric.raw, metric.unit, metric.scale); err != nil {
				return nil, err
			}
		}

		text := truncateUTF8(digestText, maxDigestTextBytes)
		textObservation := signal.Observation{
			Key:         "mysql.query.digest_text",
			Object:      ref,
			Exactness:   signal.ExactnessScraped,
			Text:        &text,
			CollectedAt: req.CollectedAt,
			Sensitivity: signal.SensitivityQueryShape,
			Source:      "performance_schema.events_statements_summary_by_digest",
		}
		observations = append(observations, textObservation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.queries: %w", err)
	}
	return observations, nil
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end]
}
