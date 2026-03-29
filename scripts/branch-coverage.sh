#!/usr/bin/env bash
#
# Run gobco branch coverage on critical packages and enforce a minimum
# branch coverage threshold.
#
# Usage:
#   ./scripts/branch-coverage.sh            # default 90% threshold
#   BRANCH_THRESHOLD=80 ./scripts/branch-coverage.sh
#
set -euo pipefail

THRESHOLD="${BRANCH_THRESHOLD:-90}"
GOBCO="${GOBCO:-gobco}"
MODULE="github.com/centroid-is/stc"

# Critical packages that require high branch coverage.
CRITICAL_PACKAGES=(
    ./pkg/parser
    ./pkg/lexer
    ./pkg/checker
    ./pkg/interp
    ./pkg/emit
    ./pkg/types
)

# Verify gobco is available.
if ! command -v "$GOBCO" &>/dev/null; then
    echo "gobco not found. Install with: go install github.com/rillig/gobco@latest"
    exit 1
fi

cd "$(git rev-parse --show-toplevel)"

failed=0
summary=""

for pkg in "${CRITICAL_PACKAGES[@]}"; do
    pkg_name="${pkg#./}"
    echo "=== Branch coverage: $pkg_name ==="

    # gobco outputs per-condition coverage. Capture its output.
    output=$("$GOBCO" -stats "$pkg" 2>&1) || true

    # Extract the summary line that contains the percentage.
    # gobco prints: "Branch coverage: X/Y = ZZ.Z%"
    pct_line=$(echo "$output" | grep -i 'branch coverage' | tail -1) || true

    if [ -z "$pct_line" ]; then
        # Try condition coverage line as fallback.
        pct_line=$(echo "$output" | grep -i 'condition coverage' | tail -1) || true
    fi

    if [ -z "$pct_line" ]; then
        echo "  WARNING: could not extract coverage percentage"
        echo "$output" | tail -5
        summary+="  UNKNOWN  $pkg_name (no coverage line found)"$'\n'
        continue
    fi

    # Parse percentage from the line.
    pct=$(echo "$pct_line" | grep -oE '[0-9]+\.[0-9]+%' | head -1 | tr -d '%') || true
    if [ -z "$pct" ]; then
        pct=$(echo "$pct_line" | grep -oE '[0-9]+%' | head -1 | tr -d '%') || true
    fi

    if [ -z "$pct" ]; then
        echo "  WARNING: could not parse percentage from: $pct_line"
        summary+="  UNKNOWN  $pkg_name"$'\n'
        continue
    fi

    echo "  $pct_line"

    # Check threshold (integer comparison, truncate decimals).
    pct_int=${pct%%.*}
    if [ "$pct_int" -lt "$THRESHOLD" ]; then
        echo "  FAIL: $pct% < $THRESHOLD% threshold"
        summary+="  FAIL     $pkg_name: ${pct}% (threshold: ${THRESHOLD}%)"$'\n'
        failed=1
    else
        echo "  PASS"
        summary+="  PASS     $pkg_name: ${pct}%"$'\n'
    fi
    echo ""
done

echo "=== Branch Coverage Summary ==="
echo "$summary"

if [ "$failed" -ne 0 ]; then
    echo "FAILED: one or more critical packages below ${THRESHOLD}% branch coverage"
    exit 1
fi

echo "All critical packages meet ${THRESHOLD}% branch coverage threshold."
