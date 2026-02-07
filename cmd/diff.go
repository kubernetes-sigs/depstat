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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
var vendorFlag bool
var vendorFilesFlag bool

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
	Before         DiffCounts      `json:"before"`
	After          DiffCounts      `json:"after"`
	Delta          DiffCounts      `json:"delta"`
	Added          []string        `json:"added"`
	Removed        []string        `json:"removed"`
	EdgesAdded     []string        `json:"edgesAdded"`
	EdgesRemoved   []string        `json:"edgesRemoved"`
	VersionChanges []VersionChange `json:"versionChanges,omitempty"`
}

// DiffSplitResult holds separate test-only vs non-test dependency changes.
type DiffSplitResult struct {
	TestOnly    DiffFilteredSection `json:"testOnly"`
	NonTestOnly DiffFilteredSection `json:"nonTestOnly"`
}

// VersionChange represents a module whose version changed between refs.
type VersionChange struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// VendorDiffResult holds vendor-level diff information.
type VendorDiffResult struct {
	BeforeCount        int             `json:"beforeCount"`
	AfterCount         int             `json:"afterCount"`
	DeltaCount         int             `json:"deltaCount"`
	Added              []VendorModule  `json:"added"`
	Removed            []VendorModule  `json:"removed"`
	VersionChanges     []VersionChange `json:"versionChanges,omitempty"`
	VendorOnlyRemovals []VendorModule  `json:"vendorOnlyRemovals,omitempty"`
	FilesAdded         []string        `json:"filesAdded,omitempty"`
	FilesDeleted       []string        `json:"filesDeleted,omitempty"`
}

// DiffResult holds the complete diff analysis
type DiffResult struct {
	Filter         string            `json:"filter,omitempty"`
	BaseRef        string            `json:"baseRef"`
	HeadRef        string            `json:"headRef"`
	Before         DiffStats         `json:"before"`
	After          DiffStats         `json:"after"`
	Delta          DiffStats         `json:"delta"`
	FilteredBefore *DiffCounts       `json:"filteredBefore,omitempty"`
	FilteredAfter  *DiffCounts       `json:"filteredAfter,omitempty"`
	FilteredDelta  *DiffCounts       `json:"filteredDelta,omitempty"`
	Split          *DiffSplitResult  `json:"split,omitempty"`
	Added          []string          `json:"added"`
	Removed        []string          `json:"removed"`
	EdgesAdded     []string          `json:"edgesAdded"`
	EdgesRemoved   []string          `json:"edgesRemoved"`
	VersionChanges []VersionChange   `json:"versionChanges,omitempty"`
	Vendor         *VendorDiffResult `json:"vendor,omitempty"`
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
	if dotOutput && svgOutput {
		return fmt.Errorf("--dot and --svg are mutually exclusive")
	}

	baseRef := args[0]
	headRef := "HEAD"
	if len(args) > 1 {
		headRef = args[1]
	}

	needClassification := diffSplitTestOnly || testOnly || nonTestOnly

	// Save current ref state to restore later.
	originalRef, err := gitCurrentRefState()
	if err != nil {
		return fmt.Errorf("failed to get current git ref state: %w", err)
	}
	if dirty, err := gitWorkingTreeDirty(); err != nil {
		return fmt.Errorf("failed to check working tree status: %w", err)
	} else if dirty {
		stashed, stashErr := gitStashPush()
		if stashErr != nil {
			return fmt.Errorf("working tree is dirty and automatic stash failed: %w", stashErr)
		}
		if stashed {
			defer func() {
				if popErr := gitStashPop(); popErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to restore stashed changes: %v\n", popErr)
				}
			}()
		}
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
		if restoreErr := gitCheckout(originalRef); restoreErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to restore git ref %s: %v\n", originalRef, restoreErr)
		}
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
		Added:          diffSlices(baseDeps, headDeps),
		Removed:        diffSlices(headDeps, baseDeps),
		EdgesAdded:     diffSlices(baseEdges, headEdges),
		EdgesRemoved:   diffSlices(headEdges, baseEdges),
		VersionChanges: computeVersionChanges(baseDepGraph, headDepGraph),
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
		result.VersionChanges = filterVersionChangesByTestStatus(result.VersionChanges, headTestOnly, testOnly)

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

	// Vendor diff
	includeVendor := vendorFlag || vendorFilesFlag
	if includeVendor {
		vendor, vendorErr := computeVendorDiff(baseSHA, headSHA, vendorFilesFlag)
		if vendorErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: vendor diff skipped: %v\n", vendorErr)
		} else {
			vendor.VendorOnlyRemovals = computeVendorOnlyRemovals(vendor.Removed, result.Removed)
			result.Vendor = vendor
		}
	}

	// Output based on format
	if jsonOutput {
		return outputJSON(result)
	}
	if dotOutput {
		return outputDOT(result, baseDepGraph, headDepGraph)
	}
	if svgOutput {
		return outputSVG(result, baseDepGraph, headDepGraph)
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
		Before:         beforeCounts,
		After:          afterCounts,
		Delta:          DiffCounts{DirectDeps: afterCounts.DirectDeps - beforeCounts.DirectDeps, TransDeps: afterCounts.TransDeps - beforeCounts.TransDeps, TotalDeps: afterCounts.TotalDeps - beforeCounts.TotalDeps},
		Added:          filterDepsByTestStatus(result.Added, afterTestOnly, wantTestOnly),
		Removed:        filterDepsByTestStatus(result.Removed, beforeTestOnly, wantTestOnly),
		EdgesAdded:     filterEdgesByTestStatus(result.EdgesAdded, afterTestOnly, wantTestOnly),
		EdgesRemoved:   filterEdgesByTestStatus(result.EdgesRemoved, beforeTestOnly, wantTestOnly),
		VersionChanges: filterVersionChangesByTestStatus(result.VersionChanges, afterTestOnly, wantTestOnly),
	}
}

