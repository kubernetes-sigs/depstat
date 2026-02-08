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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var dep string
var showEdgeTypes bool
var graphDotOutput bool
var graphJSONOutput bool
var graphOutputPath string
var graphTopMode string
var graphTopN int

type graphNode struct {
	Module       string `json:"module"`
	InDegree     int    `json:"inDegree"`
	OutDegree    int    `json:"outDegree"`
	Depth        int    `json:"depth"` // -1 means unreachable from any main module
	IsMainModule bool   `json:"isMainModule"`
}

type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type graphRankings struct {
	Mode string      `json:"mode"`
	N    int         `json:"n"`
	In   []graphNode `json:"in,omitempty"`
	Out  []graphNode `json:"out,omitempty"`
}

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Generate a .dot file to be used with Graphviz's dot command.",
	Long: `A graph.dot file will be generated which can be used with Graphviz's dot command.
	For example to generate a svg image use:
	twopi -Tsvg -o dag.svg graph.dot

	Use --show-edge-types to distinguish between direct and transitive dependencies:
	- Direct edges (solid blue): from main module(s) to their direct dependencies
	- Transitive edges (dashed gray): dependencies of dependencies`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if graphDotOutput && graphJSONOutput {
			return fmt.Errorf("--dot and --json are mutually exclusive")
		}
		if graphTopMode != "" && graphDotOutput {
			return fmt.Errorf("cannot use --top with --dot")
		}
		if graphTopMode != "" && graphTopMode != "in" && graphTopMode != "out" && graphTopMode != "both" {
			return fmt.Errorf("--top must be one of: in, out, both")
		}
		if graphTopMode != "" && graphTopN <= 0 {
			return fmt.Errorf("-n must be > 0")
		}
		if graphTopMode != "" && graphTopN <= 0 {
			return fmt.Errorf("-n must be > 0")
		}
		overview := getDepInfo(mainModules)
		if len(overview.MainModules) == 0 {
			return fmt.Errorf("could not determine main module; run from a Go module directory or set --mainModules")
		}
		nodes, edgeObjects := buildGraphTopology(overview)

		if graphTopMode != "" && !graphJSONOutput && !graphDotOutput {
			printTopNodes(nodes, graphTopMode, graphTopN)
			return nil
		}
		// strict ensures that there is only one edge between two vertices
		// overlap = false ensures the vertices don't overlap
		fileContents := "strict digraph {\ngraph [overlap=false];\n"

		// graph to be generated is based around input dep
		if dep != "" {
			var chains []Chain
			var temp Chain
			getAllChains(overview.MainModules[0], overview.Graph, temp, &chains)
			fileContents += getFileContentsForSingleDep(chains, dep)
		} else {
			fileContents += getFileContentsForAllDepsWithTypes(overview, showEdgeTypes)
		}
		fileContents += "}"
		if graphJSONOutput {
			edges := getEdges(overview.Graph)
			var rankings *graphRankings
			if graphTopMode != "" {
				rankings = buildRankings(nodes, graphTopMode, graphTopN)
			}
			outputObj := struct {
				MainModules         []string            `json:"mainModules"`
				DirectDependencies  []string            `json:"directDependencies"`
				TransDependencies   []string            `json:"transitiveDependencies"`
				Graph               map[string][]string `json:"graph"`
				Edges               []string            `json:"edges"`
				Nodes               []graphNode         `json:"nodes"`
				EdgeObjects         []graphEdge         `json:"edgeObjects"`
				Rankings            *graphRankings      `json:"rankings,omitempty"`
				FocusedDependency   string              `json:"focusedDependency,omitempty"`
				ShowEdgeTypes       bool                `json:"showEdgeTypes"`
				DirectCount         int                 `json:"directDependencyCount"`
				TransitiveCount     int                 `json:"transitiveDependencyCount"`
				TotalDependencyEdge int                 `json:"edgeCount"`
			}{
				MainModules:         overview.MainModules,
				DirectDependencies:  overview.DirectDepList,
				TransDependencies:   overview.TransDepList,
				Graph:               overview.Graph,
				Edges:               edges,
				Nodes:               nodes,
				EdgeObjects:         edgeObjects,
				Rankings:            rankings,
				FocusedDependency:   dep,
				ShowEdgeTypes:       showEdgeTypes,
				DirectCount:         len(overview.DirectDepList),
				TransitiveCount:     len(overview.TransDepList),
				TotalDependencyEdge: len(edges),
			}
			out, err := json.MarshalIndent(outputObj, "", "\t")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		}
		if graphDotOutput {
			fmt.Print(fileContents)
			return nil
		}

		fileContentsByte := []byte(fileContents)
		err := os.WriteFile(graphOutputPath, fileContentsByte, 0644)
		if err != nil {
			return err
		}
		fmt.Printf("\nCreated %s file!\n", graphOutputPath)
		return nil
	},
}

