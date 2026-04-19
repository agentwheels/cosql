#!/bin/sh
# Build the dbops binary on first plugin activation.
#
# Claude Code automatically prepends ${CLAUDE_PLUGIN_ROOT}/bin/ to PATH while
# the plugin is enabled, so the compiled binary lands exactly where the
# skill expects to find it.

set -eu

cd "$CLAUDE_PLUGIN_ROOT"

if [ -x bin/dbops ]; then
    exit 0
fi

if ! command -v go >/dev/null 2>&1; then
    echo "dbops plugin: Go 1.22+ is required to build the binary." >&2
    echo "             Install Go, then rm '$CLAUDE_PLUGIN_ROOT/bin/dbops' to retry." >&2
    exit 1
fi

echo "dbops plugin: building bin/dbops..." >&2
go build -o bin/dbops ./cmd/dbops
echo "dbops plugin: ready ($(./bin/dbops 2>&1 | head -1 || echo bin/dbops))" >&2