func buildSplitResult(result DiffResult, beforeGraph, afterGraph *DependencyOverview, beforeTestOnly, afterTestOnly map[string]bool) *DiffSplitResult {
	return &DiffSplitResult{
		TestOnly:    buildSplitSection(result, beforeGraph, afterGraph, beforeTestOnly, afterTestOnly, true),
		NonTestOnly: buildSplitSection(result, beforeGraph, afterGraph, beforeTestOnly, afterTestOnly, false),
	}
}

func computeStats(depGraph *DependencyOverview) DiffStats {
	maxDepth := 0
	if len(depGraph.MainModules) > 0 {
		var temp Chain
		longestChain := getLongestChain(depGraph.MainModules[0], depGraph.Graph, temp, map[string]Chain{})
		maxDepth = len(longestChain)
	}
	return DiffStats{
		DirectDeps: len(depGraph.DirectDepList),
		TransDeps:  len(depGraph.TransDepList),
		TotalDeps:  len(getAllDeps(depGraph.DirectDepList, depGraph.TransDepList)),
		MaxDepth:   maxDepth,
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
	cmd := exec.Command("git", "symbolic-ref", "-q", "HEAD")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		detached := exec.Command("git", "rev-parse", "HEAD")
		if dir != "" {
			detached.Dir = dir
		}
		out, err = detached.Output()
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(string(out)), nil
}

func gitCurrentRefState() (string, error) {
	ref, err := gitCurrentRef()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/"), nil
	}
	return ref, nil
}

func gitWorkingTreeDirty() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=no")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func gitStashRef() string {
	cmd := exec.Command("git", "rev-parse", "-q", "--verify", "refs/stash")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitStashPush() (bool, error) {
	before := gitStashRef()
	cmd := exec.Command("git", "stash", "push", "-m", "depstat diff temporary stash")
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, err
	}
	after := gitStashRef()
	return before != after && after != "", nil
}

