/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"
)

func printChain(slice []string) {
	fmt.Println()
	fmt.Println(strings.Join(slice, " -> "))
}

// DependencyOverview holds dependency module informations
type DependencyOverview struct {
	// Dependency graph edges modelled as node plus adjacency nodes
	Graph map[string][]string
	// List of all direct dependencies
	DirectDepList []string
	// List of all transitive dependencies
	TransDepList []string
	// Name of the module from which the dependencies are computed
	MainModules []string
}

// getMainModule returns the main module name using "go list -m"
func getMainModule() string {
	goListM := exec.Command("go", "list", "-m")
	if dir != "" {
		goListM.Dir = dir
	}
	output, err := goListM.Output()
	if err != nil {
		return ""
	}
	// In workspaces, "go list -m" returns multiple modules, one per line.
	// The first line is the main module of the current directory.
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func getDepInfo(mainModules []string) *DependencyOverview {
	// If no main modules specified, detect using "go list -m"
	if len(mainModules) == 0 {
		if mainMod := getMainModule(); mainMod != "" {
			mainModules = []string{mainMod}
		}
	}

	// get output of "go mod graph" in a string
	goModGraph := exec.Command("go", "mod", "graph")
	if dir != "" {
		goModGraph.Dir = dir
	}
	goModGraphOutput, err := goModGraph.Output()
	if err != nil {
		log.Fatal(err)
	}
	goModGraphOutputString := string(goModGraphOutput)

	// create a graph of dependencies from that output
	depGraph := generateGraph(goModGraphOutputString, mainModules)
	return &depGraph
}

func printDeps(deps []string) {
	fmt.Println()
	sort.Strings(deps)
	for _, dep := range deps {
		fmt.Println(dep)
	}
	fmt.Println()
}

// we need this since a dependency can be both a direct and an indirect dependency
func getAllDeps(directDeps []string, transDeps []string) []string {
	var allDeps []string
	for _, dep := range directDeps {
		if !contains(allDeps, dep) {
			allDeps = append(allDeps, dep)
		}
	}
	for _, dep := range transDeps {
		if !contains(allDeps, dep) {
			allDeps = append(allDeps, dep)
		}
	}
	return allDeps
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// compares two slices of strings
func isSliceSame(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for iterator := 0; iterator < len(a); iterator++ {
		if a[iterator] != b[iterator] {
			return false
		}
	}
	return true
}

func sliceContains(val []Chain, key Chain) bool {
	for _, v := range val {
		if isSliceSame(v, key) {
			return true
		}
	}
	return false
}

type module struct {
	name    string
	version string
}

func parseModule(s string) module {
	if strings.Contains(s, "@") {
		parts := strings.SplitN(s, "@", 2)
		return module{name: parts[0], version: parts[1]}
	}
	return module{name: s}
}

func generateGraph(goModGraphOutputString string, mainModules []string) DependencyOverview {
	depGraph := DependencyOverview{MainModules: mainModules}
	versionedGraph := make(map[module][]module)
	var lhss []module
	graph := make(map[string][]string)
	scanner := bufio.NewScanner(strings.NewReader(goModGraphOutputString))

	var versionedMainModules []module
	var seenVersionedMainModules = map[module]bool{}
	for scanner.Scan() {
		line := scanner.Text()
		words := strings.Fields(line)

		lhs := parseModule(words[0])
		// Skip go toolchain lines (e.g., "go@1.21.0 toolchain@go1.21.0")
		// These are not real modules and should not be treated as main modules
		if lhs.name == "go" || strings.HasPrefix(lhs.name, "toolchain") {
			continue
		}
		if len(versionedMainModules) == 0 || contains(mainModules, lhs.name) {
			if !seenVersionedMainModules[lhs] {
				// remember our root module and listed main modules
				versionedMainModules = append(versionedMainModules, lhs)
				seenVersionedMainModules[lhs] = true
			}
		}
		if len(depGraph.MainModules) == 0 {
			// record the first module we see as the main module by default
			depGraph.MainModules = append(depGraph.MainModules, lhs.name)
		}
		rhs := parseModule(words[1])

		// remember the order we observed lhs modules in
		if len(versionedGraph[lhs]) == 0 {
			lhss = append(lhss, lhs)
		}
		// record this lhs -> rhs relationship
		versionedGraph[lhs] = append(versionedGraph[lhs], rhs)
	}

	// record effective versions of modules required by our main modules
	// in go1.17+, the main module records effective versions of all dependencies, even indirect ones
	effectiveVersions := map[string]string{}
	for _, mm := range versionedMainModules {
		for _, m := range versionedGraph[mm] {
			if effectiveVersions[m.name] < m.version {
				effectiveVersions[m.name] = m.version
			}
		}
	}

	type edge struct {
		from module
		to   module
	}

	// figure out which modules in the graph are reachable from the effective versions required by our main modules
	reachableModules := map[string]module{}
	// start with our main modules
	var toVisit []edge
	for _, m := range versionedMainModules {
		toVisit = append(toVisit, edge{to: m})
	}
	for len(toVisit) > 0 {
		from := toVisit[0].from
		v := toVisit[0].to
		toVisit = toVisit[1:]
		if _, reachable := reachableModules[v.name]; reachable {
			// already flagged as reachable
			continue
		}
		// mark as reachable
		reachableModules[v.name] = from
		if effectiveVersion, ok := effectiveVersions[v.name]; ok && effectiveVersion > v.version {
			// replace with the effective version if applicable
			v.version = effectiveVersion
		} else {
			// set the effective version
			effectiveVersions[v.name] = v.version
		}
		// queue dependants of this to check for reachability
		for _, m := range versionedGraph[v] {
			toVisit = append(toVisit, edge{from: v, to: m})
		}
	}

	for _, lhs := range lhss {
		if _, reachable := reachableModules[lhs.name]; !reachable {
			// this is not reachable via required versions, skip it
			continue
		}
		if effectiveVersion, ok := effectiveVersions[lhs.name]; ok && effectiveVersion != lhs.version {
			// this is not the effective version in our graph, skip it
			continue
		}
		// fmt.Println(lhs.name, "via", reachableModules[lhs.name])

		for _, rhs := range versionedGraph[lhs] {
			// we don't want to add the same dep again
			if !contains(graph[lhs.name], rhs.name) {
				graph[lhs.name] = append(graph[lhs.name], rhs.name)
			}

			// if the LHS is a mainModule
			// then RHS is a direct dep else transitive dep
			if contains(depGraph.MainModules, lhs.name) && contains(depGraph.MainModules, rhs.name) {
				continue
			} else if contains(depGraph.MainModules, lhs.name) {
				if !contains(depGraph.DirectDepList, rhs.name) {
					// fmt.Println(rhs.name, "via", lhs)
					depGraph.DirectDepList = append(depGraph.DirectDepList, rhs.name)
				}
			} else if !contains(depGraph.MainModules, lhs.name) {
				if !contains(depGraph.TransDepList, rhs.name) {
					// fmt.Println(rhs.name, "via", lhs)
					depGraph.TransDepList = append(depGraph.TransDepList, rhs.name)
				}
			}
		}
	}

	depGraph.Graph = graph

	return depGraph
}
