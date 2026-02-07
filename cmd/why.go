/*
Copyright 2024 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// WhyPath represents a dependency path from main module to target
type WhyPath struct {
	Path   []string `json:"path"`
	Direct bool     `json:"direct"` // true if this is a direct dependency of a main module
}

// WhyResult holds the result of why analysis
type WhyResult struct {
	Target      string    `json:"target"`
	Found       bool      `json:"found"`
	Paths       []WhyPath `json:"paths"`
	DirectDeps  []string  `json:"directDependents"` // modules that directly depend on target
	MainModules []string  `json:"mainModules"`
	Truncated   bool      `json:"truncated,omitempty"`
	TotalPaths  int       `json:"totalPaths,omitempty"`
}

const (
	whyDefaultTextPaths = 20
	whyDefaultMaxPaths  = 1000
)

var whyMaxPaths int

var whyCmd = &cobra.Command{
	Use:   "why <dependency>",
	Short: "Show why a dependency is included",
	Long: `Show all dependency paths from main module(s) to a specific dependency.

This helps understand why a particular dependency exists in your project
and which modules are pulling it in.

Examples:
  # Find why a dependency is included
  depstat why github.com/google/btree

  # Output as JSON
  depstat why github.com/google/btree --json

  # Output as DOT for visualization
  depstat why github.com/google/btree --dot | dot -Tsvg -o why.svg

  # Output as self-contained SVG
  depstat why github.com/google/btree --svg > why.svg`,
	Args: cobra.ExactArgs(1),
	RunE: runWhy,
}

func runWhy(cmd *cobra.Command, args []string) error {
	target := args[0]

	depGraph := getDepInfo(mainModules)

	// Find all paths to the target
	result := WhyResult{
		Target:      target,
		Found:       false,
		MainModules: depGraph.MainModules,
	}

	// Check if target exists in dependencies
	allDeps := getAllDeps(depGraph.DirectDepList, depGraph.TransDepList)
	for _, dep := range allDeps {
		if dep == target {
			result.Found = true
			break
		}
	}

	if !result.Found {
		if jsonOutput {
			return outputWhyJSON(result)
		}
		fmt.Printf("Dependency %q not found in the dependency graph.\n", target)
		return nil
	}

	// Find all modules that directly depend on target
	for from, tos := range depGraph.Graph {
		for _, to := range tos {
			if to == target {
				result.DirectDeps = append(result.DirectDeps, from)
			}
		}
	}
	sort.Strings(result.DirectDeps)

	// Find all paths from main modules to target.
	var allPaths [][]string
	for _, mainMod := range depGraph.MainModules {
		findAllPaths(mainMod, target, depGraph.Graph, []string{}, make(map[string]bool), &allPaths, whyMaxPaths)
		if whyMaxPaths > 0 && len(allPaths) >= whyMaxPaths {
			result.Truncated = true
			break
		}
	}
	for _, path := range allPaths {
		isDirect := len(path) == 2 && contains(depGraph.MainModules, path[0])
		result.Paths = append(result.Paths, WhyPath{
			Path:   path,
			Direct: isDirect,
		})
	}

	// Sort paths by length (shortest first)
	sort.Slice(result.Paths, func(i, j int) bool {
		if len(result.Paths[i].Path) != len(result.Paths[j].Path) {
			return len(result.Paths[i].Path) < len(result.Paths[j].Path)
		}
		return strings.Join(result.Paths[i].Path, " -> ") < strings.Join(result.Paths[j].Path, " -> ")
	})
	result.TotalPaths = len(result.Paths)

	if jsonOutput {
		return outputWhyJSON(result)
	}
	if dotOutput {
		return outputWhyDOT(result, depGraph)
	}
	if svgOutput {
		return outputWhySVG(result)
	}
	return outputWhyText(result)
}

// findAllPaths finds paths from start to target using DFS and appends to out.
// If maxPaths > 0, search stops once out reaches maxPaths.
func findAllPaths(start, target string, graph map[string][]string, currentPath []string, visited map[string]bool, out *[][]string, maxPaths int) {
	if maxPaths > 0 && len(*out) >= maxPaths {
		return
	}

	currentPath = append(currentPath, start)

	if start == target {
		// Found the target, append a copy of the path.
		pathCopy := make([]string, len(currentPath))
		copy(pathCopy, currentPath)
		*out = append(*out, pathCopy)
		return
	}

	if visited[start] {
		return
	}
	visited[start] = true
	defer func() { visited[start] = false }()

	for _, next := range graph[start] {
		findAllPaths(next, target, graph, currentPath, visited, out, maxPaths)
		if maxPaths > 0 && len(*out) >= maxPaths {
			return
		}
	}
}

func outputWhyJSON(result WhyResult) error {
	out, err := json.MarshalIndent(result, "", "\t")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func outputWhyText(result WhyResult) error {
	fmt.Printf("Why is %s included?\n", result.Target)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	if !result.Found {
		fmt.Println("Not found in dependency graph.")
		return nil
	}

	// Show direct dependents
	fmt.Printf("Directly depended on by (%d modules):\n", len(result.DirectDeps))
	for _, dep := range result.DirectDeps {
		marker := "  "
		if contains(result.MainModules, dep) {
			marker = "* " // Mark main modules
		}
		fmt.Printf("  %s%s\n", marker, dep)
	}
	fmt.Println()

	// Show paths in text mode with a default display cap to keep output readable.
	pathsToShow := result.Paths
	if len(pathsToShow) > whyDefaultTextPaths {
		pathsToShow = pathsToShow[:whyDefaultTextPaths]
	}
	fmt.Printf("Dependency paths (showing %d of %d):\n", len(pathsToShow), len(result.Paths))
	fmt.Println()

	for i, wp := range pathsToShow {
		if wp.Direct {
			fmt.Printf("  %d. [DIRECT] ", i+1)
		} else {
			fmt.Printf("  %d. ", i+1)
		}
		fmt.Println(strings.Join(wp.Path, " -> "))
	}

	if len(result.Paths) > len(pathsToShow) || result.Truncated {
		fmt.Println()
		if result.Truncated {
			fmt.Printf("  (search truncated at --max-paths=%d)\n", whyMaxPaths)
		} else {
			fmt.Printf("  (showing first %d in text output; use --json/--dot/--svg for full set)\n", whyDefaultTextPaths)
		}
	}

	return nil
}

func outputWhyDOT(result WhyResult, depGraph *DependencyOverview) error {
	fmt.Println("strict digraph {")
	fmt.Printf("graph [overlap=false, label=\"Why: %s\", labelloc=t];\n", result.Target)
	fmt.Println("node [shape=box, style=filled, fillcolor=white];")
	fmt.Println()

	// Collect all nodes and edges from paths
	nodes := make(map[string]bool)
	edges := make(map[string]bool)

	for _, wp := range result.Paths {
		for i, node := range wp.Path {
			nodes[node] = true
			if i > 0 {
				edge := fmt.Sprintf("%s -> %s", wp.Path[i-1], node)
				edges[edge] = true
			}
		}
	}

	// Output nodes with colors
	fmt.Println("// Nodes")
	nodeList := make([]string, 0, len(nodes))
	for node := range nodes {
		nodeList = append(nodeList, node)
	}
	sort.Strings(nodeList)
	for _, node := range nodeList {
		color := "white"
		if node == result.Target {
			color = "#ffffcc" // yellow for target
		} else if contains(result.MainModules, node) {
			color = "#ccffcc" // green for main modules
		}
		fmt.Printf("\"%s\" [fillcolor=\"%s\"];\n", node, color)
	}
	fmt.Println()

	// Output edges
	fmt.Println("// Edges")
	edgeList := make([]string, 0, len(edges))
	for edge := range edges {
		edgeList = append(edgeList, edge)
	}
	sort.Strings(edgeList)
	for _, edge := range edgeList {
		parts := strings.Split(edge, " -> ")
		if len(parts) == 2 {
			fmt.Printf("\"%s\" -> \"%s\";\n", parts[0], parts[1])
		}
	}

	fmt.Println("}")
	return nil
}

func init() {
	rootCmd.AddCommand(whyCmd)
	whyCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate")
	whyCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	whyCmd.Flags().BoolVarP(&dotOutput, "dot", "", false, "Output in DOT format for Graphviz")
	whyCmd.Flags().BoolVarP(&svgOutput, "svg", "s", false, "Output as self-contained SVG diagram")
	whyCmd.Flags().IntVar(&whyMaxPaths, "max-paths", whyDefaultMaxPaths, "Maximum dependency paths to search. Set 0 for no limit")
	whyCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Specify main modules")
}
