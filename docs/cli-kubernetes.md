# depstat CLI Guide for Kubernetes

This guide shows how to run `depstat` against a real Kubernetes checkout (`k8s.io/kubernetes`), including the same patterns used in Kubernetes test-infra jobs.

## Prerequisites

- `go` installed (1.22+)
- `depstat` installed:

```bash
go install github.com/kubernetes-sigs/depstat@latest
```

- Optional tools for richer output:
```bash
sudo apt-get update && sudo apt-get install -y jq graphviz
```

## Kubernetes Setup

```bash
export K8S_DIR=<your-kubernetes-checkout>   # e.g. ~/go/src/k8s.io/kubernetes
cd "${K8S_DIR}"
```

**Important:** Disable Go workspaces. Kubernetes uses `replace` directives in `go.mod` to point at staging modules. With workspaces enabled, `go mod graph` resolves differently and produces incorrect results for depstat.

```bash
export GOWORK=off
```

Build the `MAIN_MODULES` list exactly like Prow jobs. This tells depstat to treat both `k8s.io/kubernetes` and all its staging modules as "main" modules (rather than external dependencies):

```bash
MAIN_MODULES="k8s.io/kubernetes$(ls staging/src/k8s.io | awk '{printf ",k8s.io/" $0}')"
echo "${MAIN_MODULES}"
```

## Core Commands on Kubernetes

### `stats`

```bash
depstat stats -m "${MAIN_MODULES}" -v
depstat stats -m "${MAIN_MODULES}" --json > stats.json
depstat stats -m "${MAIN_MODULES}" --csv > stats.csv
depstat stats -m "${MAIN_MODULES}" --split-test-only --json > stats-split.json
```

### `list`

**Note:** `list`, `graph`, and `cycles` do not support the `--dir` flag. You must `cd` to the target directory before running them.

```bash
cd "${K8S_DIR}"
depstat list
depstat list --split-test-only
```

### `graph`

The `graph` command always writes to `./graph.dot` in the current directory.

```bash
depstat graph -m "${MAIN_MODULES}" --show-edge-types
dot -Tsvg graph.dot -o graph.svg
```

### `cycles`

```bash
depstat cycles -m "${MAIN_MODULES}"
depstat cycles -m "${MAIN_MODULES}" --json > cycles.json
```

### `why`

Pick a dependency and trace why it exists:

```bash
depstat why github.com/google/cel-go -m "${MAIN_MODULES}"
depstat why github.com/google/cel-go -m "${MAIN_MODULES}" --json > why.json
depstat why github.com/google/cel-go -m "${MAIN_MODULES}" --dot > why.dot
depstat why github.com/google/cel-go -m "${MAIN_MODULES}" --svg > why.svg
```

### `diff`

Compare dependency changes between git refs.

**Warning:** The `diff` command performs `git checkout` of the base and head refs internally to analyze each snapshot. It restores the original HEAD on success, but a crash mid-operation could leave the repo in a detached HEAD state. Ensure you have no uncommitted changes before running `diff`.

```bash
BASE_SHA=$(git rev-parse HEAD~1)
depstat diff "${BASE_SHA}" HEAD -m "${MAIN_MODULES}" -v
depstat diff "${BASE_SHA}" HEAD -m "${MAIN_MODULES}" --json > diff.json
depstat diff "${BASE_SHA}" HEAD -m "${MAIN_MODULES}" --dot > diff.dot
dot -Tsvg diff.dot -o diff.svg
```

Split output by test-only vs non-test dependency changes (uses `go mod why -m` to classify):

```bash
depstat diff "${BASE_SHA}" HEAD -m "${MAIN_MODULES}" --split-test-only --json > diff-split.json
jq '.split.testOnly, .split.nonTestOnly' diff-split.json
```

Include vendor-level and vendor-file changes:

