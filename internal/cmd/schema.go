package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/icebear/dbops/internal/driver"
	"github.com/icebear/dbops/internal/output"
)

// Schema implements `dbops schema <db> [<table>]`.
func Schema(args []string) error {
	fs := newFlagSet("schema")
	var bf baseFlags
	bf.bind(fs)
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: dbops schema <db> [<table>]")
	}
	alias := fs.Arg(0)
	var table string
	if fs.NArg() >= 2 {
		table = fs.Arg(1)
	}

	db, drv, _, closeDB, err := openDB(bf.ConfigPath, alias)
	if err != nil {
		return err
	}
	defer closeDB()

	ctx := context.Background()

	if table == "" {
		objs, err := drv.ListObjects(ctx, db)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(objs))
		for _, o := range objs {
			rc := ""
			if o.Rows != nil {
				rc = strconv.FormatInt(*o.Rows, 10)
			}
			rows = append(rows, []string{o.Schema, o.Name, o.Kind, rc})
		}
		return output.RenderTable(os.Stdout, []string{"schema", "name", "kind", "rows(est)"}, rows, bf.JSON)
	}

	info, err := drv.DescribeTable(ctx, db, table)
	if err != nil {
		return err
	}
	if bf.JSON {
		return outputJSON(info)
	}
	return renderTable(os.Stdout, info)
}

func renderTable(w *os.File, info *driver.TableInfo) error {
	fmt.Fprintf(w, "Table: %s.%s\n\n", info.Schema, info.Name)

	fmt.Fprintln(w, "Columns:")
	crows := make([][]string, 0, len(info.Columns))
	for _, c := range info.Columns {
		n := "NO"
		if c.Nullable {
			n = "YES"
		}
		crows = append(crows, []string{c.Name, c.Type, n, c.Default})
	}
	if err := output.RenderTable(w, []string{"name", "type", "nullable", "default"}, crows, false); err != nil {
		return err
	}

	if len(info.Indexes) > 0 {
		fmt.Fprintln(w, "\nIndexes:")
		irows := make([][]string, 0, len(info.Indexes))
		for _, idx := range info.Indexes {
			attrs := []string{}
			if idx.Primary {
				attrs = append(attrs, "primary")
			}
			if idx.Unique {
				attrs = append(attrs, "unique")
			}
			irows = append(irows, []string{idx.Name, strings.Join(idx.Columns, ","), strings.Join(attrs, " ")})
		}
		if err := output.RenderTable(w, []string{"name", "columns", "attrs"}, irows, false); err != nil {
			return err
		}
	}

	if len(info.ForeignKeys) > 0 {
		fmt.Fprintln(w, "\nForeign keys:")
		frows := make([][]string, 0, len(info.ForeignKeys))
		for _, fk := range info.ForeignKeys {
			frows = append(frows, []string{fk.Name, strings.Join(fk.Columns, ","), fk.RefTable, strings.Join(fk.RefColumns, ",")})
		}
		if err := output.RenderTable(w, []string{"name", "columns", "ref_table", "ref_columns"}, frows, false); err != nil {
			return err
		}
	}
	return nil
}
