# Installing cosql (for coding agents)

You are a coding agent. Follow these steps to install and configure the
`cosql` CLI on this machine. After install, you MUST use `cosql` for
every PostgreSQL/MySQL operation — never call `psql`, `mysql`, or any
other raw client.

## 1. Install the binary

Download the pre-built binary for this host and put it on PATH:

```sh
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -fsSL "https://github.com/agentwheels/cosql/releases/latest/download/cosql-${OS}-${ARCH}" \
  -o /tmp/cosql
chmod +x /tmp/cosql
sudo mv /tmp/cosql /usr/local/bin/cosql
```

If sudo isn't available, install to `~/.local/bin/cosql` instead and
make sure that directory is on PATH.

Fallback — build from source if you have Go 1.22+ and would rather not
trust a pre-built binary:

```sh
go install github.com/agentwheels/cosql/cmd/cosql@latest
```

Verify with `cosql --help`.

## 2. Load the skill rules

Fetch and read these two files, then obey them for the rest of the session:

- https://raw.githubusercontent.com/agentwheels/cosql/main/skills/cosql/SKILL.md
- https://raw.githubusercontent.com/agentwheels/cosql/main/skills/cosql/references/write-ops.md

Key invariants from those docs:

- Read paths (`query`, `schema`, `explain`) run unattended — they're
  wrapped in a read-only DB transaction.
- Writes MUST go through `cosql propose` → `sudo cosql apply <id>`.
  Never run `sudo cosql apply` yourself; hand the command to the user.
- Hand the user copy-paste-ready commands in fenced code blocks.

## 3. Bootstrap the config

```sh
mkdir -p ~/.config/cosql
curl -fsSL https://raw.githubusercontent.com/agentwheels/cosql/main/examples/config.toml \
  -o ~/.config/cosql/config.toml
chmod 600 ~/.config/cosql/config.toml
```

Then ask the user for each database they want to register and edit
`~/.config/cosql/config.toml` accordingly. `cosql` refuses to start if
the file is not mode `0600`.

## 4. Verify

```sh
cosql list
```

If it prints the aliases the user added, you're done. If not, surface
the error verbatim — don't retry blindly.
