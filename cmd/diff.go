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
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var dotOutput bool
var svgOutput bool

// DiffStats holds the stats for a single analysis
type DiffStats struct {
	DirectDeps int `json:"directDependencies"`
	TransDeps  int `json:"transitiveDependencies"`
	TotalDeps  int `json:"totalDependencies"`
	MaxDepth   int `json:"maxDepthOfDependencies"`
}

// DiffResult holds the complete diff analysis
type DiffResult struct {
	BaseRef      string    `json:"baseRef"`
	HeadRef      string    `json:"headRef"`
	Before       DiffStats `json:"before"`
	After        DiffStats `json:"after"`
	Delta        DiffStats `json:"delta"`
	Added        []string  `json:"added"`
	Removed      []string  `json:"removed"`
	EdgesAdded   []string  `json:"edgesAdded"`
	EdgesRemoved []string  `json:"edgesRemoved"`
}

var diffCmd = &cobra.Command{
	Use:   "diff <base-ref> [head-ref]",
	Short: "Compare dependencies between two git refs",
	Long: `Compare dependency changes between two git commits, branches, or tags.

Examples:
  # Compare current HEAD with main branch
  depstat diff main

  # Compare two specific commits
  depstat diff abc123 def456

  # Output as JSON for CI processing
  depstat diff main --json

  # Output as DOT format for visualization
  depstat diff main --dot | dot -Tsvg -o diff.svg`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDiff,
}

func runDiff(cmd *cobra.Command, args []string) error {
	baseRef := args[0]
	headRef := "HEAD"
	if len(args) > 1 {
		headRef = args[1]
	}

	// Save current HEAD to restore later
	originalRef, err := gitCurrentRef()
	if err != nil {
		return fmt.Errorf("failed to get current git ref: %w", err)
	}

	// Ensure we restore the original state when done
	defer func() {
		_ = gitCheckout(originalRef)
	}()

	// Analyze base ref
	if err := gitCheckout(baseRef); err != nil {
		return fmt.Errorf("failed to checkout base ref %s: %w", baseRef, err)
	}
	baseDepGraph := getDepInfo(mainModules)
	baseStats := computeStats(baseDepGraph)
	baseDeps := getAllDeps(baseDepGraph.DirectDepList, baseDepGraph.TransDepList)
	baseEdges := getEdges(baseDepGraph.Graph)

	// Analyze head ref
	if err := gitCheckout(headRef); err != nil {
		return fmt.Errorf("failed to checkout head ref %s: %w", headRef, err)
	}
	headDepGraph := getDepInfo(mainModules)
	headStats := computeStats(headDepGraph)
	headDeps := getAllDeps(headDepGraph.DirectDepList, headDepGraph.TransDepList)
	headEdges := getEdges(headDepGraph.Graph)

	// Compute diff
	result := DiffResult{
		BaseRef: baseRef,
		HeadRef: headRef,
		Before:  baseStats,
		After:   headStats,
		Delta: DiffStats{
			DirectDeps: headStats.DirectDeps - baseStats.DirectDeps,
			TransDeps:  headStats.TransDeps - baseStats.TransDeps,
			TotalDeps:  headStats.TotalDeps - baseStats.TotalDeps,
			MaxDepth:   headStats.MaxDepth - baseStats.MaxDepth,
		},
		Added:        diffSlices(baseDeps, headDeps),
		Removed:      diffSlices(headDeps, baseDeps),
		EdgesAdded:   diffSlices(baseEdges, headEdges),
		EdgesRemoved: diffSlices(headEdges, baseEdges),
	}

	// Output based on format
	if jsonOutput {
		return outputJSON(result)
	}
	if dotOutput {
		return outputDOT(result, baseDepGraph, headDepGraph)
	}
	return outputText(result)
}

func computeStats(depGraph *DependencyOverview) DiffStats {
	var temp Chain
	longestChain := getLongestChain(depGraph.MainModules[0], depGraph.Graph, temp, map[string]Chain{})
	return DiffStats{
		DirectDeps: len(depGraph.DirectDepList),
		TransDeps:  len(depGraph.TransDepList),
		TotalDeps:  len(getAllDeps(depGraph.DirectDepList, depGraph.TransDepList)),
		MaxDepth:   len(longestChain),
	}
}

func getEdges(graph map[string][]string) []string {
	var edges []string
	for from, tos := range graph {
		for _, to := range tos {
			edges = append(edges, fmt.Sprintf("%s -> %s", from, to))
		}
	}
	sort.Strings(edges)
	return edges
}

// diffSlices returns items in b that are not in a
func diffSlices(a, b []string) []string {
	aMap := make(map[string]bool)
	for _, item := range a {
		aMap[item] = true
	}
	var diff []string
	for _, item := range b {
		if !aMap[item] {
			diff = append(diff, item)
		}
	}
	sort.Strings(diff)
	return diff
}

