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
	// List of all transitive dependencies
	TransDepList []string
	// Name of the module from which the dependencies are computed
	MainModuleName string
}

func getDepInfo(mainModules []string) *DependencyOverview {
	// get output of "go mod graph" in a string
	goModGraph := exec.Command("go", "mod", "graph")
	goModGraphOutput, err := goModGraph.Output()
	if err != nil {
		log.Fatal(err)
	}
	goModGraphOutputString := string(goModGraphOutput)

	// create a graph of dependencies from that output
	depGraph := make(map[string][]string)
	scanner := bufio.NewScanner(strings.NewReader(goModGraphOutputString))

	// transDeps will store all the transitive dependencies
	var transDeps []string
	mainModule := "notset"

	for scanner.Scan() {
		line := scanner.Text()
		words := strings.Fields(line)
		// remove versions
		words[0] = (strings.Split(words[0], "@"))[0]
		words[1] = (strings.Split(words[1], "@"))[0]

		// we don't want to add the same dep again
		if !contains(depGraph[words[0]], words[1]) {
			depGraph[words[0]] = append(depGraph[words[0]], words[1])
		}

		if mainModule == "notset" {
			mainModule = words[0]
		}

		// anything where the LHS is not mainModule
		// is a transitive dependency

		if len(mainModules) == 0 {
			if words[0] != mainModule {
				if !contains(transDeps, words[1]) {
					transDeps = append(transDeps, words[1])
				}
			}
		} else {
			// if the user has specified a list of modules to be used
			if !contains(mainModules, words[0]) {
				if !contains(mainModules, words[1]) && !contains(transDeps, words[1]) {
					transDeps = append(transDeps, words[1])
				}
			}
		}

	}
	return &DependencyOverview{
		Graph:          depGraph,
		TransDepList:   transDeps,
		MainModuleName: mainModule,
	}
}

func printDeps(deps []string) {
	fmt.Println()
	sort.Strings(deps)
	for _, dep := range deps {
		fmt.Println(dep)
	}
	fmt.Println()
}

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
