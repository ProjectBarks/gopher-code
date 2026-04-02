#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BINARY="$PROJECT_ROOT/../research/claude-code-source-build/dist/cli.js"

PASS=0
FAIL=0

pass() { echo -e "  \033[32m✓\033[0m $1"; PASS=$((PASS + 1)); }
fail() { echo -e "  \033[31m✗\033[0m $1"; FAIL=$((FAIL + 1)); }

echo "=== Gopher Code TypeScript Binary Validation ==="
echo ""

# Check node
if command -v node &>/dev/null; then
    pass "Node.js available ($(node --version))"
else
    fail "Node.js not found"
    exit 1
fi

# Check binary exists
if [ -f "$BINARY" ]; then
    pass "Binary exists at $BINARY"
else
    fail "Binary not found at $BINARY"
    exit 1
fi

# Check version
VERSION=$(node "$BINARY" --version 2>&1 || true)
if echo "$VERSION" | grep -q "2.1.88"; then
    pass "Version: $VERSION"
else
    fail "Expected version 2.1.88, got: $VERSION"
fi

# Check help output
HELP=$(node "$BINARY" --help 2>&1 || true)

for flag in "--tools" "--output-format" "--print" "--dangerously-skip-permissions" "--model" "--allowed-tools" "--system-prompt"; do
    if echo "$HELP" | grep -q -- "$flag"; then
        pass "Help contains $flag"
    else
        fail "Help missing $flag"
    fi
done

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