func gitCurrentRef() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitCheckout(ref string) error {
	cmd := exec.Command("git", "checkout", "-q", ref)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func outputJSON(result DiffResult) error {
	out, err := json.MarshalIndent(result, "", "\t")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func outputText(result DiffResult) error {
	fmt.Printf("Dependency Diff: %s..%s\n", result.BaseRef, result.HeadRef)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	// Metrics table
	fmt.Println("Metrics:")
	fmt.Println("┌────────────────────┬──────────┬──────────┬─────────┐")
	fmt.Println("│ Metric             │  Before  │  After   │  Delta  │")
	fmt.Println("├────────────────────┼──────────┼──────────┼─────────┤")
	fmt.Printf("│ Direct Deps        │ %8d │ %8d │ %+7d │\n", result.Before.DirectDeps, result.After.DirectDeps, result.Delta.DirectDeps)
	fmt.Printf("│ Transitive Deps    │ %8d │ %8d │ %+7d │\n", result.Before.TransDeps, result.After.TransDeps, result.Delta.TransDeps)
	fmt.Printf("│ Total Deps         │ %8d │ %8d │ %+7d │\n", result.Before.TotalDeps, result.After.TotalDeps, result.Delta.TotalDeps)
	fmt.Printf("│ Max Depth          │ %8d │ %8d │ %+7d │\n", result.Before.MaxDepth, result.After.MaxDepth, result.Delta.MaxDepth)
	fmt.Println("└────────────────────┴──────────┴──────────┴─────────┘")
	fmt.Println()

	// Dependencies added
	fmt.Printf("Dependencies Added (%d):\n", len(result.Added))
	if len(result.Added) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, dep := range result.Added {
			fmt.Printf("  + %s\n", dep)
		}
	}
	fmt.Println()

	// Dependencies removed
	fmt.Printf("Dependencies Removed (%d):\n", len(result.Removed))
	if len(result.Removed) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, dep := range result.Removed {
			fmt.Printf("  - %s\n", dep)
		}
	}
	fmt.Println()

	// Edge changes (verbose only)
	if verbose {
		fmt.Printf("Edges Added (%d):\n", len(result.EdgesAdded))
		for _, edge := range result.EdgesAdded {
			fmt.Printf("  + %s\n", edge)
		}
		fmt.Println()

		fmt.Printf("Edges Removed (%d):\n", len(result.EdgesRemoved))
		for _, edge := range result.EdgesRemoved {
			fmt.Printf("  - %s\n", edge)
		}
	}

	return nil
}

func outputDOT(result DiffResult, baseGraph, headGraph *DependencyOverview) error {
	fmt.Println("strict digraph {")
	fmt.Println("graph [overlap=false, label=\"Dependency Diff: " + result.BaseRef + ".." + result.HeadRef + "\", labelloc=t];")
	fmt.Println("node [shape=box, style=filled, fillcolor=white];")
	fmt.Println()

	// Collect all nodes involved in changes
	changedNodes := make(map[string]string) // node -> status (added/removed/unchanged)

	for _, dep := range result.Added {
		changedNodes[dep] = "added"
	}
	for _, dep := range result.Removed {
		changedNodes[dep] = "removed"
	}

	// Add nodes involved in edge changes
	for _, edge := range result.EdgesAdded {
		parts := strings.Split(edge, " -> ")
		if len(parts) == 2 {
			if changedNodes[parts[0]] == "" {
				changedNodes[parts[0]] = "unchanged"
			}
			if changedNodes[parts[1]] == "" {
				changedNodes[parts[1]] = "added"
			}
		}
	}
	for _, edge := range result.EdgesRemoved {
		parts := strings.Split(edge, " -> ")
		if len(parts) == 2 {
			if changedNodes[parts[0]] == "" {
				changedNodes[parts[0]] = "unchanged"
			}
			if changedNodes[parts[1]] == "" {
				changedNodes[parts[1]] = "removed"
			}
		}
	}

	// Output nodes with colors
	fmt.Println("// Nodes")
	for node, status := range changedNodes {
		color := "white"
		style := "filled"
		switch status {
		case "added":
			color = "#ccffcc" // green
		case "removed":
			color = "#ffcccc" // red
			style = "filled,dashed"
		}
		fmt.Printf("\"%s\" [fillcolor=\"%s\", style=\"%s\"];\n", node, color, style)
	}
	fmt.Println()

	// Output removed edges
	fmt.Println("// Removed edges")
	for _, edge := range result.EdgesRemoved {
		parts := strings.Split(edge, " -> ")
		if len(parts) == 2 {
			fmt.Printf("\"%s\" -> \"%s\" [color=\"red\", style=\"dashed\", label=\"REMOVED\"];\n", parts[0], parts[1])
		}
	}
	fmt.Println()

	// Output added edges
	fmt.Println("// Added edges")
	for _, edge := range result.EdgesAdded {
		parts := strings.Split(edge, " -> ")
		if len(parts) == 2 {
			fmt.Printf("\"%s\" -> \"%s\" [color=\"green\", style=\"bold\", label=\"ADDED\"];\n", parts[0], parts[1])
		}
	}

	fmt.Println("}")
	return nil
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate")
	diffCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	diffCmd.Flags().BoolVarP(&dotOutput, "dot", "", false, "Output in DOT format for Graphviz")
	diffCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Include edge-level changes")
	diffCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Specify main modules")
}
