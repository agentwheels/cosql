// Package output renders command results as either a text table or JSON.
package output

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
)

// RenderRows pretty-prints sql.Rows. It consumes the cursor.
func RenderRows(w io.Writer, rows *sql.Rows, asJSON bool) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	if asJSON {
		var out []map[string]any
		for rows.Next() {
			if err := rows.Scan(ptrs...); err != nil {
				return err
			}
			row := make(map[string]any, len(cols))
			for i, c := range cols {
				row[c] = normalize(vals[i])
			}
			out = append(out, row)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	_ = types // reserved for future use (e.g. NUMERIC column alignment)

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(cols, "\t"))
	sep := make([]string, len(cols))
	for i, c := range cols {
		sep[i] = strings.Repeat("-", max(3, len(c)))
	}
	fmt.Fprintln(tw, strings.Join(sep, "\t"))

	n := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		parts := make([]string, len(cols))
		for i := range cols {
			parts[i] = formatCell(vals[i])
		}
		fmt.Fprintln(tw, strings.Join(parts, "\t"))
		n++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(w, "\n(%d row%s)\n", n, plural(n))
	return nil
}

// RenderTable prints a generic 2D table (used by schema/proposal list).
func RenderTable(w io.Writer, headers []string, rows [][]string, asJSON bool) error {
	if asJSON {
		records := make([]map[string]string, len(rows))
		for i, r := range rows {
			m := make(map[string]string, len(headers))
			for j, h := range headers {
				if j < len(r) {
					m[h] = r[j]
				}
			}
			records[i] = m
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	sep := make([]string, len(headers))
	for i, h := range headers {
		sep[i] = strings.Repeat("-", max(3, len(h)))
	}
	fmt.Fprintln(tw, strings.Join(sep, "\t"))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	return tw.Flush()
}

func formatCell(v any) string {
	if v == nil {
		return "NULL"
	}
	switch x := v.(type) {
	case []byte:
		return string(x)
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

// normalize converts driver-specific scalar types into JSON-friendly values.
func normalize(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	default:
		return x
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
