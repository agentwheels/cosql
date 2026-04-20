package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agentwheels/cosql/internal/output"
	"github.com/agentwheels/cosql/internal/proposal"
)

// Proposal implements `cosql proposal {list|show|reject}`.
func Proposal(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cosql proposal <list|show|reject> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return proposalList(rest)
	case "show":
		return proposalShow(rest)
	case "reject":
		return proposalReject(rest)
	default:
		return fmt.Errorf("unknown proposal subcommand: %s", sub)
	}
}

func proposalList(args []string) error {
	fs := newFlagSet("proposal list")
	var bf baseFlags
	bf.bind(fs)
	var status string
	fs.StringVar(&status, "status", "", "filter by status (pending/applied/rejected/expired)")
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if err := fixHomeForSudo(); err != nil {
		return err
	}

	store, err := proposal.Open()
	if err != nil {
		return err
	}
	ps, err := store.List(proposal.Status(status))
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(ps))
	for _, p := range ps {
		rows = append(rows, []string{p.ID, string(p.Status), p.DB, p.CreatedAt, oneLine(p.SQL, 60)})
	}
	return output.RenderTable(os.Stdout, []string{"id", "status", "db", "created_at", "sql"}, rows, bf.JSON)
}

func proposalShow(args []string) error {
	fs := newFlagSet("proposal show")
	var bf baseFlags
	bf.bind(fs)
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: cosql proposal show <id>")
	}
	if err := fixHomeForSudo(); err != nil {
		return err
	}

	store, err := proposal.Open()
	if err != nil {
		return err
	}
	p, err := store.Get(fs.Arg(0))
	if err != nil {
		return err
	}
	if bf.JSON {
		return outputJSON(p)
	}
	printProposal(os.Stdout, p)
	return nil
}

func proposalReject(args []string) error {
	fs := newFlagSet("proposal reject")
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: cosql proposal reject <id>")
	}
	if err := fixHomeForSudo(); err != nil {
		return err
	}

	store, err := proposal.Open()
	if err != nil {
		return err
	}
	p, err := store.Get(fs.Arg(0))
	if err != nil {
		return err
	}
	if p.Status != proposal.StatusPending {
		return fmt.Errorf("proposal %s is %s, not pending", p.ID, p.Status)
	}
	p.Status = proposal.StatusRejected
	if err := store.Update(p); err != nil {
		return err
	}
	fmt.Printf("proposal %s rejected.\n", p.ID)
	return nil
}

func printProposal(w io.Writer, p *proposal.Proposal) {
	fmt.Fprintf(w, "id          %s\n", p.ID)
	fmt.Fprintf(w, "status      %s\n", p.Status)
	fmt.Fprintf(w, "db          %s (%s)\n", p.DB, p.Driver)
	fmt.Fprintf(w, "created     %s by %s\n", p.CreatedAt, p.CreatedBy)
	if p.AppliedAt != "" {
		fmt.Fprintf(w, "applied     %s by %s\n", p.AppliedAt, p.AppliedBy)
	}
	if p.Note != "" {
		fmt.Fprintf(w, "note        %s\n", p.Note)
	}
	fmt.Fprintf(w, "\nSQL:\n%s\n", p.SQL)
	if p.DryRun != nil && p.DryRun.Explain != "" {
		fmt.Fprintf(w, "\nEXPLAIN (dry-run):\n%s", p.DryRun.Explain)
		if !strings.HasSuffix(p.DryRun.Explain, "\n") {
			fmt.Fprintln(w)
		}
	}
	if p.Result != nil {
		fmt.Fprintf(w, "\nResult:\n  affected_rows = %d\n  duration_ms   = %d\n",
			p.Result.AffectedRows, p.Result.DurationMS)
		if p.Result.Error != "" {
			fmt.Fprintf(w, "  error         = %s\n", p.Result.Error)
		}
	}
}

func oneLine(s string, n int) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