```bash
depstat diff "${BASE_SHA}" HEAD -m "${MAIN_MODULES}" --vendor --vendor-files --json > diff-vendor.json
jq '{
  versionChanges: (.versionChanges | length),
  split: {
    nonTestVersionChanges: (.split.nonTestOnly.versionChanges | length),
    testOnlyVersionChanges: (.split.testOnly.versionChanges | length)
  },
  vendor: {
    added: (.vendor.added | length),
    removed: (.vendor.removed | length),
    versionChanges: (.vendor.versionChanges | length),
    vendorOnlyRemovals: (.vendor.vendorOnlyRemovals | length),
    filesDeleted: (.vendor.filesDeleted | length)
  }
}' diff-vendor.json
```

Show vendor-only removals (high-value cleanup signal):

```bash
jq -r '.vendor.vendorOnlyRemovals[]? | "\(.path) \(.version)"' diff-vendor.json
```

Show vendored Go files deleted (possible API removals):

```bash
jq -r '.vendor.filesDeleted[]?' diff-vendor.json
```

PR-style usage (matches Prow pattern):

```bash
depstat diff "${PULL_BASE_SHA}" HEAD -m "${MAIN_MODULES}" --split-test-only --vendor --vendor-files --json > diff.json

# Why newly added modules exist
for dep in $(jq -r '.added[]?' diff.json); do
  depstat why "${dep}" -m "${MAIN_MODULES}" || true
done

# Vendor-only removals are removed from vendor, but still in module graph
for dep in $(jq -r '.vendor.vendorOnlyRemovals[]?.path' diff.json); do
  depstat why "${dep}" -m "${MAIN_MODULES}" || true
done

# Quick high-signal summary for reviewers
jq -r '[
  "module added=\(.added|length) removed=\(.removed|length) versionChanges=\(.versionChanges|length)",
  "non-test added=\(.split.nonTestOnly.added|length) removed=\(.split.nonTestOnly.removed|length) versionChanges=\(.split.nonTestOnly.versionChanges|length)",
  "test-only added=\(.split.testOnly.added|length) removed=\(.split.testOnly.removed|length) versionChanges=\(.split.testOnly.versionChanges|length)",
  "vendor added=\(.vendor.added|length) removed=\(.vendor.removed|length) versionChanges=\(.vendor.versionChanges|length)",
  "vendorOnlyRemovals=\(.vendor.vendorOnlyRemovals|length) filesDeleted=\(.vendor.filesDeleted|length)"
] | .[]' diff.json
```

### `archived`

`archived` checks if dependency source repos are archived on GitHub. It needs a token.

```bash
export GITHUB_TOKEN=<token>
depstat archived --dir "${K8S_DIR}" --json > archived.json
```

Or use a token file:

```bash
depstat archived --dir "${K8S_DIR}" --github-token-path /etc/github/token --json > archived.json
```

## CI Mapping (Kubernetes test-infra patterns)

These are the main patterns used in Kubernetes test-infra:

- Periodic stats/graph/cycles generation
- Presubmit/base-vs-head dependency diff with `why` for newly added modules
- Archived dependency verification with `depstat archived --json`

Reference files in the [kubernetes/test-infra](https://github.com/kubernetes/test-infra) repository:

- [`config/jobs/kubernetes/sig-arch/kubernetes-depstat.yaml`](https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes/sig-arch/kubernetes-depstat.yaml)
- [`config/jobs/kubernetes/sig-arch/kubernetes-depstat-periodical.yaml`](https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes/sig-arch/kubernetes-depstat-periodical.yaml)
- [`experiment/dependencies/verify-archived-repos.sh`](https://github.com/kubernetes/test-infra/blob/master/experiment/dependencies/verify-archived-repos.sh)

## Local Shortcuts in This Repo

From this repository (`sigs.k8s.io/depstat`), the Makefile includes:

```bash
make ci-fixture
make ci-kubernetes-smoke K8S_DIR=<your-kubernetes-checkout>
```

`ci-kubernetes-smoke` runs `stats`, `list`, `graph`, `cycles`, `why`, and `diff` against your Kubernetes checkout and saves artifacts.
