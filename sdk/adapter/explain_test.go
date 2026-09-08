package adapter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExplainRequestDoesNotSerializeStatement(t *testing.T) {
	payload, err := json.Marshal(ExplainRequest{Statement: "SELECT secret FROM users"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "SELECT") || strings.Contains(string(payload), "secret") {
		t.Fatalf("serialized explain request leaks statement: %s", payload)
	}
}

func TestExplainResultSerializesSafetyMetadata(t *testing.T) {
	payload, err := json.Marshal(ExplainResult{
		Engine:    "mysql",
		Format:    "mysql-json-sanitized",
		Estimated: true,
		Sanitized: true,
		Plan:      `{"query_block":{}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, expected := range []string{`"estimated":true`, `"sanitized":true`, "mysql-json-sanitized"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
}
