# cosql

Safe PostgreSQL / MySQL CLI for Claude Code agents.

- **Reads run free** — `query`, `schema`, `explain` are wrapped in a
  read-only DB transaction, so even a mis-typed `UPDATE` is refused.
- **Writes need a human** — agents create a **proposal**; only
  `sudo cosql apply <id>` actually executes it. The sudo password is the
  human-in-the-loop signal.

## Install — one-liner for any coding agent

Paste into any agent that can run shell commands and fetch a URL
(Cursor, aider, Codex, Copilot CLI, Gemini CLI, Claude Code, …):

> Install cosql following https://raw.githubusercontent.com/agentwheels/cosql/main/INSTALL.md

The linked doc downloads the right pre-built binary for the host — no
Go toolchain needed. Go 1.22+ source build is offered as a fallback.

## Install — Claude Code plugin (one step)

```
/plugin marketplace add agentwheels/cosql
/plugin install cosql@agentwheels
```

The `SessionStart` hook auto-builds the binary; ask the agent to
bootstrap the config.

## Usage

```sh
# Reads — no sudo
cosql list
cosql query   local_pg --sql "select count(*) from users"
cosql schema  local_pg public.users
cosql explain local_pg --sql "..."

# Write — agent side
cosql propose local_pg --sql "update users set active=false where id=7" \
  --note "GDPR delete — ticket #1234"
# => proposal <id> created. next: run `sudo cosql apply <id>`

# Write — human side
sudo cosql apply <id>

# Inspect
cosql proposal list
cosql proposal show <id>
cosql proposal reject <id>
```

All commands accept `--json` for structured output. SQL can come from
`--sql`, `-f <file>`, or stdin.

## Config

See [`examples/config.toml`](examples/config.toml). Each DB is a
`[db.<alias>]` table with `driver = "postgres" | "mysql"` and a DSN.
The config file must be mode `0600` or cosql refuses to start.

For multi-statement proposals, enable it at the DSN level:
`multiStatements=true` (MySQL) or `prefer_simple_protocol=true` (Postgres).

## How the write gate works

| Surface | Protection |
|---|---|
| `query`, `explain` | `BeginTx(ReadOnly: true)` — writes rejected by the DB |
| `propose` | Writes a proposal file; never executes SQL |
| `apply` | Refuses to start unless `euid == 0` |
| Human check | The `sudo` password. Don't configure `NOPASSWD` |

Under sudo, cosql rewrites `$HOME` to `$SUDO_USER`'s home so paths still
resolve to your user's files. The tool trusts the OS integrity boundary —
it doesn't try to detect tampered sudo.

## Storage

- `~/.config/cosql/config.toml`              — DB aliases (mode 0600)
- `~/.local/share/cosql/proposals/<id>.json` — individual proposals
- `~/.local/share/cosql/audit.log`           — append-only apply log

## Further reading

- [`skills/cosql/SKILL.md`](skills/cosql/SKILL.md) — how agents should drive the CLI
- [`skills/cosql/references/write-ops.md`](skills/cosql/references/write-ops.md) — full write workflow, red lines, multi-statement rules

## Development

```sh
make build          # bin/cosql
make test           # go test ./...
make tidy           # go mod tidy
```

## License

MIT — see [`LICENSE`](LICENSE).
