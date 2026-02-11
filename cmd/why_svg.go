/*
Copyright 2025 The Kubernetes Authors.

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
	"fmt"
	"math"
	"sort"
	"strings"
)

type svgEdge struct {
	From, To string
}

type nodePos struct {
	X, Y, W, H float64
}

type nodeColor struct {
	Fill, Stroke, Text string
}

const (
	svgMinNodeWidth = 160.0
	svgNodeHeight   = 34.0
	svgLayerSpacing = 110.0
	svgNodeSpacing  = 24.0
	svgPaddingX     = 40.0
	svgPaddingTop   = 90.0
	svgCornerRadius = 6.0
	svgFontSize     = 11.0
	svgCharWidth    = 6.8
	svgMinWidth     = 500.0
	svgMaxWidth     = 2400.0
)

func outputWhySVG(result WhyResult) error {
	// Use pre-computed subgraph if available (O(V+E) fast path),
	// otherwise extract from enumerated paths (backward compat).
	var nodeSet map[string]bool
	var edgeSet map[svgEdge]bool

	if result.NodeSet != nil {
		nodeSet = result.NodeSet
		edgeSet = result.EdgeSet
	} else if len(result.Paths) > 0 {
		nodeSet = make(map[string]bool)
		edgeSet = make(map[svgEdge]bool)
		for _, wp := range result.Paths {
			for i, node := range wp.Path {
				nodeSet[node] = true
				if i > 0 {
					edgeSet[svgEdge{From: wp.Path[i-1], To: node}] = true
				}
			}
		}
	}

	if len(nodeSet) == 0 {
		fmt.Printf(`<svg xmlns="http://www.w3.org/2000/svg" width="400" height="80">
<text x="200" y="40" text-anchor="middle" font-family="sans-serif" font-size="14">No dependency paths found for %s</text>
</svg>
`, xmlEscape(result.Target))
		return nil
	}

	// Assign layers via BFS using longest path from root
	layerOf := assignLayers(nodeSet, edgeSet, result)

	// Group nodes by layer
	numLayers := 0
	layerNodes := make(map[int][]string)
	for node, l := range layerOf {
		layerNodes[l] = append(layerNodes[l], node)
		if l+1 > numLayers {
			numLayers = l + 1
		}
	}
	for l := range layerNodes {
		sort.Strings(layerNodes[l])
	}

	// Compute node labels and widths
	labels := make(map[string]string)
	widths := make(map[string]float64)
	for node := range nodeSet {
		label := abbreviateModule(node, result.MainModules)
		labels[node] = label
		w := math.Max(svgMinNodeWidth, float64(len(label))*svgCharWidth+24)
		widths[node] = w
	}

	// Find the widest layer to set SVG width
	maxLayerWidth := 0.0
	for l := 0; l < numLayers; l++ {
		var tw float64
		for _, node := range layerNodes[l] {
			tw += widths[node]
		}
		tw += float64(len(layerNodes[l])-1) * svgNodeSpacing
		if tw > maxLayerWidth {
			maxLayerWidth = tw
		}
	}
	svgWidth := math.Max(svgMinWidth, math.Min(svgMaxWidth, maxLayerWidth+2*svgPaddingX))
	svgHeight := svgPaddingTop + float64(numLayers-1)*svgLayerSpacing + svgNodeHeight + 40

	// Compute positions (centered per layer)
	positions := make(map[string]nodePos)
	for l := 0; l < numLayers; l++ {
		nodes := layerNodes[l]
		var totalW float64
		for _, n := range nodes {
			totalW += widths[n]
		}
		totalW += float64(len(nodes)-1) * svgNodeSpacing
		x := (svgWidth - totalW) / 2
		y := svgPaddingTop + float64(l)*svgLayerSpacing
		for _, n := range nodes {
			positions[n] = nodePos{X: x, Y: y, W: widths[n], H: svgNodeHeight}
			x += widths[n] + svgNodeSpacing
		}
	}

	// Build SVG
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="system-ui,-apple-system,sans-serif">`, svgWidth, svgHeight, svgWidth, svgHeight)
	fmt.Fprintln(&b)

	// Defs: arrow markers
	fmt.Fprint(&b, `<defs>
  <marker id="a" viewBox="0 0 10 6" refX="10" refY="3" markerWidth="8" markerHeight="5" orient="auto-start-reverse">
    <path d="M0 0L10 3L0 6z" fill="#888"/>
  </marker>
  <marker id="ar" viewBox="0 0 10 6" refX="10" refY="3" markerWidth="8" markerHeight="5" orient="auto-start-reverse">
    <path d="M0 0L10 3L0 6z" fill="#D32F2F"/>
  </marker>
</defs>
`)

	// Title
	fmt.Fprintf(&b, `<text x="%.1f" y="28" text-anchor="middle" font-size="14" font-weight="600" fill="#333">Why is %s included?</text>`, svgWidth/2, xmlEscape(result.Target))
	fmt.Fprintln(&b)
	pathCount := len(result.Paths)
	if pathCount == 0 {
		pathCount = result.TotalPaths
	}
	fmt.Fprintf(&b, `<text x="%.1f" y="46" text-anchor="middle" font-size="11" fill="#888">%d paths, %d direct dependent(s)</text>`, svgWidth/2, pathCount, len(result.DirectDeps))
	fmt.Fprintln(&b)

	// Legend
	renderSVGLegend(&b, 16, 60)

	// Edges (before nodes so nodes draw on top)
	directDepSet := make(map[string]bool)
	for _, d := range result.DirectDeps {
		directDepSet[d] = true
	}

	for e := range edgeSet {
		fp := positions[e.From]
		tp := positions[e.To]
		path := svgBezierPath(fp, tp)

		isDirectToTarget := e.To == result.Target && directDepSet[e.From]
		layerDiff := layerOf[e.To] - layerOf[e.From]

		stroke := "#888"
		sw := "1.3"
		marker := "url(#a)"
		dash := ""

		if isDirectToTarget {
			stroke = "#D32F2F"
			sw = "2.2"
			marker = "url(#ar)"
		} else if layerDiff > 1 {
			dash = ` stroke-dasharray="5,3"`
		}

		fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="%s" marker-end="%s"%s/>`, path, stroke, sw, marker, dash)
		fmt.Fprintln(&b)
	}

	// Nodes
	sortedNodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		sortedNodes = append(sortedNodes, n)
	}
	sort.Strings(sortedNodes)

	for _, node := range sortedNodes {
		p := positions[node]
		c := classifyNodeColor(node, result)
		sw := "1.5"
		if node == result.Target || contains(result.MainModules, node) {
			sw = "2"
		}
		fmt.Fprintf(&b, `<g><title>%s</title>`, xmlEscape(node))
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.0f" fill="%s" stroke="%s" stroke-width="%s"/>`,
			p.X, p.Y, p.W, p.H, svgCornerRadius, c.Fill, c.Stroke, sw)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" dominant-baseline="central" font-size="%.0f" fill="%s">%s</text>`,
			p.X+p.W/2, p.Y+p.H/2, svgFontSize, c.Text, xmlEscape(labels[node]))
		fmt.Fprintln(&b, `</g>`)
	}

	// Footer
	fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" text-anchor="middle" font-size="10" fill="#aaa">generated by depstat</text>`,
		svgWidth/2, svgHeight-12)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, `</svg>`)
	fmt.Print(b.String())
	return nil
}

// assignLayers does BFS from main modules, using the longest path from root
// so that the target naturally sinks to the bottom layer.
func assignLayers(nodeSet map[string]bool, edgeSet map[svgEdge]bool, result WhyResult) map[string]int {
	layerOf := make(map[string]int)

	// Build adjacency list from edge set
	adj := make(map[string][]string)
	for e := range edgeSet {
		adj[e.From] = append(adj[e.From], e.To)
	}

	// Initialize roots at layer 0
	queue := make([]string, 0)
	for _, m := range result.MainModules {
		if nodeSet[m] {
			layerOf[m] = 0
			queue = append(queue, m)
		}
	}

	// BFS assigning max depth, with safety cap to prevent infinite loops
	// in case the edge set contains cycles.
	maxIter := len(nodeSet) * len(nodeSet) * 2
	iter := 0
	for len(queue) > 0 {
		if iter >= maxIter {
			break
		}
		iter++
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if !nodeSet[next] {
				continue
			}
			newLayer := layerOf[cur] + 1
			if prev, ok := layerOf[next]; !ok || newLayer > prev {
				layerOf[next] = newLayer
				queue = append(queue, next)
			}
		}
	}

	// Ensure target is at the bottom
	maxLayer := 0
	for _, l := range layerOf {
		if l > maxLayer {
			maxLayer = l
		}
	}
	if layerOf[result.Target] < maxLayer {
		layerOf[result.Target] = maxLayer
	}

	return layerOf
}

func classifyNodeColor(node string, result WhyResult) nodeColor {
	if node == result.Target {
		return nodeColor{"#FFE0E0", "#D32F2F", "#B71C1C"}
	}
	if contains(result.MainModules, node) {
		return nodeColor{"#E8F5E9", "#388E3C", "#1B5E20"}
	}
	// Check if same org as first main module
	if len(result.MainModules) > 0 {
		main := result.MainModules[0]
		if idx := strings.Index(main, "/"); idx > 0 {
			prefix := main[:idx+1]
			if strings.HasPrefix(node, prefix) {
				return nodeColor{"#E3F2FD", "#1976D2", "#0D47A1"}
			}
		}
	}
	return nodeColor{"#FFF3E0", "#F57C00", "#E65100"}
}

func abbreviateModule(mod string, mainModules []string) string {
	const maxLen = 38
	if len(mod) <= maxLen {
		return mod
	}
	// Try stripping the domain prefix shared with main module
	if len(mainModules) > 0 {
		main := mainModules[0]
		if idx := strings.Index(main, "/"); idx > 0 {
			prefix := main[:idx+1]
			if strings.HasPrefix(mod, prefix) {
				short := mod[idx+1:]
				if len(short) <= maxLen-4 {
					return ".../" + short
				}
			}
		}
	}
	// Truncate middle
	half := (maxLen - 3) / 2
	return mod[:half] + "..." + mod[len(mod)-half:]
}

func svgBezierPath(from, to nodePos) string {
	x1 := from.X + from.W/2
	y1 := from.Y + from.H
	x2 := to.X + to.W/2
	y2 := to.Y
	cx := (x1 + x2) / 2
	cy := (y1 + y2) / 2
	return fmt.Sprintf("M%.1f %.1fQ%.1f %.1f %.1f %.1f", x1, y1, cx, cy, x2, y2)
}

func renderSVGLegend(b *strings.Builder, x, y float64) {
	entries := []struct {
		fill, stroke, label string
	}{
		{"#E8F5E9", "#388E3C", "Main module"},
		{"#E3F2FD", "#1976D2", "Same org"},
		{"#FFF3E0", "#F57C00", "External"},
		{"#FFE0E0", "#D32F2F", "Target"},
	}
	for i, e := range entries {
		ex := x + float64(i)*110
		fmt.Fprintf(b, `<rect x="%.0f" y="%.0f" width="12" height="12" rx="3" fill="%s" stroke="%s" stroke-width="1"/>`, ex, y, e.fill, e.stroke)
		fmt.Fprintf(b, `<text x="%.0f" y="%.0f" font-size="11" dominant-baseline="central" fill="#555">%s</text>`, ex+16, y+6, e.label)
	}
	fmt.Fprintln(b)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
