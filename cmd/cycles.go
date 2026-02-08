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
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

var jsonOutputCycles bool
var summaryOutputCycles bool
var maxCycleLength int
var cyclesTopN int

// cyclesFinder implements Johnson's algorithm for finding all elementary cycles
// in a directed graph. Time complexity: O((V+E)(C+1)) where C is the number of cycles.
type cyclesFinder struct {
	graph      map[string][]string
	nodeIndex  map[string]int
	indexNode  []string
	blocked    []bool
	blockedMap []map[int]bool
	stack      []int
	cycles     []Chain
	maxLength  int
}

type cycleSummary struct {
	TotalCycles     int                `json:"totalCycles"`
	ByLength        map[string]int     `json:"byLength"`
	TwoNodeCycles   [][]string         `json:"twoNodeCycles"`
	TopParticipants []cycleParticipant `json:"topParticipants"`
}

type cycleParticipant struct {
	Module     string `json:"module"`
	CycleCount int    `json:"cycleCount"`
}

// analyzeDepsCmd represents the analyzeDeps command
var cyclesCmd = &cobra.Command{
	Use:   "cycles",
	Short: "Prints cycles in dependency chains.",
	Long:  `Will show all the cycles in the dependencies of the project.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			return fmt.Errorf("cycles does not take any arguments")
		}

		overview := getDepInfo(mainModules)
		if maxCycleLength != 0 && maxCycleLength < 2 {
			return fmt.Errorf("--max-length must be >= 2 (minimum cycle length is 2)")
		}
		if summaryOutputCycles && cyclesTopN <= 0 {
			return fmt.Errorf("-n must be > 0")
		}

		cycles := findAllCyclesWithMaxLength(overview.Graph, maxCycleLength)
		var summary cycleSummary
		if summaryOutputCycles {
			summary = summarizeCycles(cycles, cyclesTopN)
		}

		if !jsonOutputCycles && !summaryOutputCycles {
			fmt.Println("All cycles in dependencies are: ")
			for _, c := range cycles {
				printChain(c)
			}
		}

		if !jsonOutputCycles && summaryOutputCycles {
			printCycleSummary(summary)
		}

		if jsonOutputCycles {
			outputObj := map[string]interface{}{}
			if !summaryOutputCycles {
				outputObj["cycles"] = cycles
			}
			if summaryOutputCycles {
				outputObj["summary"] = summary
			}

			outputRaw, err := json.MarshalIndent(outputObj, "", "\t")
			if err != nil {
				return err
			}
			fmt.Print(string(outputRaw))
		}
		return nil
	},
}

// findAllCycles finds all elementary cycles in the graph using Johnson's algorithm.
// Time complexity: O((V+E)(C+1)) where C is the number of cycles.
func findAllCycles(graph map[string][]string) []Chain {
	return findAllCyclesWithMaxLength(graph, 0)
}

func findAllCyclesWithMaxLength(graph map[string][]string, maxLength int) []Chain {
	// Collect all nodes
	nodeSet := make(map[string]bool)
	for node := range graph {
		nodeSet[node] = true
	}
	for _, deps := range graph {
		for _, dep := range deps {
			nodeSet[dep] = true
		}
	}

	// Create sorted node list for deterministic output
	nodes := make([]string, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	// Create node index mappings
	nodeIndex := make(map[string]int)
	for i, node := range nodes {
		nodeIndex[node] = i
	}

	cf := &cyclesFinder{
		graph:      graph,
		nodeIndex:  nodeIndex,
		indexNode:  nodes,
		blocked:    make([]bool, len(nodes)),
		blockedMap: make([]map[int]bool, len(nodes)),
		stack:      make([]int, 0),
		cycles:     make([]Chain, 0),
		maxLength:  maxLength,
	}

	for i := range cf.blockedMap {
		cf.blockedMap[i] = make(map[int]bool)
	}

	// Johnson's algorithm: iterate through each node as potential cycle start
	for startIdx := 0; startIdx < len(nodes); startIdx++ {
		// Find SCCs in subgraph induced by nodes[startIdx:]
		subgraphSCC := cf.findSCCContaining(startIdx)

		if len(subgraphSCC) > 0 {
			// Reset blocked state for nodes in this SCC
			for _, nodeIdx := range subgraphSCC {
				cf.blocked[nodeIdx] = false
				cf.blockedMap[nodeIdx] = make(map[int]bool)
			}

			// Find cycles starting from startIdx within this SCC
			sccSet := make(map[int]bool)
			for _, idx := range subgraphSCC {
				sccSet[idx] = true
			}
			cf.circuit(startIdx, startIdx, sccSet)
		}
	}

	return cf.cycles
}

// findSCCContaining finds the SCC containing startIdx in the subgraph induced by nodes >= startIdx
func (cf *cyclesFinder) findSCCContaining(startIdx int) []int {
	n := len(cf.indexNode)

	// Tarjan's algorithm on subgraph
	index := 0
	indices := make(map[int]int)
	lowlinks := make(map[int]int)
	onStack := make(map[int]bool)
	stack := make([]int, 0)

	var strongConnect func(v int) []int
	strongConnect = func(v int) []int {
		indices[v] = index
		lowlinks[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for _, neighbor := range cf.graph[cf.indexNode[v]] {
			neighborIdx := cf.nodeIndex[neighbor]
			// Only consider nodes >= startIdx (subgraph restriction)
			if neighborIdx < startIdx {
				continue
			}
			if _, visited := indices[neighborIdx]; !visited {
				result := strongConnect(neighborIdx)
				if result != nil && containsInt(result, startIdx) {
					return result
				}
				if lowlinks[neighborIdx] < lowlinks[v] {
					lowlinks[v] = lowlinks[neighborIdx]
				}
			} else if onStack[neighborIdx] {
				if indices[neighborIdx] < lowlinks[v] {
					lowlinks[v] = indices[neighborIdx]
				}
			}
		}

		// If v is a root of an SCC
		if lowlinks[v] == indices[v] {
			var scc []int
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			// Return SCC containing startIdx if it has more than one node or has a self-loop
			if containsInt(scc, startIdx) && (len(scc) > 1 || cf.hasSelfLoop(startIdx)) {
				return scc
			}
		}
		return nil
	}

	// Start from startIdx
	if _, visited := indices[startIdx]; !visited {
		result := strongConnect(startIdx)
		if result != nil {
			return result
		}
	}

	// Check other nodes in subgraph that might reach startIdx
	for i := startIdx; i < n; i++ {
		if _, visited := indices[i]; !visited {
			result := strongConnect(i)
			if result != nil && containsInt(result, startIdx) {
				return result
			}
		}
	}

	return nil
}

// hasSelfLoop checks if a node has an edge to itself
func (cf *cyclesFinder) hasSelfLoop(nodeIdx int) bool {
	nodeName := cf.indexNode[nodeIdx]
	for _, neighbor := range cf.graph[nodeName] {
		if cf.nodeIndex[neighbor] == nodeIdx {
			return true
		}
	}
	return false
}

// circuit is the main recursive function in Johnson's algorithm
func (cf *cyclesFinder) circuit(v, start int, sccSet map[int]bool) bool {
	found := false
	cf.stack = append(cf.stack, v)
	cf.blocked[v] = true

	for _, neighbor := range cf.graph[cf.indexNode[v]] {
		neighborIdx := cf.nodeIndex[neighbor]

		// Only consider nodes in the current SCC
		if !sccSet[neighborIdx] {
			continue
		}

		if neighborIdx == start {
			// Found a cycle
			if cf.maxLength == 0 || len(cf.stack) <= cf.maxLength {
				cycle := make(Chain, len(cf.stack)+1)
				for i, idx := range cf.stack {
					cycle[i] = cf.indexNode[idx]
				}
				cycle[len(cf.stack)] = cf.indexNode[start]
				cf.cycles = append(cf.cycles, cycle)
				found = true
			}
		} else if !cf.blocked[neighborIdx] && (cf.maxLength == 0 || len(cf.stack) < cf.maxLength) {
			if cf.circuit(neighborIdx, start, sccSet) {
				found = true
			}
		}
	}

	if found {
		cf.unblock(v)
	} else {
		for _, neighbor := range cf.graph[cf.indexNode[v]] {
			neighborIdx := cf.nodeIndex[neighbor]
			if sccSet[neighborIdx] {
				cf.blockedMap[neighborIdx][v] = true
			}
		}
	}

	cf.stack = cf.stack[:len(cf.stack)-1]
	return found
}

// unblock unblocks a node and recursively unblocks nodes that were blocked because of it
func (cf *cyclesFinder) unblock(v int) {
	cf.blocked[v] = false
	for w := range cf.blockedMap[v] {
		delete(cf.blockedMap[v], w)
		if cf.blocked[w] {
			cf.unblock(w)
		}
	}
}

// containsInt checks if a slice contains an integer
func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func summarizeCycles(cycles []Chain, topN int) cycleSummary {
	byLength := map[string]int{}
	twoNodeSeen := map[string]bool{}
	var twoNodeCycles [][]string
	participantCounts := map[string]int{}

	for _, cycle := range cycles {
		if len(cycle) < 2 {
			continue
		}
		cycleLen := len(cycle) - 1
		byLength[strconv.Itoa(cycleLen)]++

		seenInCycle := map[string]bool{}
		for _, module := range cycle[:len(cycle)-1] {
			if !seenInCycle[module] {
				participantCounts[module]++
				seenInCycle[module] = true
			}
		}

		if cycleLen == 2 {
			a, b := cycle[0], cycle[1]
			if b < a {
				a, b = b, a
			}
			key := a + "|" + b
			if !twoNodeSeen[key] {
				twoNodeSeen[key] = true
				twoNodeCycles = append(twoNodeCycles, []string{a, b})
			}
		}
	}

	sort.Slice(twoNodeCycles, func(i, j int) bool {
		if twoNodeCycles[i][0] == twoNodeCycles[j][0] {
			return twoNodeCycles[i][1] < twoNodeCycles[j][1]
		}
		return twoNodeCycles[i][0] < twoNodeCycles[j][0]
	})

	topParticipants := make([]cycleParticipant, 0, len(participantCounts))
	for module, cycleCount := range participantCounts {
		topParticipants = append(topParticipants, cycleParticipant{
			Module:     module,
			CycleCount: cycleCount,
		})
	}
	sort.Slice(topParticipants, func(i, j int) bool {
		if topParticipants[i].CycleCount == topParticipants[j].CycleCount {
			return topParticipants[i].Module < topParticipants[j].Module
		}
		return topParticipants[i].CycleCount > topParticipants[j].CycleCount
	})
	if len(topParticipants) > topN {
		topParticipants = topParticipants[:topN]
	}

	return cycleSummary{
		TotalCycles:     len(cycles),
		ByLength:        byLength,
		TwoNodeCycles:   twoNodeCycles,
		TopParticipants: topParticipants,
	}
}

func printCycleSummary(summary cycleSummary) {
	fmt.Printf("Total cycles: %d\n", summary.TotalCycles)
	fmt.Println("By cycle length:")
	lengths := make([]int, 0, len(summary.ByLength))
	for s := range summary.ByLength {
		l, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		lengths = append(lengths, l)
	}
	sort.Ints(lengths)
	for _, l := range lengths {
		key := strconv.Itoa(l)
		fmt.Printf("- %s: %d\n", key, summary.ByLength[key])
	}

	fmt.Printf("2-node mutual dependencies: %d\n", len(summary.TwoNodeCycles))
	for _, pair := range summary.TwoNodeCycles {
		fmt.Printf("- %s <-> %s\n", pair[0], pair[1])
	}

	fmt.Println("Top participants:")
	for _, p := range summary.TopParticipants {
		fmt.Printf("- %s: %d\n", p.Module, p.CycleCount)
	}
}

func init() {
	rootCmd.AddCommand(cyclesCmd)
	cyclesCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate. Defaults to the current directory.")
	cyclesCmd.Flags().BoolVarP(&jsonOutputCycles, "json", "j", false, "Get the output in JSON format")
	cyclesCmd.Flags().BoolVar(&summaryOutputCycles, "summary", false, "Show cycle summary instead of raw cycle list")
	cyclesCmd.Flags().IntVar(&maxCycleLength, "max-length", 0, "Limit cycles to length <= N (0 = no limit)")
	cyclesCmd.Flags().IntVarP(&cyclesTopN, "top", "n", 10, "Number of top participants to show in summary")
	cyclesCmd.Flags().StringSliceVar(&excludeModules, "exclude-modules", []string{}, "Exclude module path patterns (repeatable, supports * wildcard)")
	cyclesCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Enter modules whose dependencies should be considered direct dependencies; defaults to the first module encountered in `go mod graph` output")
}
