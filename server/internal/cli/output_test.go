package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrintTable_Basic(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"Name", "Age"}
	rows := [][]string{{"Alice", "30"}, {"Bob", "25"}}
	PrintTable(&buf, headers, rows)
	out := buf.String()
	if !strings.Contains(out, "Name") {
		t.Error("expected header 'Name' in output")
	}
	if !strings.Contains(out, "Alice") {
		t.Error("expected row 'Alice' in output")
	}
	if !strings.Contains(out, "Bob") {
		t.Error("expected row 'Bob' in output")
	}
}

func TestPrintTable_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	PrintTable(&buf, []string{"Col"}, nil)
	out := buf.String()
	if !strings.Contains(out, "Col") {
		t.Error("expected header even with no rows")
	}
}

func TestPrintJSON_Valid(t *testing.T) {
	var buf bytes.Buffer
	v := map[string]string{"key": "value"}
	if err := PrintJSON(&buf, v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("key = %q, want value", result["key"])
	}
}

func TestPrintJSON_Indented(t *testing.T) {
	var buf bytes.Buffer
	PrintJSON(&buf, map[string]int{"a": 1})
	out := buf.String()
	if !strings.Contains(out, "  ") {
		t.Error("expected indented JSON (2-space indent)")
	}
}
