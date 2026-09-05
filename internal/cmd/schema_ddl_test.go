package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaDDLRequiresTableBeforeOpeningDatabase(t *testing.T) {
	for _, args := range [][]string{{"example", "--ddl"}, {"--ddl", "example"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			args = append(args, "--config", filepath.Join(t.TempDir(), "missing.toml"))
			err := Schema(args)
			if err == nil || !strings.Contains(err.Error(), "--ddl") || !strings.Contains(strings.ToLower(err.Error()), "table") || strings.Contains(err.Error(), "not defined") {
				t.Fatalf("Schema must reject missing table before loading database config: %v", err)
			}
		})
	}
}
