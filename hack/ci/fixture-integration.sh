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

mkdir -p "${workdir}/"{root,a,b,c,d,e}

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

for m in root a b c d e; do
  cat >"${workdir}/${m}/dummy.go" <<EOF
package ${m}
EOF
done

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

popd >/dev/null

echo "fixture integration checks passed"
