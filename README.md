# dbops

Safe PostgreSQL / MySQL CLI for Claude Code agents.

- **Reads run free** — `query`, `schema`, `explain` are wrapped in a
  read-only DB transaction, so even a mis-typed `UPDATE` is refused.
- **Writes need a human** — agents create a **proposal**; only
  `sudo dbops apply <id>` actually executes it. The sudo password is the
  human-in-the-loop signal.

## Install — one-line prompt for any coding agent

Works with any agent that can run shell commands and read a URL
(Claude Code, Cursor, aider, Codex, Copilot CLI, Gemini CLI, …). Paste
this verbatim:

> Set up the `dbops` CLI from https://github.com/oriengy/dbops.
> (1) Install the binary: `go install github.com/oriengy/dbops/cmd/dbops@latest`
> and make sure `$(go env GOPATH)/bin` is on my PATH.
> (2) Fetch and read `skills/dbops/SKILL.md` and
> `skills/dbops/references/write-ops.md` from the repo — they describe the
> propose → sudo-apply workflow you MUST follow when I ask for database work.
> (3) Bootstrap the config: copy `examples/config.toml` from the repo to
> `~/.config/dbops/config.toml` with mode `0600`, then ask me for the DSNs
> and edit the file. Don't proceed until `dbops list` shows the aliases I want.

Requires Go 1.22+ on PATH.

## Install — Claude Code plugin (one step)

```
/plugin marketplace add oriengy/dbops
/plugin install dbops@dbops
```

The `SessionStart` hook auto-builds the binary; then copy
`examples/config.toml` to `~/.config/dbops/config.toml` (`chmod 600`)
and fill in the DSNs.

## Install — manually

```sh
make install                       # builds + installs /usr/local/bin/dbops and the Skill
make install-example-config        # copies examples/config.toml → ~/.config/dbops/config.toml
chmod 600 ~/.config/dbops/config.toml
$EDITOR   ~/.config/dbops/config.toml
```

## Usage

```sh
# Reads — no sudo
dbops list
dbops query   local_pg --sql "select count(*) from users"
dbops schema  local_pg public.users
dbops explain local_pg --sql "..."

# Write — agent side
dbops propose local_pg --sql "update users set active=false where id=7" \
  --note "GDPR delete — ticket #1234"
# => proposal <id> created. next: run `sudo dbops apply <id>`

# Write — human side
sudo dbops apply <id>

# Inspect
dbops proposal list
dbops proposal show <id>
dbops proposal reject <id>
```

All commands accept `--json` for structured output. SQL can come from
`--sql`, `-f <file>`, or stdin.

## Config

See [`examples/config.toml`](examples/config.toml). Each DB is a
`[db.<alias>]` table with `driver = "postgres" | "mysql"` and a DSN.
The config file must be mode `0600` or dbops refuses to start.

For multi-statement proposals, enable it at the DSN level:
`multiStatements=true` (MySQL) or `prefer_simple_protocol=true` (Postgres).

## How the write gate works

| Surface | Protection |
|---|---|
| `query`, `explain` | `BeginTx(ReadOnly: true)` — writes rejected by the DB |
| `propose` | Writes a proposal file; never executes SQL |
| `apply` | Refuses to start unless `euid == 0` |
| Human check | The `sudo` password. Don't configure `NOPASSWD` |

Under sudo, dbops rewrites `$HOME` to `$SUDO_USER`'s home so paths still
resolve to your user's files. The tool trusts the OS integrity boundary —
it doesn't try to detect tampered sudo.

## Storage

- `~/.config/dbops/config.toml`              — DB aliases (mode 0600)
- `~/.local/share/dbops/proposals/<id>.json` — individual proposals
- `~/.local/share/dbops/audit.log`           — append-only apply log

## Further reading

- [`skills/dbops/SKILL.md`](skills/dbops/SKILL.md) — how agents should drive the CLI
- [`skills/dbops/references/write-ops.md`](skills/dbops/references/write-ops.md) — full write workflow, red lines, multi-statement rules

## Development

```sh
make build          # bin/dbops
make test           # go test ./...
make tidy           # go mod tidy
```

## License

MIT — see [`LICENSE`](LICENSE).
