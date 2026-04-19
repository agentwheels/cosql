# Write operations: propose → apply

This is the detailed reference for any write against a database (INSERT /
UPDATE / DELETE / DDL). SKILL.md has the short version; this document is
what you load when you're actually about to propose something.

You MUST NOT issue a write directly. The only supported path is:

```
agent          human (in a separate terminal)
-----          -----------------------------
propose   ───► sudo dbops apply <id>
```

## The four-step flow

### 1. Size the blast radius

Before proposing, figure out how many rows the statement will touch
and put the number in `--note`. The method depends on the shape of the
change — pick whichever is cheapest and accurate:

- `UPDATE` / `DELETE` with a WHERE clause → run a `dbops query` with
  the same WHERE to get `count(*)`.
- Bulk `INSERT` from a CSV / JSON source → count rows in the source file.
- DDL / idempotent setup → a shape description is enough
  (`"creates 1 index on users(email)"`, `"adds nullable column"`).

What matters is that the figure in `--note` is defensible and can be
cross-checked against `result.affected_rows` after apply. If you can't
commit to a number or a shape, you're not ready to propose.

### 2. Create the proposal

```sh
dbops propose <db> --sql "UPDATE users SET active=false WHERE id=7" \
  --note "GDPR delete — support ticket #1234; 1 row counted"
```

Or from a file:

```sh
dbops propose <db> -f ./migration.sql --note "adds idx_users_email; EXPLAIN estimates 120k rows scanned once"
```

Or piped via stdin — handy when generating SQL with a script, which you
should always prefer over hand-writing bulk `INSERT`/`UPDATE` rows:

```sh
awk -f to-sql.awk data.csv | dbops propose <db> --note "bulk import of N rows"
```

Output looks like:

```
proposal a1b2c3d4 created.
next: run `sudo dbops apply a1b2c3d4` in a terminal you control.
inspect: `dbops proposal show a1b2c3d4`
```

By default `propose` also runs an `EXPLAIN` dry-run (inside a read-only
transaction) and stores the plan on the proposal. If the dry-run fails,
the proposal still gets created but its EXPLAIN field records the error —
fix the SQL and submit a new proposal rather than trying to repair the
old one.

### 3. Hand off to the human

Tell the user in plain language, with the exact command in a code block:

> Proposal `a1b2c3d4` is ready. Run this in a terminal you control:
>
> ```sh
> sudo dbops apply a1b2c3d4
> ```
>
> I'll wait for the result.

You do NOT run `sudo dbops apply` yourself. The CLI refuses when
`euid != 0`, and even if it didn't, running it yourself would defeat the
human-in-the-loop guarantee.

### 4. Verify when the user returns

```sh
dbops proposal show a1b2c3d4
dbops proposal show a1b2c3d4 --json    # when you need to parse it
```

Interpret the `status` field:

- `applied`  — done. Report `result.affected_rows` and compare against
  your step-1 count. If they disagree, explain why to the user.
- `pending`  — the user hasn't applied it yet. Wait, don't nag.
- `rejected` — the user said no (or you ran `dbops proposal reject`).
  Ask what to change before re-proposing.
- `expired`  — over 7 days old. Submit a fresh proposal; don't try to
  mutate this one.

## Red lines (stop and check before you propose)

- **No WHERE clause on UPDATE / DELETE?** Almost always a bug. If it's
  genuinely intentional, the `--note` must spell out why AND cite the
  row count you measured in step 1.
- **DDL (CREATE / ALTER / DROP)?** The `--note` must include a rollback
  plan — either the reverse DDL verbatim, or an explicit "this is
  irreversible; user confirmed in chat at HH:MM."
- **DROP / TRUNCATE / anything that removes data?** Ask the user in chat
  first ("I'm about to propose `DROP TABLE foo` — confirm?"), quote
  their reply in the `--note`, then propose.
- **Production database?** If the `notes` field in `dbops list`
  mentions prod / production / live, double the caution: measure twice,
  propose with the full row count, and tell the user in chat which
  alias you're targeting before the proposal.

## Multi-statement SQL

A proposal is a single `Exec` call. Whether multiple statements in one
proposal work depends on the DSN:

- MySQL — requires `multiStatements=true` in the DSN.
- Postgres (pgx/v5 stdlib) — requires `prefer_simple_protocol=true` in
  the DSN.

If you don't know whether the DSN has these flags, DON'T assume. Either
check `examples/config.toml` on the host, ask the user, or split the
change into one proposal per statement (which is the safer default
anyway — each statement is reviewable and revertible on its own).

## Inspecting and recovering

```sh
dbops proposal list                     # all non-expired proposals, newest first
dbops proposal list --status pending    # filter by status
dbops proposal list --json              # for parsing
dbops proposal show <id>                # full details including EXPLAIN
dbops proposal show <id> --json
dbops proposal reject <id>              # mark a pending proposal as rejected
```

Every `apply` attempt — success or failure — is appended to
`~/.local/share/dbops/audit.log`. Read it when you need history:

```sh
dbops query <db> --sql "..."  # NOT this; audit.log is a plain-text file
```

Just read it directly with the Read tool — it's one line per apply,
RFC3339 timestamp first.
