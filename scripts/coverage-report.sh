#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "=== Test Coverage Report ==="
echo ""

# Count tests
TOTAL=$(go test ./... -v -count=1 2>&1 | grep -c "=== RUN" || true)
PASS=$(go test ./... -v -count=1 2>&1 | grep "PASS:" | wc -l | tr -d ' ')
FAIL=$(go test ./... -v -count=1 2>&1 | grep "FAIL:" | grep -v "^FAIL" | grep -v "^---" | wc -l | tr -d ' ')
SKIP=$(go test ./... -v -count=1 2>&1 | grep "SKIP:" | wc -l | tr -d ' ')

echo "Total tests:  $TOTAL"
echo "Passing:      $PASS"
echo "Failing:      $FAIL"
echo "Skipped:      $SKIP"
echo ""

# Count golden files
GOLDEN=$(find testdata -type f | wc -l | tr -d ' ')
echo "Golden files: $GOLDEN"
echo ""

# Count test files
TEST_FILES=$(find . -name "*_test.go" -not -path "./.claude/*" | wc -l | tr -d ' ')
echo "Test files:   $TEST_FILES"
echo ""

# Per-package breakdown
echo "=== Per-Package ==="
for pkg in $(go list ./... 2>/dev/null | grep -v testharness | grep -v cmd | grep -v internal/cli); do
  short=$(echo "$pkg" | sed 's|.*gopher/||')
  count=$(go test "$pkg" -v -count=1 2>&1 | grep -c "=== RUN" 2>/dev/null || echo "0")
  if [ "$count" -gt 0 ]; then
    echo "  $short: $count"
  fi
done

echo ""
echo "=== Coverage ==="
go test ./... -coverprofile=/tmp/gopher-coverage.out -count=1 2>/dev/null
COVERAGE=$(go tool cover -func=/tmp/gopher-coverage.out 2>/dev/null | grep "^total:" | awk '{print $3}')
echo "Line coverage: $COVERAGE"
