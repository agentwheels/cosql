#!/bin/sh
# Build the cosql binary on first plugin activation.
#
# Claude Code automatically prepends ${CLAUDE_PLUGIN_ROOT}/bin/ to PATH while
# the plugin is enabled, so the compiled binary lands exactly where the
# skill expects to find it.

set -eu

cd "$CLAUDE_PLUGIN_ROOT"

if [ -x bin/cosql ]; then
    exit 0
fi

if ! command -v go >/dev/null 2>&1; then
    echo "cosql plugin: Go 1.22+ is required to build the binary." >&2
    echo "             Install Go, then rm '$CLAUDE_PLUGIN_ROOT/bin/cosql' to retry." >&2
    exit 1
fi

echo "cosql plugin: building bin/cosql..." >&2
go build -o bin/cosql ./cmd/cosql
echo "cosql plugin: ready ($(./bin/cosql 2>&1 | head -1 || echo bin/cosql))" >&2
