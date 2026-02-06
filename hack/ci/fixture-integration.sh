#!/usr/bin/env bash

set -euo pipefail

DEPSTAT_BIN="${1:-}"
if [[ -z "${DEPSTAT_BIN}" ]]; then
  echo "usage: $0 <depstat-binary>"
  exit 1
fi
DEPSTAT_BIN="$(cd "$(dirname "${DEPSTAT_BIN}")" && pwd)/$(basename "${DEPSTAT_BIN}")"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

mkdir -p "${workdir}/"{root,a,b,c,d,e,t}

cat >"${workdir}/root/go.mod" <<'EOF'
module example.com/root

go 1.22

require (
	example.com/a v0.0.0
	example.com/b v0.0.0
)

replace (
	example.com/a => ../a
	example.com/b => ../b
	example.com/c => ../c
	example.com/d => ../d
	example.com/e => ../e
	example.com/t => ../t
)
EOF

cat >"${workdir}/a/go.mod" <<'EOF'
module example.com/a

go 1.22

require example.com/c v0.0.0

replace example.com/c => ../c
EOF

cat >"${workdir}/b/go.mod" <<'EOF'
module example.com/b

go 1.22

require example.com/d v0.0.0

replace example.com/d => ../d
EOF

cat >"${workdir}/c/go.mod" <<'EOF'
module example.com/c

go 1.22

require example.com/a v0.0.0

replace example.com/a => ../a
EOF

cat >"${workdir}/d/go.mod" <<'EOF'
module example.com/d

go 1.22
EOF

cat >"${workdir}/e/go.mod" <<'EOF'
module example.com/e

go 1.22
EOF

# Create Go source files with real imports so `go mod why -m` works
cat >"${workdir}/root/dummy.go" <<'EOF'
package root

import (
	_ "example.com/a"
	_ "example.com/b"
)
EOF

cat >"${workdir}/a/dummy.go" <<'EOF'
package a

import _ "example.com/c"
EOF

cat >"${workdir}/b/dummy.go" <<'EOF'
package b

import _ "example.com/d"
EOF

for m in c d e t; do
  cat >"${workdir}/${m}/dummy.go" <<EOF
package ${m}
EOF
done

# Add go.mod for test-only module t (will be required in a later commit)
cat >"${workdir}/t/go.mod" <<'EOF'
module example.com/t

go 1.22
EOF

pushd "${workdir}/root" >/dev/null

git init -q
git config user.name "depstat-ci"
git config user.email "depstat-ci@example.com"
git add .
git commit -q -m "initial fixture graph"

echo "==> Testing stats --json..."
"${DEPSTAT_BIN}" stats --json > stats.json
jq -e '.directDependencies >= 2 and .transitiveDependencies >= 1 and .totalDependencies >= 3 and .maxDepthOfDependencies >= 2' stats.json >/dev/null \
  || { echo "FAIL: stats JSON field values out of range"; exit 1; }

echo "==> Testing stats --csv..."
"${DEPSTAT_BIN}" stats --csv > stats.csv
grep -q '^Direct,Transitive,Total,MaxDepth$' stats.csv \
  || { echo "FAIL: stats CSV missing expected header"; exit 1; }

echo "==> Testing list..."
"${DEPSTAT_BIN}" list > list.txt
grep -q '^example.com/a$' list.txt \
  || { echo "FAIL: list missing example.com/a"; exit 1; }
grep -q '^example.com/c$' list.txt \
  || { echo "FAIL: list missing example.com/c"; exit 1; }

echo "==> Testing graph --show-edge-types..."
"${DEPSTAT_BIN}" graph --show-edge-types
[[ -s graph.dot ]] \
  || { echo "FAIL: graph.dot is empty or missing"; exit 1; }
grep -q 'edgetype="direct"' graph.dot \
  || { echo "FAIL: graph.dot missing direct edges"; exit 1; }
grep -q 'edgetype="transitive"' graph.dot \
  || { echo "FAIL: graph.dot missing transitive edges"; exit 1; }

echo "==> Testing cycles --json..."
"${DEPSTAT_BIN}" cycles --json > cycles.json
jq -e '.cycles != null' cycles.json >/dev/null \
  || { echo "FAIL: cycles JSON missing .cycles key"; exit 1; }

