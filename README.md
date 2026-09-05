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
/plugin marketplace add agentwheels/marketplace
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

## 不支持事务的数据库

Lindorm 等数据库使用 MySQL 协议，但不支持 `BEGIN`。建表时可选择无事务模式：

```sh
cosql propose lindorm_ad -f create_table.sql --no-transaction --note "新增一张表；应用回滚时保留表"
sudo cosql apply <proposal-id>
```

`--no-transaction` 只用于 `propose`，执行模式会保存到 proposal。`show` 和人工执行的 `apply` 都会显示该模式；`apply` 直接执行 SQL，不开启事务。创建这种 proposal 时不执行 SQL 或 EXPLAIN，`dry-run` 明确标记为跳过，不能视为 SQL 已验证。

无事务执行失败后可能已有部分变更，cosql 不会回滚；需要先核对实际结果再决定是否重试。建议每条 DDL 单独创建 proposal。普通 proposal 和历史 proposal 仍使用原来的事务路径；`query`、`explain` 仍要求数据库支持只读事务，不能用这个参数绕过只读保护。

本选项只解决执行时的事务兼容问题，不保证所有 MySQL SQL 或 `information_schema` 查询都能用于 Lindorm。它不修改数据库别名、连接凭证或 sudo 审批方式。

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
