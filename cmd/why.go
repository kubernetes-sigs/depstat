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
	// Pre-computed graph data for SVG/DOT output (avoids expensive path enumeration)
	NodeSet map[string]bool  `json:"-"`
	EdgeSet map[svgEdge]bool `json:"-"`
}

const (
	whyDefaultTextPaths = 20
	whyDefaultMaxPaths  = 1000
)

var whyMaxPaths int
var whyMaxDepth int
var whySplitTestOnly bool

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
	if len(depGraph.MainModules) == 0 {
		return fmt.Errorf("no main modules remain after exclusions; adjust --exclude-modules or --mainModules")
	}

	// Find all paths to the target
	result := WhyResult{
		Target:      target,
		Found:       false,
		MainModules: depGraph.MainModules,
	}
	if whySplitTestOnly {
		allDeps := getAllDeps(depGraph.DirectDepList, depGraph.TransDepList)
		testOnlySet, err := classifyTestDeps(allDeps)
		if err != nil {
			return fmt.Errorf("failed to classify dependencies: %w", err)
		}
		if testOnlySet[target] {
			result.Found = true
			result.Paths = []WhyPath{}
			if jsonOutput {
				return outputWhyJSON(result)
			}
			fmt.Printf("Dependency %q is test-only. No non-test paths available.\n", target)
			return nil
		}
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

	// Pre-compute which nodes can reach the target to prune DFS.
	reachable := computeReachableToTarget(target, depGraph.Graph)
	anyMainReachable := false
	for _, mainMod := range depGraph.MainModules {
		if reachable[mainMod] {
			anyMainReachable = true
			break
		}
	}
	if !anyMainReachable {
		if jsonOutput {
			return outputWhyJSON(result)
		}
		fmt.Printf("Dependency %q not reachable from any main module.\n", target)
		return nil
	}

	// For SVG/DOT output, compute the path subgraph directly in O(V+E)
	// instead of enumerating individual paths (which can be exponentially slow).
	if svgOutput || dotOutput {
		nodeSet, edgeSet := computePathSubgraph(depGraph.MainModules, depGraph.Graph, reachable)
		if len(nodeSet) == 0 {
			if svgOutput {
				return outputWhySVG(result)
			}
			fmt.Printf("Dependency %q found in graph, but no paths were discovered.\n", target)
			return nil
		}
		result.NodeSet = nodeSet
		result.EdgeSet = edgeSet
		result.TotalPaths = len(edgeSet) // edge count as proxy for header
		fmt.Fprintf(cmd.ErrOrStderr(), "[depstat why] subgraph nodes=%d edges=%d\n", len(nodeSet), len(edgeSet))

		if dotOutput {
			return outputWhyDOT(result, depGraph)
		}
		return outputWhySVG(result)
	}

	// For text/JSON output, enumerate individual paths using DFS.
	var allPaths [][]string
	if len(depGraph.MainModules) == 0 {
		return fmt.Errorf("no main modules available to search")
	}
	searchMax := whyMaxPaths
	for _, mainMod := range depGraph.MainModules {
		if !reachable[mainMod] {
			continue
		}
		prevCount := len(allPaths)
		findAllPaths(mainMod, target, depGraph.Graph, reachable, []string{}, make(map[string]bool), &allPaths, searchMax)
		added := len(allPaths) - prevCount
		if searchMax > 0 {
			searchMax -= added
		}
		if searchMax > 0 && len(allPaths) >= searchMax {
			result.Truncated = true
			break
		}
		if searchMax <= 0 && whyMaxPaths > 0 {
			result.Truncated = true
			break
		}
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "[depstat why] paths=%d truncated=%v\n", len(allPaths), result.Truncated)
	if len(allPaths) == 0 {
		if jsonOutput {
			return outputWhyJSON(result)
		}
		fmt.Printf("Dependency %q found in graph, but no paths were discovered.\n", target)
		fmt.Printf("Try increasing --max-paths or checking module exclusions.\n")
		return nil
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
	return outputWhyText(result)
}

// computeReachableToTarget does a reverse BFS from target to find all nodes
// that can reach it, allowing DFS to prune dead-end branches early.
func computeReachableToTarget(target string, graph map[string][]string) map[string]bool {
	// Build reverse adjacency list
	reverse := make(map[string][]string)
	for from, tos := range graph {
		for _, to := range tos {
			reverse[to] = append(reverse[to], from)
		}
	}
	// BFS backward from target
	reachable := map[string]bool{target: true}
	queue := []string{target}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, prev := range reverse[cur] {
			if !reachable[prev] {
				reachable[prev] = true
				queue = append(queue, prev)
			}
		}
	}
	return reachable
}

