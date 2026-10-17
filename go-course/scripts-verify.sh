#!/usr/bin/env bash
# Usage: ./scripts-verify.sh [module ...]  — compact health check of course modules.
cd "$(dirname "$0")"
mods=("$@"); [ ${#mods[@]} -eq 0 ] && mods=($(ls modules))
for m in "${mods[@]}"; do
  echo "== $m"
  f=$(gofmt -l modules/$m solutions/$m 2>&1); [ -n "$f" ] && echo "  GOFMT: $f"
  v=$(go vet ./modules/$m/... ./solutions/$m/... 2>&1 | grep -v '^#' | head -5); [ -n "$v" ] && echo "  VET: $v"
  s=$(go test -race -timeout 120s ./solutions/$m/... 2>&1 | grep -E '^(FAIL|---|panic)' | head -5); [ -n "$s" ] && echo "  SOLUTION FAIL: $s"
  st=$(go test -timeout 60s ./modules/$m/... 2>&1 | grep -E '^(ok|FAIL)\s' )
  echo "$st" | grep -q 'build failed' && echo "  STARTER BUILD FAIL: $(echo "$st" | grep 'build failed')"
  echo "$st" | grep '^ok' | sed 's/^/  STARTER PASSES (should fail?): /'
  for t in $(cd modules/$m && find . -name '*_test.go'); do
    [ -f solutions/$m/$t ] || { echo "  MISSING in solutions: $t"; continue; }
    [ "$m" = "04-testing" ] || cmp -s modules/$m/$t solutions/$m/$t || echo "  TEST DIFFERS: $t"
  done
  if command -v docker >/dev/null 2>&1; then
    [ -f modules/$m/compose.yaml ] || echo "  MISSING modules/$m/compose.yaml"
    [ -f modules/$m/compose.yaml ] && { c=$(docker compose -f modules/$m/compose.yaml config -q 2>&1); [ -n "$c" ] && echo "  COMPOSE: $c"; }
  fi
  for r in modules/$m/README.md solutions/$m/REVIEW.md; do [ -f $r ] && echo "  $(basename $r): $(wc -l < $r) lines" || echo "  MISSING $r"; done
done
