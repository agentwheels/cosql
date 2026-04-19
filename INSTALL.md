# Installing dbops (for coding agents)

You are a coding agent. Follow these steps to install and configure the
`dbops` CLI on this machine. After install, you MUST use `dbops` for
every PostgreSQL/MySQL operation — never call `psql`, `mysql`, or any
other raw client.

## 1. Install the binary

Requires Go 1.22+ on PATH.

```sh
go install github.com/oriengy/dbops/cmd/dbops@latest
```

Make sure `$(go env GOPATH)/bin` is on the user's PATH. If `dbops list`
prints `command not found`, tell the user to add it.

## 2. Load the skill rules

Fetch and read these two files, then obey them for the rest of the session:

- https://raw.githubusercontent.com/oriengy/dbops/main/skills/dbops/SKILL.md
- https://raw.githubusercontent.com/oriengy/dbops/main/skills/dbops/references/write-ops.md

Key invariants from those docs:

- Read paths (`query`, `schema`, `explain`) run unattended — they're
  wrapped in a read-only DB transaction.
- Writes MUST go through `dbops propose` → `sudo dbops apply <id>`.
  Never run `sudo dbops apply` yourself; hand the command to the user.
- Hand the user copy-paste-ready commands in fenced code blocks.

## 3. Bootstrap the config

```sh
mkdir -p ~/.config/dbops
curl -fsSL https://raw.githubusercontent.com/oriengy/dbops/main/examples/config.toml \
  -o ~/.config/dbops/config.toml
chmod 600 ~/.config/dbops/config.toml
```

Then ask the user for each database they want to register and edit
`~/.config/dbops/config.toml` accordingly. `dbops` refuses to start if
the file is not mode `0600`.

## 4. Verify

```sh
dbops list
```

If it prints the aliases the user added, you're done. If not, surface
the error verbatim — don't retry blindly.
