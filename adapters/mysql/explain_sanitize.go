package mysql

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

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

func sanitizeMySQLJSONPlan(raw string) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("invalid MySQL JSON plan")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", fmt.Errorf("invalid MySQL JSON plan")
	}
	sanitized := sanitizePlanValue("", value)
	encoded, err := json.Marshal(sanitized)
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
			out[childKey] = sanitizePlanValue(childKey, childValue)
		}
		return out
	case []any:
		return sanitizePlanSlice(key, typed)
	case string:
		if _, allowed := safePlanStringKeys[key]; allowed {
			return typed
		}
		return nil
	case json.Number, float64, bool, nil:
		return typed
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
		clean := sanitizePlanValue(key, value)
		if clean != nil {
			out = append(out, clean)
		}
	}
	return out
}