// find all possible chains starting from currentDep
func getAllChains(currentDep string, graph map[string][]string, currentChain Chain, chains *[]Chain) {
	currentChain = append(currentChain, currentDep)
	_, ok := graph[currentDep]
	if ok {
		for _, dep := range graph[currentDep] {
			if !contains(currentChain, dep) {
				cpy := make(Chain, len(currentChain))
				copy(cpy, currentChain)
				getAllChains(dep, graph, cpy, chains)
			} else {
				*chains = append(*chains, currentChain)
			}
		}
	} else {
		*chains = append(*chains, currentChain)
	}
}

// get the contents of the .dot file for the graph
// when the --dep flag is set
func getFileContentsForSingleDep(chains []Chain, dep string) string {
	// to color the entered node as yellow
	data := colorMainNode(dep)

	// add all chains which have the input dep to the .dot file
	for _, chain := range chains {
		if chainContains(chain, dep) {
			for i := range chain {
				if chain[i] == dep {
					chain[i] = "MainNode"
				} else {
					chain[i] = "\"" + chain[i] + "\""
				}
			}
			data += strings.Join(chain, " -> ")
			data += "\n"
		}
	}
	return data
}

// get the contents of the .dot file for the graph
// of all dependencies (when --dep is not set)
func getFileContentsForAllDeps(overview *DependencyOverview) string {
	return getFileContentsForAllDepsWithTypes(overview, false)
}

// getFileContentsForAllDepsWithTypes generates DOT content with optional edge type annotations
func getFileContentsForAllDepsWithTypes(overview *DependencyOverview, showTypes bool) string {
	if len(overview.MainModules) == 0 {
		return ""
	}
	// color the main module as yellow
	data := colorMainNode(overview.MainModules[0])

	// Create a set of main modules for quick lookup
	mainModSet := make(map[string]bool)
	for _, m := range overview.MainModules {
		mainModSet[m] = true
	}

	// Create a set of direct dependencies for quick lookup
	directDepSet := make(map[string]bool)
	for _, d := range overview.DirectDepList {
		directDepSet[d] = true
	}

	allDeps := getAllDeps(overview.DirectDepList, overview.TransDepList)
	allDeps = append(allDeps, overview.MainModules[0])
	sort.Strings(allDeps)

	for _, dep := range allDeps {
		_, ok := overview.Graph[dep]
		if !ok {
			continue
		}
		// main module can never be a neighbour
		for _, neighbour := range overview.Graph[dep] {
			var edgeAttrs string
			if showTypes {
				if mainModSet[dep] {
					// Edge from main module = direct dependency
					edgeAttrs = " [color=\"blue\", style=\"bold\", edgetype=\"direct\"]"
				} else {
					// Edge from non-main module = transitive dependency
					edgeAttrs = " [color=\"gray\", style=\"dashed\", edgetype=\"transitive\"]"
				}
			}

			if mainModSet[dep] {
				// for the main module use a colored node
				data += fmt.Sprintf("\"MainNode\" -> \"%s\"%s\n", neighbour, edgeAttrs)
			} else {
				data += fmt.Sprintf("\"%s\" -> \"%s\"%s\n", dep, neighbour, edgeAttrs)
			}
		}
	}
	return data
}

func chainContains(chain Chain, dep string) bool {
	for _, d := range chain {
		if d == dep {
			return true
		}
	}
	return false
}

func colorMainNode(mainNode string) string {
	return fmt.Sprintf("MainNode [label=\"%s\", style=\"filled\" color=\"yellow\"]\n", mainNode)
}

