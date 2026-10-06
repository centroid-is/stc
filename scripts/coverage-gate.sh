#!/usr/bin/env bash
#
# Reproduce the CI coverage gate (.github/workflows/coverage.yml) locally.
#
# Builds the same merged profile CI builds (unit tests with -coverpkg=./...
# plus exec-based cmd/stc coverage via GOCOVERDIR), then prints per-package
# statement coverage for the packages gated in .testcoverage.yml and exits
# non-zero if any of them is below its threshold.
#
# Usage:
#   ./scripts/coverage-gate.sh
#   STC_COVER_TOOL=1 ./scripts/coverage-gate.sh   # also run go-test-coverage (needs network)
#
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

MODULE="github.com/centroid-is/stc"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/stc-coverage.XXXXXX")"
COVDATA="$WORK/covdata"
mkdir -p "$COVDATA"

echo "==> unit tests (-coverpkg=./...)"
go test -coverprofile="$WORK/unit.txt" -covermode=atomic \
    -coverpkg=./... ./... -count=1 -timeout 5m >"$WORK/unit.log" 2>&1 || {
    cat "$WORK/unit.log"
    echo "unit tests failed" >&2
    exit 1
}

echo "==> exec-based tests (cmd/stc, GOCOVERDIR)"
GOCOVERDIR="$COVDATA" go test ./cmd/stc/ -count=1 -timeout 5m >"$WORK/exec.log" 2>&1 || {
    cat "$WORK/exec.log"
    echo "exec-based tests failed" >&2
    exit 1
}

if ls "$COVDATA"/covcounters.* >/dev/null 2>&1; then
    go tool covdata textfmt -i="$COVDATA" -o="$WORK/exec.txt"
fi

cp "$WORK/unit.txt" "$WORK/coverage.txt"
if [ -f "$WORK/exec.txt" ]; then
    tail -n +2 "$WORK/exec.txt" >>"$WORK/coverage.txt"
fi

# Thresholds mirror .testcoverage.yml (override list and total).
echo
awk -v module="$MODULE/" '
BEGIN {
    n = split("pkg/parser:95 pkg/lexer:95 pkg/checker:94 pkg/interp:95 pkg/types:95 pkg/emit:95", rows, " ")
    for (i = 1; i <= n; i++) {
        split(rows[i], kv, ":")
        order[i] = kv[1]
        thresh[kv[1]] = kv[2]
    }
    totalThresh = 85
}
NR == 1 && /^mode:/ { next }
{
    key = $1
    file = key
    sub(/:.*/, "", file)
    sub("^" module, "", file)
    # .testcoverage.yml exclude paths
    if (file ~ /^cmd\/.*\/main\.go$/ || file ~ /^pkg\/version\//) next
    if (!(key in stmts)) {
        stmts[key] = $2
        pkgdir = file
        sub(/\/[^\/]*$/, "", pkgdir)
        pkgOf[key] = pkgdir
    }
    if ($3 > 0) hit[key] = 1
}
END {
    for (k in stmts) {
        p = pkgOf[k]
        tot[p] += stmts[k]; all += stmts[k]
        if (k in hit) { cov[p] += stmts[k]; allCov += stmts[k] }
    }
    fail = 0
    printf "%-14s %10s %8s %6s  %s\n", "package", "covered", "percent", "min", "result"
    for (i = 1; i <= n; i++) {
        p = order[i]
        pct = tot[p] ? 100 * cov[p] / tot[p] : 0
        res = (pct >= thresh[p]) ? "PASS" : "FAIL"
        if (res == "FAIL") fail = 1
        printf "%-14s %5d/%-5d %7.2f%% %5d%%  %s\n", p, cov[p], tot[p], pct, thresh[p], res
    }
    pct = all ? 100 * allCov / all : 0
    res = (pct >= totalThresh) ? "PASS" : "FAIL"
    if (res == "FAIL") fail = 1
    printf "%-14s %5d/%-5d %7.2f%% %5d%%  %s\n", "total", allCov, all, pct, totalThresh, res
    exit fail
}
' "$WORK/coverage.txt" && status=0 || status=$?

if [ "${STC_COVER_TOOL:-0}" = "1" ]; then
    echo
    echo "==> go-test-coverage (.testcoverage.yml)"
    go run github.com/vladopajic/go-test-coverage/v2@latest \
        --config .testcoverage.yml --profile "$WORK/coverage.txt" || status=1
fi

echo
echo "merged profile: $WORK/coverage.txt"
exit "$status"
