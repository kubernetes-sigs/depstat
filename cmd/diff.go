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
var testOnly bool
var nonTestOnly bool
var diffSplitTestOnly bool

// DiffStats holds the stats for a single analysis
type DiffStats struct {
	DirectDeps int `json:"directDependencies"`
	TransDeps  int `json:"transitiveDependencies"`
	TotalDeps  int `json:"totalDependencies"`
	MaxDepth   int `json:"maxDepthOfDependencies"`
}

// DiffCounts holds filtered dependency counts.
type DiffCounts struct {
	DirectDeps int `json:"directDependencies"`
	TransDeps  int `json:"transitiveDependencies"`
	TotalDeps  int `json:"totalDependencies"`
}

// DiffFilteredSection holds dependency changes/counts for one category.
type DiffFilteredSection struct {
	Before       DiffCounts `json:"before"`
	After        DiffCounts `json:"after"`
	Delta        DiffCounts `json:"delta"`
	Added        []string   `json:"added"`
	Removed      []string   `json:"removed"`
	EdgesAdded   []string   `json:"edgesAdded"`
	EdgesRemoved []string   `json:"edgesRemoved"`
}

// DiffSplitResult holds separate test-only vs non-test dependency changes.
type DiffSplitResult struct {
	TestOnly    DiffFilteredSection `json:"testOnly"`
	NonTestOnly DiffFilteredSection `json:"nonTestOnly"`
}