func gitStashPop() error {
	cmd := exec.Command("git", "stash", "pop", "-q")
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

	printSummary(result)

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

	// Version changes
	if len(result.VersionChanges) > 0 {
		fmt.Printf("Version Changes (%d):\n", len(result.VersionChanges))
		for _, vc := range result.VersionChanges {
			fmt.Printf("  ~ %-50s %s → %s\n", vc.Path, vc.Before, vc.After)
		}
		fmt.Println()
	}

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
		fmt.Println()
	}

	// Vendor changes
	if result.Vendor != nil {
		v := result.Vendor
		fmt.Println("Vendor Changes:")
		fmt.Println("┌────────────────────┬──────────┬──────────┬─────────┐")
		fmt.Println("│ Metric             │  Before  │  After   │  Delta  │")
		fmt.Println("├────────────────────┼──────────┼──────────┼─────────┤")
		fmt.Printf("│ Vendored Modules   │ %8d │ %8d │ %+7d │\n", v.BeforeCount, v.AfterCount, v.DeltaCount)
		fmt.Println("└────────────────────┴──────────┴──────────┴─────────┘")
		fmt.Println()

		fmt.Printf("Vendor Modules Added (%d):\n", len(v.Added))
		if len(v.Added) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, m := range v.Added {
				fmt.Printf("  + %-50s %s\n", m.Path, m.Version)
			}
		}
		fmt.Println()

		fmt.Printf("Vendor Modules Removed (%d):\n", len(v.Removed))
		if len(v.Removed) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, m := range v.Removed {
				fmt.Printf("  - %-50s %s\n", m.Path, m.Version)
			}
		}
		fmt.Println()

		if len(v.VersionChanges) > 0 {
			fmt.Printf("Vendor Version Changes (%d):\n", len(v.VersionChanges))
			for _, vc := range v.VersionChanges {
				fmt.Printf("  ~ %-50s %s → %s\n", vc.Path, vc.Before, vc.After)
			}
			fmt.Println()
		}

		if len(v.VendorOnlyRemovals) > 0 {
			fmt.Printf("Vendor-only Removals (%d):\n", len(v.VendorOnlyRemovals))
			for _, m := range v.VendorOnlyRemovals {
				fmt.Printf("  - %-50s %s\n", m.Path, m.Version)
			}
			fmt.Println()
		}

		if len(v.FilesDeleted) > 0 {
			fmt.Printf("Vendor Files Deleted (%d):\n", len(v.FilesDeleted))
			for _, f := range v.FilesDeleted {
				fmt.Printf("  %s\n", f)
			}
			fmt.Println()
		}

		if len(v.FilesAdded) > 0 {
			fmt.Printf("Vendor Files Added (%d):\n", len(v.FilesAdded))
			for _, f := range v.FilesAdded {
				fmt.Printf("  %s\n", f)
			}
			fmt.Println()
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
	if len(sec.VersionChanges) > 0 {
		fmt.Printf("Version Changes (%d)\n", len(sec.VersionChanges))
	}
	fmt.Println()
}

