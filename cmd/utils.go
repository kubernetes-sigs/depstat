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

func getDepInfo(mainModules []string) *DependencyOverview {
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

func generateGraph(goModGraphOutputString string, mainModules []string) DependencyOverview {
	depGraph := DependencyOverview{MainModules: mainModules}
	graph := make(map[string][]string)
	scanner := bufio.NewScanner(strings.NewReader(goModGraphOutputString))

	for scanner.Scan() {
		line := scanner.Text()
		words := strings.Fields(line)
		// remove versions
		words[0] = (strings.Split(words[0], "@"))[0]
		words[1] = (strings.Split(words[1], "@"))[0]

		// we don't want to add the same dep again
		if !contains(graph[words[0]], words[1]) {
			graph[words[0]] = append(graph[words[0]], words[1])
		}

		if len(depGraph.MainModules) == 0 {
			depGraph.MainModules = append(depGraph.MainModules, words[0])
		}

		// if the LHS is a mainModule
		// then RHS is a direct dep else transitive dep
		if contains(depGraph.MainModules, words[0]) && contains(depGraph.MainModules, words[1]) {
			continue
		} else if contains(depGraph.MainModules, words[0]) {
			if !contains(depGraph.DirectDepList, words[1]) {
				depGraph.DirectDepList = append(depGraph.DirectDepList, words[1])
			}
		} else if !contains(depGraph.MainModules, words[0]) {
			if !contains(depGraph.TransDepList, words[1]) {
				depGraph.TransDepList = append(depGraph.TransDepList, words[1])
			}
		}
	}

	depGraph.Graph = graph

	return depGraph
}
