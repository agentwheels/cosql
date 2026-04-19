// Command dbops is a thin CLI for day-to-day PostgreSQL/MySQL ops.
//
// Read paths (list, query, schema, explain) run unattended. Write paths are
// split: any user can create a write *proposal*; only root (sudo) can apply
// one. This keeps agents and automation from acquiring write capabilities by
// accident, while making approvals human-verifiable via the sudo password.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/icebear/dbops/internal/cmd"
)

const usage = `dbops - database ops CLI for PostgreSQL and MySQL

Commands:
  list                               List configured databases
  query   <db>   [--sql ... | -f F]  Run a read-only SQL query
  schema  <db>   [<table>]           List schemas/tables or describe one table
  explain <db>   [--sql ... | -f F]  Show query execution plan (read-only)
  propose <db>   [--sql ... | -f F]  Create a write proposal (agents use this)
                 [--note "..."]
  proposal list  [--status S]        List proposals
  proposal show  <id>                Show proposal details
  proposal reject <id>               Mark a proposal rejected
  apply   <id>                       Apply a proposal (requires sudo)

Flags (may appear before or after positional args, e.g. 'dbops query mydb --sql ...'):
  --config PATH    Override config file (default: ~/.config/dbops/config.toml)
  --json           Emit JSON instead of text tables

Examples:
  dbops query local_pg --sql "select count(*) from users"
  dbops schema local_pg users
  dbops propose local_pg --sql "update users set active=false where id=7" --note "GDPR delete"
  awk -f to-sql.awk data.csv | dbops propose local_pg --note "..."
  sudo dbops apply a1b2c3d4
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	sub := os.Args[1]
	args := os.Args[2:]

	var err error
	switch sub {
	case "list":
		err = cmd.List(args)
	case "query":
		err = cmd.Query(args)
	case "schema":
		err = cmd.Schema(args)
	case "explain":
		err = cmd.Explain(args)
	case "propose":
		err = cmd.Propose(args)
	case "proposal":
		err = cmd.Proposal(args)
	case "apply":
		err = cmd.Apply(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", sub, usage)
		os.Exit(2)
	}

	if err != nil {
		msg := err.Error()
		// "usage: ..." errors are self-explanatory; don't double-prefix.
		if strings.HasPrefix(msg, "usage:") {
			fmt.Fprintln(os.Stderr, msg)
		} else {
			fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		}
		os.Exit(1)
	}
}
