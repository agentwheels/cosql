---
name: cosql
description: Use the cosql CLI for all PostgreSQL/MySQL operations. Read paths run directly; write paths always go through propose → sudo apply.
---

# cosql — safe database operations

Any SQL you run against a Postgres or MySQL database on this machine MUST go
through the `cosql` CLI. Never call `psql`, `mysql`, `mysqlcli`, or any
other raw client directly.

Why: the binary enforces a read-only transaction for queries, and splits
writes into `propose` (what you can do) vs `apply` (what only the human
can do via `sudo`). Going around it breaks that invariant.

## Quick orientation

Always start with `cosql list` if you don't already know what's configured.
Aliases in the output are the `<db>` argument for every other command.

```
cosql list
cosql list --json
```

## Read operations

These commands need no approval. Use `--json` whenever you intend to parse
the output.

```sh
# Ad-hoc query (wrapped in a read-only transaction; writes are DB-rejected).
# SQL can come from --sql, a file, or stdin — pick whichever is cleanest.
cosql query   <db> --sql "select ..."
cosql query   <db> -f ./question.sql
cosql query   <db> -f -                       # explicit stdin
echo "select 1"   | cosql query <db>          # implicit stdin (same thing)

# Schema browsing
cosql schema  <db>                      # list schemas/tables with rough row counts
cosql schema  <db> public.users         # columns + indexes + foreign keys

# EXPLAIN. --analyze executes the query and is still bounded by the read-only txn.
cosql explain <db> --sql "..."
cosql explain <db> --sql "..." --analyze
```

Flag order is flexible — `cosql query <db> --sql "..."` and
`cosql query --sql "..." <db>` both work.

If the user asks "show me X", start with `schema` (to confirm shape) then
`query` (to pull data). For performance questions, go `explain` first.

## Write operations — must use propose → apply

Any write (INSERT / UPDATE / DELETE / DDL) MUST go through:

```
agent:   cosql propose <db> --sql "..." --note "..."
human:   sudo cosql apply <id>
agent:   cosql proposal show <id>    # verify
```

The detailed workflow — including row-counting before proposing, red
lines, DDL rollback rules, multi-statement handling, and the full
handoff script — lives in **[references/write-ops.md](references/write-ops.md)**.
Load that reference before you propose anything.

## Troubleshooting quick table

| Error | Fix |
|---|---|
| `config not found` | Tell the user to run `make install-example-config`. |
| `config ... has permissions XXX; must be 0600` | Tell the user to run `chmod 600 ~/.config/cosql/config.toml`. |
| `no such db alias: "foo"` | Run `cosql list`; ask the user to add the missing `[db.foo]` entry. |
| `connect <alias>: ...` | DSN is wrong or server is down. Don't retry blindly; show the error to the user. |
| `cannot execute ... in a read-only transaction` | You used `query` for a write. Switch to `propose` (see [references/write-ops.md](references/write-ops.md)). |
| `apply must run with sudo` | You tried to run apply yourself. Hand off to the user. |

## What NOT to do

- Don't suggest `psql -c`, `mysql -e`, or any alternative client.
- Don't suggest editing the config file for the user; describe the change
  and let them do it.
- Don't chain `propose` with an expectation that it runs — it never does.
- Don't share or memorize the user's database passwords; they live in the
  600-protected config, and the agent has no reason to quote them.

## Handing commands to the user

Whenever you need the user to run something, give them the **complete
copy-paste-ready command in a fenced code block**. Never ask them to
derive it from prose. This applies to every category above — apply,
chmod, config edits, install, diagnostics.

Good:

> Proposal `a1b2c3d4` is ready. Run this in a terminal you control:
>
> ```sh
> sudo cosql apply a1b2c3d4
> ```
>
> I'll wait for the result.

Bad:

> Please apply proposal a1b2c3d4 with sudo.

For config edits, paste the final TOML block, not "add a flag":

> Add this to `~/.config/cosql/config.toml`:
>
> ```toml
> [db.staging]
> driver = "postgres"
> dsn    = "postgres://ro:secret@staging.example:5432/app?sslmode=require"
> notes  = "read replica"
> ```

For permission errors, paste the exact fix:

> ```sh
> chmod 600 ~/.config/cosql/config.toml
> ```

Sequence multiple commands with short ordered labels, but keep every
command in its own code block. Include absolute paths, every flag, and
every literal value the user needs.