echo "==> Testing why --json..."
"${DEPSTAT_BIN}" why example.com/c --json > why.json
jq -e '.target == "example.com/c" and .found == true and (.paths | length >= 1)' why.json >/dev/null \
  || { echo "FAIL: why JSON check failed for example.com/c"; exit 1; }

echo "==> Testing why --dot..."
"${DEPSTAT_BIN}" why example.com/c --dot > why.dot
grep -q 'strict digraph' why.dot \
  || { echo "FAIL: why --dot output missing 'strict digraph'"; exit 1; }

echo "==> Testing why --svg..."
"${DEPSTAT_BIN}" why example.com/c --svg > why.svg
grep -q '<svg' why.svg \
  || { echo "FAIL: why --svg output missing '<svg' tag"; exit 1; }

echo "==> Testing completion..."
"${DEPSTAT_BIN}" completion bash > /dev/null \
  || { echo "FAIL: completion bash failed"; exit 1; }

echo "==> Preparing diff fixture (adding dependency e)..."
awk '
/^require \($/ {print; print "\texample.com/e v0.0.0"; inreq=1; next}
/^\)$/ && inreq {inreq=0}
{print}
' go.mod > go.mod.new
mv go.mod.new go.mod
git add go.mod
git commit -q -m "add dependency e"

echo "==> Testing diff --json..."
"${DEPSTAT_BIN}" diff HEAD~1 HEAD --json > diff.json
jq -e 'has("before") and has("after") and has("delta")' diff.json >/dev/null \
  || { echo "FAIL: diff JSON missing expected keys"; exit 1; }

echo "==> Testing diff --dot..."
"${DEPSTAT_BIN}" diff HEAD~1 HEAD --dot > diff.dot
grep -q 'strict digraph' diff.dot \
  || { echo "FAIL: diff --dot output missing 'strict digraph'"; exit 1; }

echo "==> Testing diff-then-why loop (Prow presubmit pattern)..."
for dep in $(jq -r '.added[]?' diff.json); do
  "${DEPSTAT_BIN}" why "${dep}" --json > /dev/null || true
done

echo "==> Preparing test-only dep fixture (adding test-only dependency t)..."
# Add require for t in go.mod
awk '
/^require \($/ {print; print "\texample.com/t v0.0.0"; inreq=1; next}
/^\)$/ && inreq {inreq=0}
{print}
' go.mod > go.mod.new
mv go.mod.new go.mod

# Add a _test.go file that imports t (making t a test-only dependency)
cat > dummy_test.go <<'EOF'
package root

import (
	"testing"

	_ "example.com/t"
)

func TestDummy(t *testing.T) {}
EOF

git add -A
git commit -q -m "add test-only dependency t"

echo "==> Testing stats --split-test-only --json..."
"${DEPSTAT_BIN}" stats --split-test-only --json > stats-split.json
jq -e '.testOnlyDependencies >= 1 and .nonTestOnlyDependencies >= 1' stats-split.json >/dev/null \
  || { echo "FAIL: stats --split-test-only did not report expected test/non-test counts"; exit 1; }

echo "==> Testing list --split-test-only..."
"${DEPSTAT_BIN}" list --split-test-only > list-split.txt
grep -q 'Non-test dependencies' list-split.txt \
  || { echo "FAIL: list --split-test-only missing non-test section"; exit 1; }
grep -q 'Test-only dependencies' list-split.txt \
  || { echo "FAIL: list --split-test-only missing test-only section"; exit 1; }
grep -q 'example.com/t' list-split.txt \
  || { echo "FAIL: list --split-test-only missing example.com/t"; exit 1; }

echo "==> Testing diff --split-test-only --json..."
"${DEPSTAT_BIN}" diff HEAD~1 HEAD --split-test-only --json > diff-split.json
jq -e '.split != null' diff-split.json >/dev/null \
  || { echo "FAIL: diff --split-test-only missing split section"; exit 1; }
# t should appear in test-only added
jq -e '.split.testOnly.added | any(. == "example.com/t")' diff-split.json >/dev/null \
  || { echo "FAIL: split.testOnly.added should include example.com/t"; exit 1; }
# t should NOT appear in non-test-only added
if jq -e '.split.nonTestOnly.added | any(. == "example.com/t")' diff-split.json >/dev/null 2>&1; then
  echo "FAIL: split.nonTestOnly.added should NOT include example.com/t"
  exit 1
fi

popd >/dev/null

echo "fixture integration checks passed"
