package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestTablePrintAlignsColumns(t *testing.T) {
	tbl := Table{
		Headers: []string{"NAME", "SERVER"},
		Rows: [][]string{
			{"acme-prod", "acme.blynk.cloud"},
			{"qa", "fra.blynk-qa.com"},
		},
	}
	var buf bytes.Buffer
	tbl.Print(&buf)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d: %q", len(lines), buf.String())
	}
	// Every line should be the same length once padded/aligned.
	for i, l := range lines {
		if len(l) != len(lines[0]) {
			t.Errorf("line %d length = %d, want %d (columns not aligned): %q", i, len(l), len(lines[0]), l)
		}
	}
}

func TestTablePrintEmptyHeadersNoOutput(t *testing.T) {
	var buf bytes.Buffer
	(Table{}).Print(&buf)
	if buf.Len() != 0 {
		t.Errorf("expected no output for a table with no headers, got %q", buf.String())
	}
}

func TestTablePrintShortRowPadsBlank(t *testing.T) {
	tbl := Table{
		Headers: []string{"A", "B", "C"},
		Rows:    [][]string{{"x"}}, // fewer cells than headers
	}
	var buf bytes.Buffer
	tbl.Print(&buf) // must not panic
	if !strings.Contains(buf.String(), "x") {
		t.Errorf("expected output to contain the one provided cell: %q", buf.String())
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	if err := Render(&buf, "json", data, nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output isn't valid JSON: %v (%q)", err, buf.String())
	}
	if got["key"] != "value" {
		t.Errorf("got %v", got)
	}
}

func TestRenderYAML(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, "yaml", map[string]string{"key": "value"}, nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), "key: value") {
		t.Errorf("output = %q, want to contain 'key: value'", buf.String())
	}
}

func TestRenderTableRequiresTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, "table", nil, nil); err == nil {
		t.Fatal("expected an error when format is table but no *Table is given")
	}
}

func TestRenderTableUsesTable(t *testing.T) {
	var buf bytes.Buffer
	tbl := &Table{Headers: []string{"H"}, Rows: [][]string{{"v"}}}
	if err := Render(&buf, "table", nil, tbl); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), "v") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRenderDefaultsToTableWhenFormatEmpty(t *testing.T) {
	var buf bytes.Buffer
	tbl := &Table{Headers: []string{"H"}, Rows: [][]string{{"v"}}}
	if err := Render(&buf, "", nil, tbl); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), "v") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRenderUnknownFormatIsError(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, "csv", nil, nil); err == nil {
		t.Fatal("expected an error for an unknown output format")
	}
}