func outputDOT(result DiffResult, baseGraph, headGraph *DependencyOverview) error {
	fmt.Println("strict digraph {")
	fmt.Println("graph [overlap=false, rankdir=LR, label=\"Dependency Diff: " + result.BaseRef + ".." + result.HeadRef + "\", labelloc=t, fontsize=16];")
	fmt.Println("node [shape=box, style=filled, fillcolor=white, fontsize=11];")
	fmt.Println("edge [fontsize=9];")
	fmt.Println()

	// Build version change lookup
	versionChangeMap := make(map[string]VersionChange)
	for _, vc := range result.VersionChanges {
		versionChangeMap[vc.Path] = vc
	}

	// Collect all diff-relevant nodes BEFORE transitive reduction so
	// the reduction only considers paths through nodes visible in the diff.
	diffNodes := make(map[string]bool)
	for _, dep := range result.Added {
		diffNodes[dep] = true
	}
	for _, dep := range result.Removed {
		diffNodes[dep] = true
	}
	for _, vc := range result.VersionChanges {
		diffNodes[vc.Path] = true
	}
	for _, edge := range result.EdgesAdded {
		parts := strings.Split(edge, " -> ")
		if len(parts) == 2 {
			diffNodes[parts[0]] = true
			diffNodes[parts[1]] = true
		}
	}
	for _, edge := range result.EdgesRemoved {
		parts := strings.Split(edge, " -> ")
		if len(parts) == 2 {
			diffNodes[parts[0]] = true
			diffNodes[parts[1]] = true
		}
	}

	// Transitive reduction within the diff-relevant subgraph.
	edgesAdded := transitiveReduceEdges(result.EdgesAdded, headGraph.Graph, diffNodes)
	edgesRemoved := transitiveReduceEdges(result.EdgesRemoved, baseGraph.Graph, diffNodes)

	// Collect all nodes involved in changes
	changedNodes := make(map[string]string) // node -> status

	for _, dep := range result.Added {
		changedNodes[dep] = "added"
	}
	for _, dep := range result.Removed {
		changedNodes[dep] = "removed"
	}
	for _, vc := range result.VersionChanges {
		if changedNodes[vc.Path] == "" {
			changedNodes[vc.Path] = "changed"
		}
	}

	for _, edge := range edgesAdded {
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
	for _, edge := range edgesRemoved {
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

	// Restore main modules that were pruned by transitive reduction.
	// For each main module that had diff edges but lost them all, add back
	// a single thin edge to its most direct changed dependency.
	reducedEdgeSet := make(map[string]bool)
	for _, e := range edgesAdded {
		reducedEdgeSet[e] = true
	}
	for _, e := range edgesRemoved {
		reducedEdgeSet[e] = true
	}

	var mainModuleEdges []string
	isMainModule := make(map[string]bool)
	for _, m := range baseGraph.MainModules {
		isMainModule[m] = true
	}
	for _, m := range headGraph.MainModules {
		isMainModule[m] = true
	}

	for _, origEdge := range result.EdgesAdded {
		parts := strings.Split(origEdge, " -> ")
		if len(parts) != 2 || !isMainModule[parts[0]] {
			continue
		}
		if changedNodes[parts[0]] != "" {
			continue // already in the graph
		}
		// Find the first changed dep this main module connects to
		target := parts[1]
		if changedNodes[target] != "" {
			changedNodes[parts[0]] = "main"
			mainModuleEdges = append(mainModuleEdges, origEdge)
		}
	}
	for _, origEdge := range result.EdgesRemoved {
		parts := strings.Split(origEdge, " -> ")
		if len(parts) != 2 || !isMainModule[parts[0]] {
			continue
		}
		if changedNodes[parts[0]] != "" {
			continue
		}
		target := parts[1]
		if changedNodes[target] != "" {
			changedNodes[parts[0]] = "main"
			mainModuleEdges = append(mainModuleEdges, origEdge)
		}
	}

	// Deduplicate: keep only one edge per main module
	seenMain := make(map[string]bool)
	var dedupedMainEdges []string
	for _, e := range mainModuleEdges {
		parts := strings.Split(e, " -> ")
		if !seenMain[parts[0]] {
			seenMain[parts[0]] = true
			dedupedMainEdges = append(dedupedMainEdges, e)
		}
	}
	mainModuleEdges = dedupedMainEdges

	// Output nodes with colors
	fmt.Println("// Nodes")
	var nodeNames []string
	for n := range changedNodes {
		nodeNames = append(nodeNames, n)
	}
	sort.Strings(nodeNames)

	for _, node := range nodeNames {
		status := changedNodes[node]
		color := "white"
		style := "filled"
		label := node
		switch status {
		case "added":
			color = "#ccffcc" // green
		case "removed":
			color = "#ffcccc" // red
			style = "filled,dashed"
		case "changed":
			color = "#ffffcc" // yellow
			if vc, ok := versionChangeMap[node]; ok {
				label = fmt.Sprintf("%s\\n%s → %s", node, vc.Before, vc.After)
			}
		case "main":
			color = "#e8e8e8" // light gray
		}
		fmt.Printf("\"%s\" [fillcolor=\"%s\", style=\"%s\", label=\"%s\"];\n", node, color, style, label)
	}
	fmt.Println()

	// Output main module edges (thin, gray)
	if len(mainModuleEdges) > 0 {
		fmt.Println("// Main module edges")
		for _, edge := range mainModuleEdges {
			parts := strings.Split(edge, " -> ")
			if len(parts) == 2 {
				fmt.Printf("\"%s\" -> \"%s\" [color=\"gray\", style=\"dotted\"];\n", parts[0], parts[1])
			}
		}
		fmt.Println()
	}

	// Output reduced edges
	if len(edgesRemoved) > 0 {
		fmt.Println("// Removed edges")
		for _, edge := range edgesRemoved {
			parts := strings.Split(edge, " -> ")
			if len(parts) == 2 {
				fmt.Printf("\"%s\" -> \"%s\" [color=\"red\", style=\"dashed\"];\n", parts[0], parts[1])
			}
		}
		fmt.Println()
	}

	if len(edgesAdded) > 0 {
		fmt.Println("// Added edges")
		for _, edge := range edgesAdded {
			parts := strings.Split(edge, " -> ")
			if len(parts) == 2 {
				fmt.Printf("\"%s\" -> \"%s\" [color=\"green\", style=\"bold\"];\n", parts[0], parts[1])
			}
		}
	}

	fmt.Println("}")
	return nil
}

func outputSVG(result DiffResult, baseGraph, headGraph *DependencyOverview) error {
	dot, err := captureDOTOutput(func() error {
		return outputDOT(result, baseGraph, headGraph)
	})
	if err != nil {
		return err
	}

	cmd := exec.Command("dot", "-Tsvg")
	cmd.Stdin = strings.NewReader(dot)
	cmd.Stdout = os.Stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to render DOT as SVG via graphviz 'dot': %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func captureDOTOutput(fn func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, readErr := io.Copy(&buf, r)
	_ = r.Close()

	if runErr != nil {
		return "", runErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if readErr != nil {
		return "", readErr
	}
	return buf.String(), nil
}

// transitiveReduceEdges removes diff edges that are implied by longer paths
// through the diff-relevant subgraph, but only when the alternative path
// contains at least one other diff edge. This prevents genuinely new edges
// (like A newly depending on B) from being pruned via pre-existing paths.
func transitiveReduceEdges(diffEdges []string, fullGraph map[string][]string, diffNodes map[string]bool) []string {
	diffEdgeSet := make(map[string]bool)
	for _, e := range diffEdges {
		diffEdgeSet[e] = true
	}

	// Project the full graph onto just the diff-relevant nodes
	subGraph := make(map[string][]string)
	for node := range diffNodes {
		for _, neighbor := range fullGraph[node] {
			if diffNodes[neighbor] {
				subGraph[node] = append(subGraph[node], neighbor)
			}
		}
	}

	var reduced []string
	for _, edge := range diffEdges {
		parts := strings.Split(edge, " -> ")
		if len(parts) != 2 {
			continue
		}
		if !reachableViaDiffPath(parts[0], parts[1], subGraph, diffEdgeSet) {
			reduced = append(reduced, edge)
		}
	}
	return reduced
}

// reachableViaDiffPath checks if dst is reachable from src via a path of
// length > 1 (excluding the direct src→dst edge) where at least one
// intermediate edge is itself a diff edge.
func reachableViaDiffPath(src, dst string, graph map[string][]string, diffEdgeSet map[string]bool) bool {
	type state struct {
		node    string
		hasDiff bool
	}
	reachedWithDiff := make(map[string]bool)
	reachedNoDiff := make(map[string]bool)
	queue := []state{}

	for _, n := range graph[src] {
		if n == dst {
			continue
		}
		hasDiff := diffEdgeSet[src+" -> "+n]
		if hasDiff {
			if !reachedWithDiff[n] {
				reachedWithDiff[n] = true
				queue = append(queue, state{n, true})
			}
		} else if !reachedNoDiff[n] {
			reachedNoDiff[n] = true
			queue = append(queue, state{n, false})
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.node == dst && cur.hasDiff {
			return true
		}

		for _, n := range graph[cur.node] {
			hasDiff := cur.hasDiff || diffEdgeSet[cur.node+" -> "+n]
			if hasDiff {
				if !reachedWithDiff[n] {
					reachedWithDiff[n] = true
					queue = append(queue, state{n, true})
				}
			} else if !reachedNoDiff[n] {
				reachedNoDiff[n] = true
				queue = append(queue, state{n, false})
			}
		}
	}

	return false
}

// computeVersionChanges returns modules present in both base and head
// whose effective versions differ.
func computeVersionChanges(base, head *DependencyOverview) []VersionChange {
	var changes []VersionChange
	headDeps := make(map[string]bool)
	for _, dep := range getAllDeps(head.DirectDepList, head.TransDepList) {
		headDeps[dep] = true
	}
	for _, dep := range getAllDeps(base.DirectDepList, base.TransDepList) {
		if !headDeps[dep] {
			continue // removed module, not a version change
		}
		baseVer := base.Versions[dep]
		headVer := head.Versions[dep]
		if baseVer != "" && headVer != "" && baseVer != headVer {
			changes = append(changes, VersionChange{Path: dep, Before: baseVer, After: headVer})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})
	return changes
}

// filterVersionChangesByTestStatus filters version changes by test-only status.
func filterVersionChangesByTestStatus(changes []VersionChange, testOnlySet map[string]bool, wantTestOnly bool) []VersionChange {
	var filtered []VersionChange
	for _, vc := range changes {
		isTestOnly := testOnlySet[vc.Path]
		if wantTestOnly == isTestOnly {
			filtered = append(filtered, vc)
		}
	}
	return filtered
}

// computeVendorDiff computes vendor-level changes between two git refs
// by parsing vendor/modules.txt at each ref.
func computeVendorDiff(baseSHA, headSHA string, includeFiles bool) (*VendorDiffResult, error) {
	baseContent, baseOK := gitShowFile(baseSHA, "vendor/modules.txt")
	headContent, headOK := gitShowFile(headSHA, "vendor/modules.txt")

	if !baseOK && !headOK {
		return nil, fmt.Errorf("vendor/modules.txt not found at either ref")
	}

	baseModules := parseVendorModulesTxt(baseContent)
	headModules := parseVendorModulesTxt(headContent)

	baseMap := make(map[string]string)
	for _, m := range baseModules {
		baseMap[m.Path] = m.Version
	}
	headMap := make(map[string]string)
	for _, m := range headModules {
		headMap[m.Path] = m.Version
	}

	result := &VendorDiffResult{
		BeforeCount: len(baseModules),
		AfterCount:  len(headModules),
		DeltaCount:  len(headModules) - len(baseModules),
	}

	// Find added and version changes
	for _, m := range headModules {
		if baseVer, ok := baseMap[m.Path]; !ok {
			result.Added = append(result.Added, m)
		} else if baseVer != m.Version {
			result.VersionChanges = append(result.VersionChanges, VersionChange{
				Path: m.Path, Before: baseVer, After: m.Version,
			})
		}
	}

	// Find removed
	for _, m := range baseModules {
		if _, ok := headMap[m.Path]; !ok {
			result.Removed = append(result.Removed, m)
		}
	}

	// Sort results
	sort.Slice(result.Added, func(i, j int) bool { return result.Added[i].Path < result.Added[j].Path })
	sort.Slice(result.Removed, func(i, j int) bool { return result.Removed[i].Path < result.Removed[j].Path })
	sort.Slice(result.VersionChanges, func(i, j int) bool { return result.VersionChanges[i].Path < result.VersionChanges[j].Path })

	// File-level diff if requested
	if includeFiles {
		added, deleted, err := gitDiffFiles(baseSHA, headSHA, "vendor/")
		if err == nil {
			for _, f := range added {
				if strings.HasSuffix(f, ".go") {
					result.FilesAdded = append(result.FilesAdded, f)
				}
			}
			for _, f := range deleted {
				if strings.HasSuffix(f, ".go") {
					result.FilesDeleted = append(result.FilesDeleted, f)
				}
			}
		}
	}

	return result, nil
}

func computeVendorOnlyRemovals(vendorRemoved []VendorModule, graphRemoved []string) []VendorModule {
	removedFromGraph := make(map[string]bool, len(graphRemoved))
	for _, dep := range graphRemoved {
		removedFromGraph[dep] = true
	}
	var out []VendorModule
	for _, m := range vendorRemoved {
		if !removedFromGraph[m.Path] {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func printSummary(result DiffResult) {
	fmt.Println("Summary:")
	fmt.Printf("  Module graph: +%d added, -%d removed, ~%d version changes\n",
		len(result.Added), len(result.Removed), len(result.VersionChanges))
	if result.Split != nil {
		fmt.Printf("  Non-test:     +%d added, -%d removed, ~%d version changes\n",
			len(result.Split.NonTestOnly.Added), len(result.Split.NonTestOnly.Removed), len(result.Split.NonTestOnly.VersionChanges))
		fmt.Printf("  Test-only:    +%d added, -%d removed, ~%d version changes\n",
			len(result.Split.TestOnly.Added), len(result.Split.TestOnly.Removed), len(result.Split.TestOnly.VersionChanges))
	}
	if result.Vendor != nil {
		v := result.Vendor
		fmt.Printf("  Vendor:       +%d added, -%d removed, ~%d version changes\n",
			len(v.Added), len(v.Removed), len(v.VersionChanges))
		if len(v.FilesAdded) > 0 || len(v.FilesDeleted) > 0 {
			fmt.Printf("  Vendor files: +%d added, -%d deleted\n", len(v.FilesAdded), len(v.FilesDeleted))
		}
	}
	fmt.Println("  Key events:")
	if len(result.VersionChanges) > 0 && len(result.Added) == 0 && len(result.Removed) == 0 {
		fmt.Println("    - Dependency set unchanged, but versions changed")
	}
	if result.Vendor != nil && len(result.Vendor.VendorOnlyRemovals) > 0 {
		fmt.Printf("    - %d modules removed from vendor but still in module graph\n", len(result.Vendor.VendorOnlyRemovals))
	}
	if result.Vendor != nil && len(result.Vendor.FilesDeleted) > 0 {
		fmt.Printf("    - %d vendored Go files deleted (possible API removals)\n", len(result.Vendor.FilesDeleted))
	}
	if len(result.VersionChanges) == 0 && len(result.Added) == 0 && len(result.Removed) == 0 &&
		(result.Vendor == nil || (len(result.Vendor.VersionChanges) == 0 && len(result.Vendor.Added) == 0 && len(result.Vendor.Removed) == 0 && len(result.Vendor.FilesDeleted) == 0 && len(result.Vendor.FilesAdded) == 0)) {
		fmt.Println("    - No dependency changes detected")
	}
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate")
	diffCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	diffCmd.Flags().BoolVarP(&dotOutput, "dot", "", false, "Output in DOT format for Graphviz")
	diffCmd.Flags().BoolVarP(&svgOutput, "svg", "s", false, "Render DOT output as SVG (requires graphviz 'dot')")
	diffCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Include edge-level changes")
	diffCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Specify main modules")
	diffCmd.Flags().BoolVar(&testOnly, "test-only", false, "Only show test-only dependency changes (uses go mod why -m)")
	diffCmd.Flags().BoolVar(&nonTestOnly, "non-test-only", false, "Only show non-test (production) dependency changes (uses go mod why -m)")
	diffCmd.Flags().BoolVar(&diffSplitTestOnly, "split-test-only", false, "Split diff output into test-only and non-test sections (uses go mod why -m)")
	_ = diffCmd.Flags().MarkDeprecated("test-only", "use --split-test-only and read split.testOnly")
	_ = diffCmd.Flags().MarkDeprecated("non-test-only", "use --split-test-only and read split.nonTestOnly")
	diffCmd.Flags().BoolVar(&vendorFlag, "vendor", false, "Include vendor-level diff using vendor/modules.txt")
	diffCmd.Flags().BoolVar(&vendorFilesFlag, "vendor-files", false, "Report added/deleted Go files in vendor/ (implies --vendor)")
}