func buildGraphTopology(overview *DependencyOverview) ([]graphNode, []graphEdge) {
	nodeSet := map[string]bool{}
	inDegree := map[string]int{}
	outDegree := map[string]int{}
	mainSet := map[string]bool{}
	for _, m := range overview.MainModules {
		mainSet[m] = true
		nodeSet[m] = true
	}

	var edges []graphEdge
	for from, tos := range overview.Graph {
		nodeSet[from] = true
		outDegree[from] += len(tos)
		for _, to := range tos {
			nodeSet[to] = true
			inDegree[to]++
			edges = append(edges, graphEdge{From: from, To: to})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	depth := shortestDepthByModule(overview.MainModules, overview.Graph)
	nodes := make([]graphNode, 0, len(nodeSet))
	for module := range nodeSet {
		moduleDepth := -1 // unreachable from any main module
		if d, ok := depth[module]; ok {
			moduleDepth = d
		}
		nodes = append(nodes, graphNode{
			Module:       module,
			InDegree:     inDegree[module],
			OutDegree:    outDegree[module],
			Depth:        moduleDepth,
			IsMainModule: mainSet[module],
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Module < nodes[j].Module })
	return nodes, edges
}

func shortestDepthByModule(mainModules []string, graph map[string][]string) map[string]int {
	depth := map[string]int{}
	queue := make([]string, 0, len(mainModules))
	for _, m := range mainModules {
		if _, seen := depth[m]; seen {
			continue
		}
		depth[m] = 0
		queue = append(queue, m)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		nextDepth := depth[current] + 1
		for _, next := range graph[current] {
			if _, seen := depth[next]; seen {
				continue
			}
			depth[next] = nextDepth
			queue = append(queue, next)
		}
	}
	return depth
}

func buildRankings(nodes []graphNode, mode string, n int) *graphRankings {
	r := &graphRankings{Mode: mode, N: n}
	if mode == "in" || mode == "both" {
		r.In = topNByMetric(nodes, n, "in")
	}
	if mode == "out" || mode == "both" {
		r.Out = topNByMetric(nodes, n, "out")
	}
	return r
}

func topNByMetric(nodes []graphNode, n int, metric string) []graphNode {
	ranked := make([]graphNode, len(nodes))
	copy(ranked, nodes)
	sort.Slice(ranked, func(i, j int) bool {
		var left, right int
		if metric == "in" {
			left, right = ranked[i].InDegree, ranked[j].InDegree
		} else {
			left, right = ranked[i].OutDegree, ranked[j].OutDegree
		}
		if left == right {
			return ranked[i].Module < ranked[j].Module
		}
		return left > right
	})
	if n > len(ranked) {
		n = len(ranked)
	}
	return ranked[:n]
}

func printTopNodes(nodes []graphNode, mode string, n int) {
	switch mode {
	case "in":
		printTopByMetric(nodes, n, "in")
	case "out":
		printTopByMetric(nodes, n, "out")
	case "both":
		printTopByMetric(nodes, n, "in")
		fmt.Println()
		printTopByMetric(nodes, n, "out")
	}
}

func printTopByMetric(nodes []graphNode, n int, metric string) {
	ranked := make([]graphNode, len(nodes))
	copy(ranked, nodes)
	sort.Slice(ranked, func(i, j int) bool {
		var left, right int
		if metric == "in" {
			left, right = ranked[i].InDegree, ranked[j].InDegree
		} else {
			left, right = ranked[i].OutDegree, ranked[j].OutDegree
		}
		if left == right {
			return ranked[i].Module < ranked[j].Module
		}
		return left > right
	})
	if n > len(ranked) {
		n = len(ranked)
	}

	title := "Top by in-degree"
	if metric == "out" {
		title = "Top by out-degree"
	}
	fmt.Printf("%s (N=%d)\n", title, n)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RANK\tMODULE\tIN\tOUT\tDEPTH\tMAIN")
	for i := 0; i < n; i++ {
		node := ranked[i]
		fmt.Fprintf(w, "%d\t%s\t%d\t%d\t%d\t%t\n", i+1, node.Module, node.InDegree, node.OutDegree, node.Depth, node.IsMainModule)
	}
	_ = w.Flush()
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate. Defaults to the current directory.")
	graphCmd.Flags().StringVarP(&dep, "dep", "p", "", "Specify dependency to create a graph around")
	graphCmd.Flags().BoolVar(&showEdgeTypes, "show-edge-types", false, "Distinguish direct vs transitive edges with colors/styles")
	graphCmd.Flags().BoolVar(&graphDotOutput, "dot", false, "Output DOT graph to stdout")
	graphCmd.Flags().BoolVarP(&graphJSONOutput, "json", "j", false, "Output graph data in JSON format")
	graphCmd.Flags().StringVar(&graphTopMode, "top", "", "Show top modules by degree: in, out, or both")
	graphCmd.Flags().IntVarP(&graphTopN, "n", "n", 10, "Number of modules to show with --top")
	graphCmd.Flags().StringSliceVar(&excludeModules, "exclude-modules", []string{}, "Exclude module path patterns (repeatable, supports * wildcard)")
	graphCmd.Flags().StringVar(&graphOutputPath, "output", "graph.dot", "Path to DOT output file when not using --dot or --json")
	graphCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Specify main modules")
}
