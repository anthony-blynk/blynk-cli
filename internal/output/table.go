// Package output is the shared table/JSON/YAML renderer used by every
// blynk-cli command, selected via the global -o/--output flag.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Table is a simple, dependency-free column renderer.
type Table struct {
	Headers []string
	Rows    [][]string
}

// Print writes the table with columns aligned to their widest cell.
func (t Table) Print(w io.Writer) {
	if len(t.Headers) == 0 {
		return
	}

	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = len(h)
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	printRow := func(cells []string) {
		parts := make([]string, len(t.Headers))
		for i := range t.Headers {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			parts[i] = cell + strings.Repeat(" ", widths[i]-len(cell))
		}
		fmt.Fprintln(w, strings.Join(parts, "  "))
	}

	printRow(t.Headers)
	for _, row := range t.Rows {
		printRow(row)
	}
}

// Render writes v in the requested format ("table", "json", or "yaml").
// For "table", t is used directly; t may be nil for formats that only need
// v (e.g. a command with no sensible tabular form).
func Render(w io.Writer, format string, v any, t *Table) error {
	switch strings.ToLower(format) {
	case "", "table":
		if t == nil {
			return fmt.Errorf("no table representation available, use -o json or -o yaml")
		}
		t.Print(w)
		return nil
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case "yaml":
		enc := yaml.NewEncoder(w)
		defer enc.Close()
		return enc.Encode(v)
	default:
		return fmt.Errorf("unknown output format %q, want table|json|yaml", format)
	}
}
