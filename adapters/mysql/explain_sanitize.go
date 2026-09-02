package mysql

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const maxExplainPlanBytes = 1 << 20

var safePlanStringKeys = map[string]struct{}{
	"access_type":        {},
	"data_read_per_join": {},
	"eval_cost":          {},
	"key":                {},
	"key_length":         {},
	"prefix_cost":        {},
	"query_cost":         {},
	"read_cost":          {},
	"schema_name":        {},
	"sort_cost":          {},
	"table_name":         {},
	"using_join_buffer":  {},
}

var safePlanStringSliceKeys = map[string]struct{}{
	"possible_keys":  {},
	"used_columns":   {},
	"used_key_parts": {},
}

var safePlanNumberKeys = map[string]struct{}{
	"select_id":              {},
	"rows_examined_per_scan": {},
	"rows_produced_per_join": {},
	"filtered":               {},
	"rows_for_plan":          {},
	"rows_to_scan":           {},
}

var safePlanBoolKeys = map[string]struct{}{
	"using_index":           {},
	"using_index_condition": {},
	"using_temporary_table": {},
	"using_filesort":        {},
	"dependent":             {},
	"cacheable":             {},
	"using_mrr":             {},
}

func sanitizeMySQLJSONPlan(raw string) (string, error) {
	if len(raw) > maxExplainPlanBytes {
		return "", fmt.Errorf("MySQL JSON plan exceeds the maximum size")
	}

	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("invalid MySQL JSON plan")
	}
	if _, ok := value.(map[string]any); !ok {
		return "", fmt.Errorf("invalid MySQL JSON plan")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", fmt.Errorf("invalid MySQL JSON plan")
	}

	sanitized := sanitizePlanValue("", value)
	root, ok := sanitized.(map[string]any)
	if !ok || len(root) == 0 {
		return "", fmt.Errorf("MySQL JSON plan contains no safe metadata")
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("sanitize MySQL JSON plan")
	}
	return string(encoded), nil
}

func sanitizePlanValue(key string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			if stringValue, ok := childValue.(string); ok {
				if _, allowed := safePlanStringKeys[childKey]; allowed {
					out[childKey] = stringValue
				}
				continue
			}
			if sliceValue, ok := childValue.([]any); ok {
				cleaned := sanitizePlanSlice(childKey, sliceValue)
				if len(cleaned) > 0 {
					out[childKey] = cleaned
				}
				continue
			}
			cleaned := sanitizePlanValue(childKey, childValue)
			if cleaned != nil {
				out[childKey] = cleaned
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		return sanitizePlanSlice(key, typed)
	case string:
		if _, allowed := safePlanStringKeys[key]; allowed {
			return typed
		}
		return nil
	case json.Number:
		if _, allowed := safePlanNumberKeys[key]; allowed {
			return typed
		}
		return nil
	case float64:
		if _, allowed := safePlanNumberKeys[key]; allowed {
			return typed
		}
		return nil
	case bool:
		if _, allowed := safePlanBoolKeys[key]; allowed {
			return typed
		}
		return nil
	case nil:
		return nil
	default:
		return nil
	}
}

func sanitizePlanSlice(key string, values []any) []any {
	out := make([]any, 0, len(values))
	_, allowStrings := safePlanStringSliceKeys[key]
	for _, value := range values {
		if stringValue, ok := value.(string); ok {
			if allowStrings {
				out = append(out, stringValue)
			}
			continue
		}
		cleaned := sanitizePlanValue(key, value)
		if cleaned != nil {
			out = append(out, cleaned)
		}
	}
	return out
}
