package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func max(x, y int) int {
	if x <= y {
		return y
	}
	return x
}

// find all possible chains starting from currentDep
func getChains(currentDep string, graph map[string][]string, longestPath []string, chains map[int][]string) {
	longestPath = append(longestPath, currentDep)
	_, ok := graph[currentDep]
	if ok {
		for _, dep := range graph[currentDep] {
			if !contains(longestPath, dep) {
				cpy := make([]string, len(longestPath))
				copy(cpy, longestPath)
				getChains(dep, graph, cpy, chains)
			} else {
				chains[len(longestPath)] = longestPath
			}
		}
	} else {
		chains[len(longestPath)] = longestPath
	}
}

// get the length of the longest dependency chain
func getMaxDepth(chains map[int][]string) int {
	maxDeps := 0
	for deps := range chains {
		maxDeps = max(maxDeps, deps)
	}
	// for A -> B -> C the depth is 2
	return maxDeps - 1
}

func printChain(deps []string) {
	fmt.Println()
	for ind, dep := range deps {
		if ind == len(deps)-1 {
			fmt.Print(dep)
		} else {
			fmt.Print(dep + " -> ")
		}
	}
	fmt.Println()
}

func getDepInfo() (map[string][]string, []string, string) {

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

	// deps will store all the dependencies
	var deps []string
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
			// we don't want to add mainModule to deps list
			continue
		}

		if !contains(deps, words[0]) {
			deps = append(deps, words[0])
		}
		if !contains(deps, words[1]) {
			deps = append(deps, words[1])
		}

	}
	return depGraph, deps, mainModule
}

func printDeps(deps []string) {
	fmt.Println()
	for _, dep := range deps {
		fmt.Println(dep)
	}
	fmt.Println()
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
