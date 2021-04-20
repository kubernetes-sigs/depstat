#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

declare -a repos=("utils" "metrics" "test-infra" "kube-state-metrics" "ingress-gce" "node-problem-detector")

rm -rf "/Users/arsh/depstat-temp"

for repo in "${repos[@]}"
do
repoUrl="https://github.com/kubernetes/${repo}.git"

localFolder="/Users/arsh/depstat-temp"

echo "${repoUrl}"
git clone "$repoUrl" "$localFolder"

cd "$localFolder"
pwd
depstat stats

cd ..

rm -rf "$localFolder"
done

