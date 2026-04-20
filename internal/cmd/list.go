package cmd

import (
	"os"

	"github.com/agentwheels/cosql/internal/config"
	"github.com/agentwheels/cosql/internal/output"
)

// List implements `cosql list`.
func List(args []string) error {
	fs := newFlagSet("list")
	var bf baseFlags
	bf.bind(fs)
	if err := parseMixed(fs, args); err != nil {
		return err
	}

	cfg, err := config.Load(bf.ConfigPath)
	if err != nil {
		return err
	}

	rows := make([][]string, 0, len(cfg.DB))
	for _, name := range cfg.Aliases() {
		db := cfg.DB[name]
		marker := ""
		if name == cfg.DefaultDB {
			marker = "*"
		}
		rows = append(rows, []string{marker, name, db.Driver, db.Notes})
	}
	return output.RenderTable(os.Stdout, []string{"default", "alias", "driver", "notes"}, rows, bf.JSON)
}
