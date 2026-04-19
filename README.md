# dbops

A small CLI for day-to-day PostgreSQL / MySQL operations, designed to be
safely driven by a Claude Code agent via the companion Skill at
`skill/SKILL.md`.

## Why

- Read paths (`list`, `query`, `schema`, `explain`) run unattended — queries
  always execute inside a read-only transaction, so even an agent that
  accidentally sends `UPDATE` gets rejected by the database.
- Write paths are split: an agent can create a **proposal**, but only
  `sudo dbops apply <id>` can actually run it. The sudo password is the real
  "human in the loop" signal — the agent doesn't know it and can't feed it
  to sudo's `/dev/tty` prompt.
- Multiple databases (any mix of Postgres and MySQL) are managed by DSN
  aliases in `~/.config/dbops/config.toml`.

## Install

```sh
# Build and install the binary. Requires Go 1.22+.
make install                       # installs /usr/local/bin/dbops + Skill

# One-time config setup. The file MUST be mode 0600; dbops refuses to start otherwise.
make install-example-config        # copies examples/config.toml → ~/.config/dbops/config.toml
chmod 600 ~/.config/dbops/config.toml
$EDITOR   ~/.config/dbops/config.toml
```

The Skill is installed into `~/.claude/skills/dbops/SKILL.md` so Claude
Code can load it globally.

## Config

See `examples/config.toml`. Each database is a `[db.<alias>]` table with
`driver = "postgres" | "mysql"` and a DSN.

**Multi-statement notice**: a proposal's SQL is sent as a single Exec call.
If you want to ship several statements in one proposal, enable it at the
DSN level:

- MySQL: add `multiStatements=true`
- Postgres: add `prefer_simple_protocol=true`

Otherwise, split multi-statement changes into one proposal per statement.

## Usage

```sh
# Read paths (no sudo)
dbops list
dbops query  local_pg --sql "select count(*) from users"
dbops schema local_pg
dbops schema local_pg public.users
dbops explain local_pg --sql "select * from users where email = 'a@b'"

# Write path — agent side
dbops propose local_pg --sql "update users set active=false where id=7" \
  --note "GDPR delete — ticket #1234"
# => proposal <id> created. next: run `sudo dbops apply <id>`

# Write path — human side
sudo dbops apply <id>              # shows the SQL + EXPLAIN, asks y/N, runs it

# Inspect
dbops proposal list
dbops proposal show <id>
dbops proposal reject <id>
```

All commands take `--json` to emit structured output (useful for the
agent to parse).

## Storage layout

- `~/.config/dbops/config.toml`                 — DB aliases (mode 0600)
- `~/.local/share/dbops/proposals/<id>.json`    — individual proposals
- `~/.local/share/dbops/audit.log`              — append-only apply log

Under sudo, dbops rewrites `$HOME` to `SUDO_USER`'s home so the paths
still resolve to your user's files, and chowns updated files back.

## Security boundaries

| Surface | Protection |
|---|---|
| `query`, `explain` | Wrapped in `BeginTx(ReadOnly: true)`; DB-level guard |
| `propose` | Writes a file; never executes the SQL |
| `apply` | Refuses to start unless `os.Geteuid() == 0` |
| "Is it a human?" | The `sudo` password itself. Don't configure `NOPASSWD` or tell Claude Code your password |

The CLI does **not** try to detect NOPASSWD or tampered sudo. That's
your responsibility; the tool trusts the OS integrity boundary.

## Development

```sh
make build          # bin/dbops
make test           # go test ./...
make tidy           # go mod tidy
```