// DiffResult holds the complete diff analysis
type DiffResult struct {
	Filter         string           `json:"filter,omitempty"`
	BaseRef        string           `json:"baseRef"`
	HeadRef        string           `json:"headRef"`
	Before         DiffStats        `json:"before"`
	After          DiffStats        `json:"after"`
	Delta          DiffStats        `json:"delta"`
	FilteredBefore *DiffCounts      `json:"filteredBefore,omitempty"`
	FilteredAfter  *DiffCounts      `json:"filteredAfter,omitempty"`
	FilteredDelta  *DiffCounts      `json:"filteredDelta,omitempty"`
	Split          *DiffSplitResult `json:"split,omitempty"`
	Added          []string         `json:"added"`
	Removed        []string         `json:"removed"`
	EdgesAdded     []string         `json:"edgesAdded"`
	EdgesRemoved   []string         `json:"edgesRemoved"`
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
	if testOnly && nonTestOnly {
		return fmt.Errorf("--test-only and --non-test-only are mutually exclusive")
	}
	if diffSplitTestOnly && (testOnly || nonTestOnly) {
		return fmt.Errorf("--split-test-only cannot be combined with --test-only or --non-test-only")
	}

	baseRef := args[0]
	headRef := "HEAD"
	if len(args) > 1 {
		headRef = args[1]
	}

	needClassification := diffSplitTestOnly || testOnly || nonTestOnly

	// Save current HEAD to restore later
	originalRef, err := gitCurrentRef()
	if err != nil {
		return fmt.Errorf("failed to get current git ref: %w", err)
	}

	// Resolve symbolic refs (like HEAD, HEAD~1) to SHAs before any
	// checkout, since checkout changes what HEAD points to.
	baseSHA, err := gitResolveRef(baseRef)
	if err != nil {
		return fmt.Errorf("failed to resolve base ref: %w", err)
	}
	headSHA, err := gitResolveRef(headRef)
	if err != nil {
		return fmt.Errorf("failed to resolve head ref: %w", err)
	}

	// Ensure we restore the original state when done
	defer func() {
		_ = gitCheckout(originalRef)
	}()

	// Analyze base ref
	if err := gitCheckout(baseSHA); err != nil {
		return fmt.Errorf("failed to checkout base ref %s: %w", baseRef, err)
	}
	baseDepGraph := getDepInfo(mainModules)
	baseStats := computeStats(baseDepGraph)
	baseDeps := getAllDeps(baseDepGraph.DirectDepList, baseDepGraph.TransDepList)
	baseEdges := getEdges(baseDepGraph.Graph)

	// Classify test-only deps at base ref (while still checked out)
	var baseTestOnly map[string]bool
	if needClassification {
		baseTestOnly, err = classifyTestDeps(baseDeps)
		if err != nil {
			return fmt.Errorf("failed to classify base dependencies as test-only/non-test: %w", err)
		}
	}

	// Analyze head ref
	if err := gitCheckout(headSHA); err != nil {
		return fmt.Errorf("failed to checkout head ref %s: %w", headRef, err)
	}
	headDepGraph := getDepInfo(mainModules)
	headStats := computeStats(headDepGraph)
	headDeps := getAllDeps(headDepGraph.DirectDepList, headDepGraph.TransDepList)
	headEdges := getEdges(headDepGraph.Graph)

	// Classify test-only deps at head ref (while still checked out)
	var headTestOnly map[string]bool
	if needClassification {
		headTestOnly, err = classifyTestDeps(headDeps)
		if err != nil {
			return fmt.Errorf("failed to classify head dependencies as test-only/non-test: %w", err)
		}
	}

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

	// Build split view
	if diffSplitTestOnly {
		result.Split = buildSplitResult(result, baseDepGraph, headDepGraph, baseTestOnly, headTestOnly)
	}

	// Apply test-only filter
	if testOnly || nonTestOnly {
		if testOnly {
			result.Filter = "test-only"
		} else {
			result.Filter = "non-test-only"
		}
		result.Added = filterDepsByTestStatus(result.Added, headTestOnly, testOnly)
		result.Removed = filterDepsByTestStatus(result.Removed, baseTestOnly, testOnly)
		result.EdgesAdded = filterEdgesByTestStatus(result.EdgesAdded, headTestOnly, testOnly)
		result.EdgesRemoved = filterEdgesByTestStatus(result.EdgesRemoved, baseTestOnly, testOnly)

		filteredBefore := computeFilteredCounts(baseDepGraph, baseTestOnly, testOnly)
		filteredAfter := computeFilteredCounts(headDepGraph, headTestOnly, testOnly)
		result.FilteredBefore = &filteredBefore
		result.FilteredAfter = &filteredAfter
		result.FilteredDelta = &DiffCounts{
			DirectDeps: filteredAfter.DirectDeps - filteredBefore.DirectDeps,
			TransDeps:  filteredAfter.TransDeps - filteredBefore.TransDeps,
			TotalDeps:  filteredAfter.TotalDeps - filteredBefore.TotalDeps,
		}
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

// filterDepsByTestStatus filters a list of dependency names.
// If wantTestOnly is true, keeps only deps that are in testOnlySet.
// If wantTestOnly is false, keeps only deps that are NOT in testOnlySet.
func filterDepsByTestStatus(deps []string, testOnlySet map[string]bool, wantTestOnly bool) []string {
	var filtered []string
	for _, dep := range deps {
		isTestOnly := testOnlySet[dep]
		if wantTestOnly == isTestOnly {
			filtered = append(filtered, dep)
		}
	}
	return filtered
}

// filterEdgesByTestStatus filters edges ("from -> to") based on
// whether either endpoint is in the testOnlySet.
func filterEdgesByTestStatus(edges []string, testOnlySet map[string]bool, wantTestOnly bool) []string {
	var filtered []string
	for _, edge := range edges {
		parts := strings.Split(edge, " -> ")
		if len(parts) != 2 {
			continue
		}
		// An edge is test-only if either endpoint is test-only
		isTestOnly := testOnlySet[parts[0]] || testOnlySet[parts[1]]
		if wantTestOnly == isTestOnly {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}

func computeFilteredCounts(depGraph *DependencyOverview, testOnlySet map[string]bool, wantTestOnly bool) DiffCounts {
	direct := filterDepsByTestStatus(depGraph.DirectDepList, testOnlySet, wantTestOnly)
	trans := filterDepsByTestStatus(depGraph.TransDepList, testOnlySet, wantTestOnly)
	return DiffCounts{
		DirectDeps: len(direct),
		TransDeps:  len(trans),
		TotalDeps:  len(getAllDeps(direct, trans)),
	}
}

func buildSplitSection(result DiffResult, beforeGraph, afterGraph *DependencyOverview, beforeTestOnly, afterTestOnly map[string]bool, wantTestOnly bool) DiffFilteredSection {
	beforeCounts := computeFilteredCounts(beforeGraph, beforeTestOnly, wantTestOnly)
	afterCounts := computeFilteredCounts(afterGraph, afterTestOnly, wantTestOnly)
	return DiffFilteredSection{
		Before:       beforeCounts,
		After:        afterCounts,
		Delta:        DiffCounts{DirectDeps: afterCounts.DirectDeps - beforeCounts.DirectDeps, TransDeps: afterCounts.TransDeps - beforeCounts.TransDeps, TotalDeps: afterCounts.TotalDeps - beforeCounts.TotalDeps},
		Added:        filterDepsByTestStatus(result.Added, afterTestOnly, wantTestOnly),
		Removed:      filterDepsByTestStatus(result.Removed, beforeTestOnly, wantTestOnly),
		EdgesAdded:   filterEdgesByTestStatus(result.EdgesAdded, afterTestOnly, wantTestOnly),
		EdgesRemoved: filterEdgesByTestStatus(result.EdgesRemoved, beforeTestOnly, wantTestOnly),
	}
}

func buildSplitResult(result DiffResult, beforeGraph, afterGraph *DependencyOverview, beforeTestOnly, afterTestOnly map[string]bool) *DiffSplitResult {
	return &DiffSplitResult{
		TestOnly:    buildSplitSection(result, beforeGraph, afterGraph, beforeTestOnly, afterTestOnly, true),
		NonTestOnly: buildSplitSection(result, beforeGraph, afterGraph, beforeTestOnly, afterTestOnly, false),
	}
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

func gitResolveRef(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
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
	if result.Filter != "" {
		fmt.Printf("Dependency Diff: %s..%s (%s)\n", result.BaseRef, result.HeadRef, result.Filter)
	} else {
		fmt.Printf("Dependency Diff: %s..%s\n", result.BaseRef, result.HeadRef)
	}
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

	if result.FilteredBefore != nil && result.FilteredAfter != nil && result.FilteredDelta != nil {
		fmt.Printf("Filtered counts for %s dependencies:\n", result.Filter)
		fmt.Println("┌────────────────────┬──────────┬──────────┬─────────┐")
		fmt.Println("│ Metric             │  Before  │  After   │  Delta  │")
		fmt.Println("├────────────────────┼──────────┼──────────┼─────────┤")
		fmt.Printf("│ Direct Deps        │ %8d │ %8d │ %+7d │\n", result.FilteredBefore.DirectDeps, result.FilteredAfter.DirectDeps, result.FilteredDelta.DirectDeps)
		fmt.Printf("│ Transitive Deps    │ %8d │ %8d │ %+7d │\n", result.FilteredBefore.TransDeps, result.FilteredAfter.TransDeps, result.FilteredDelta.TransDeps)
		fmt.Printf("│ Total Deps         │ %8d │ %8d │ %+7d │\n", result.FilteredBefore.TotalDeps, result.FilteredAfter.TotalDeps, result.FilteredDelta.TotalDeps)
		fmt.Println("└────────────────────┴──────────┴──────────┴─────────┘")
		fmt.Println()
	}

	if result.Split != nil {
		fmt.Println("Split by dependency class:")
		fmt.Println()
		printSplitSection("Non-test dependencies", result.Split.NonTestOnly)
		printSplitSection("Test-only dependencies", result.Split.TestOnly)
	}

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

func printSplitSection(title string, sec DiffFilteredSection) {
	fmt.Printf("%s:\n", title)
	fmt.Println("┌────────────────────┬──────────┬──────────┬─────────┐")
	fmt.Println("│ Metric             │  Before  │  After   │  Delta  │")
	fmt.Println("├────────────────────┼──────────┼──────────┼─────────┤")
	fmt.Printf("│ Direct Deps        │ %8d │ %8d │ %+7d │\n", sec.Before.DirectDeps, sec.After.DirectDeps, sec.Delta.DirectDeps)
	fmt.Printf("│ Transitive Deps    │ %8d │ %8d │ %+7d │\n", sec.Before.TransDeps, sec.After.TransDeps, sec.Delta.TransDeps)
	fmt.Printf("│ Total Deps         │ %8d │ %8d │ %+7d │\n", sec.Before.TotalDeps, sec.After.TotalDeps, sec.Delta.TotalDeps)
	fmt.Println("└────────────────────┴──────────┴──────────┴─────────┘")
	fmt.Printf("Added (%d)\n", len(sec.Added))
	fmt.Printf("Removed (%d)\n", len(sec.Removed))
	fmt.Println()
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
	diffCmd.Flags().BoolVar(&testOnly, "test-only", false, "Only show test-only dependency changes (uses go mod why -m)")
	diffCmd.Flags().BoolVar(&nonTestOnly, "non-test-only", false, "Only show non-test (production) dependency changes (uses go mod why -m)")
	diffCmd.Flags().BoolVar(&diffSplitTestOnly, "split-test-only", false, "Split diff output into test-only and non-test sections (uses go mod why -m)")
	_ = diffCmd.Flags().MarkDeprecated("test-only", "use --split-test-only and read split.testOnly")
	_ = diffCmd.Flags().MarkDeprecated("non-test-only", "use --split-test-only and read split.nonTestOnly")
}
