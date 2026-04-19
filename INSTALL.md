# Installing dbops (for coding agents)

You are a coding agent. Follow these steps to install and configure the
`dbops` CLI on this machine. After install, you MUST use `dbops` for
every PostgreSQL/MySQL operation — never call `psql`, `mysql`, or any
other raw client.

## 1. Install the binary

Download the pre-built binary for this host and put it on PATH:

```sh
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -fsSL "https://github.com/oriengy/dbops/releases/latest/download/dbops-${OS}-${ARCH}" \
  -o /tmp/dbops
chmod +x /tmp/dbops
sudo mv /tmp/dbops /usr/local/bin/dbops
```

If sudo isn't available, install to `~/.local/bin/dbops` instead and
make sure that directory is on PATH.

Fallback — build from source if you have Go 1.22+ and would rather not
trust a pre-built binary:

```sh
go install github.com/oriengy/dbops/cmd/dbops@latest
```

Verify with `dbops --help`.

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