// computePathSubgraph computes the set of nodes and edges that lie on any path
// from a main module to the target, using bidirectional reachability in O(V+E).
// The 'reachable' map (backward BFS from target) must already be computed.
func computePathSubgraph(mainModules []string, graph map[string][]string, reachable map[string]bool) (map[string]bool, map[svgEdge]bool) {
	// Forward BFS from main modules, constrained to nodes that can reach the target.
	forward := make(map[string]bool)
	queue := make([]string, 0)
	for _, m := range mainModules {
		if reachable[m] {
			forward[m] = true
			queue = append(queue, m)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range graph[cur] {
			if reachable[next] && !forward[next] {
				forward[next] = true
				queue = append(queue, next)
			}
		}
	}

	// Collect edges between path nodes while removing back-edges from cycles.
	nodeSet := forward // forward ⊆ reachable by construction
	adj := make(map[string][]string)
	nodes := make([]string, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, from := range nodes {
		for _, to := range graph[from] {
			if nodeSet[to] {
				adj[from] = append(adj[from], to)
			}
		}
		sort.Strings(adj[from])
	}

	const (
		visitUnseen = 0
		visitActive = 1
		visitDone   = 2
	)
	state := make(map[string]int)
	edgeSet := make(map[svgEdge]bool)

	var dfs func(string)
	dfs = func(cur string) {
		state[cur] = visitActive
		for _, next := range adj[cur] {
			switch state[next] {
			case visitUnseen:
				edgeSet[svgEdge{From: cur, To: next}] = true
				dfs(next)
			case visitActive:
				// Skip back-edge to avoid cycles in the rendered graph.
			default:
				edgeSet[svgEdge{From: cur, To: next}] = true
			}
		}
		state[cur] = visitDone
	}

	for _, node := range nodes {
		if state[node] == visitUnseen {
			dfs(node)
		}
	}

	return nodeSet, edgeSet
}

// findAllPaths finds paths from start to target using DFS and appends to out.
// If maxPaths > 0, search stops once out reaches maxPaths.
// If whyMaxDepth > 0, paths longer than whyMaxDepth hops are pruned.
func findAllPaths(start, target string, graph map[string][]string, reachable map[string]bool, currentPath []string, visited map[string]bool, out *[][]string, maxPaths int) {
	if start == target {
		pathCopy := make([]string, len(currentPath)+1)
		copy(pathCopy, append(currentPath, start))
		*out = append(*out, pathCopy)
		return
	}
	if maxPaths > 0 && len(*out) >= maxPaths {
		return
	}
	if !reachable[start] {
		return
	}
	if whyMaxDepth > 0 && len(currentPath) >= whyMaxDepth {
		return
	}

	currentPath = append(currentPath, start)

	if visited[start] {
		return
	}
	visited[start] = true
	defer func() { visited[start] = false }()

	for _, next := range graph[start] {
		if !reachable[next] {
			continue
		}
		findAllPaths(next, target, graph, reachable, currentPath, visited, out, maxPaths)
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

	// Use pre-computed subgraph if available, otherwise extract from paths.
	nodes := make(map[string]bool)
	edges := make(map[string]bool)

	if result.NodeSet != nil {
		for n := range result.NodeSet {
			nodes[n] = true
		}
		for e := range result.EdgeSet {
			edges[fmt.Sprintf("%s -> %s", e.From, e.To)] = true
		}
	} else {
		for _, wp := range result.Paths {
			for i, node := range wp.Path {
				nodes[node] = true
				if i > 0 {
					edge := fmt.Sprintf("%s -> %s", wp.Path[i-1], node)
					edges[edge] = true
				}
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
	whyCmd.Flags().IntVar(&whyMaxDepth, "max-depth", 0, "Maximum path depth in hops (0 = unlimited). Useful for limiting DFS on deep graphs")
	whyCmd.Flags().BoolVar(&whySplitTestOnly, "split-test-only", false, "Exclude test-only dependencies when finding paths (uses go mod why -m)")
	whyCmd.Flags().StringSliceVar(&excludeModules, "exclude-modules", []string{}, "Exclude module path patterns (repeatable, supports * wildcard)")
	whyCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Specify main modules")
}
