#!/usr/bin/env bash

set -euo pipefail

DEPSTAT_BIN="${1:-}"
K8S_DIR="${2:-}"
ARTIFACT_DIR="${3:-}"

if [[ -z "${DEPSTAT_BIN}" || -z "${K8S_DIR}" ]]; then
  echo "usage: $0 <depstat-binary> <kubernetes-dir> [artifact-dir]"
  exit 1
fi
DEPSTAT_BIN="$(cd "$(dirname "${DEPSTAT_BIN}")" && pwd)/$(basename "${DEPSTAT_BIN}")"
K8S_DIR="$(cd "${K8S_DIR}" && pwd)"

if [[ -z "${ARTIFACT_DIR}" ]]; then
  ARTIFACT_DIR="$(pwd)/_artifacts/kubernetes-smoke"
fi
ARTIFACT_DIR="$(mkdir -p "${ARTIFACT_DIR}" && cd "${ARTIFACT_DIR}" && pwd)"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

pushd "${K8S_DIR}" >/dev/null
export GOWORK=off

[[ -d staging/src/k8s.io ]] \
  || { echo "FATAL: staging/src/k8s.io not found in ${K8S_DIR}"; exit 1; }

main_modules="k8s.io/kubernetes$(ls staging/src/k8s.io | awk '{printf ",k8s.io/" $0}')"
start_ref="$(git rev-parse HEAD)"
base_ref="$(git rev-parse HEAD~1)"

echo "==> Testing stats (verbose, json, csv)..."
"${DEPSTAT_BIN}" stats -m "${main_modules}" -v > "${ARTIFACT_DIR}/stats.txt"
"${DEPSTAT_BIN}" stats -m "${main_modules}" --json > "${ARTIFACT_DIR}/stats.json"
"${DEPSTAT_BIN}" stats -m "${main_modules}" --csv > "${ARTIFACT_DIR}/stats.csv"
jq -e '.directDependencies >= 1 and .totalDependencies >= .directDependencies and .maxDepthOfDependencies >= 1' "${ARTIFACT_DIR}/stats.json" >/dev/null \
  || { echo "FAIL: stats JSON field values out of range"; exit 1; }
grep -q '^Direct,Transitive,Total,MaxDepth$' "${ARTIFACT_DIR}/stats.csv" \
  || { echo "FAIL: stats CSV missing expected header"; exit 1; }

echo "==> Testing list..."
"${DEPSTAT_BIN}" list > "${ARTIFACT_DIR}/list.txt"

echo "==> Testing graph --show-edge-types..."
"${DEPSTAT_BIN}" graph -m "${main_modules}" --show-edge-types
mv graph.dot "${ARTIFACT_DIR}/graph.dot"
grep -q 'edgetype="direct"' "${ARTIFACT_DIR}/graph.dot" \
  || { echo "FAIL: graph.dot missing direct edges"; exit 1; }

echo "==> Testing cycles --json..."
"${DEPSTAT_BIN}" cycles -m "${main_modules}" --json > "${ARTIFACT_DIR}/cycles.json"
jq -e '.cycles != null' "${ARTIFACT_DIR}/cycles.json" >/dev/null \
  || { echo "FAIL: cycles JSON missing .cycles key"; exit 1; }

echo "==> Testing why (dynamic target)..."
# Pick a non-main dependency namespace. k8s.io/* entries are often main modules
# when -m includes staging modules, which makes "why" report found=false.
target_dep="$("${DEPSTAT_BIN}" list | awk '/^(github.com|golang.org)\// {print; exit}')" || true
if [[ -n "${target_dep}" ]]; then
  "${DEPSTAT_BIN}" why "${target_dep}" -m "${main_modules}" --json > "${ARTIFACT_DIR}/why.json"
  jq -e '.target == "'"${target_dep}"'" and .found == true' "${ARTIFACT_DIR}/why.json" >/dev/null \
    || { echo "FAIL: why JSON check failed for ${target_dep}"; exit 1; }
else
  echo "WARNING: could not determine a target dependency for 'why' test, skipping"
fi

echo "==> Testing diff --json and --dot..."
"${DEPSTAT_BIN}" diff "${base_ref}" HEAD -m "${main_modules}" --json > "${ARTIFACT_DIR}/diff.json"
"${DEPSTAT_BIN}" diff "${base_ref}" HEAD -m "${main_modules}" --dot > "${ARTIFACT_DIR}/diff.dot"
jq -e 'has("before") and has("after") and has("delta") and has("added") and has("removed")' "${ARTIFACT_DIR}/diff.json" >/dev/null \
  || { echo "FAIL: diff JSON missing expected keys"; exit 1; }
grep -q 'strict digraph' "${ARTIFACT_DIR}/diff.dot" \
  || { echo "FAIL: diff --dot output missing 'strict digraph'"; exit 1; }

echo "==> Verifying HEAD was restored after diff..."
end_ref="$(git rev-parse HEAD)"
if [[ "${start_ref}" != "${end_ref}" ]]; then
  echo "FAIL: depstat diff did not restore HEAD (start=${start_ref}, end=${end_ref})"
  exit 1
fi

popd >/dev/null

echo "kubernetes smoke checks passed"
