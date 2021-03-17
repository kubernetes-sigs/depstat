package cmd

func max(x, y int) int {
	if x <= y {
		return y
	}
	return x
}

// perform depth first search from current dependency
func dfs(k string, graph map[string][]string, dp map[string]int, visited map[string]bool, longestPath map[string]string) {
	visited[k] = true
	// for terminal deps we won't go into this for loop
	// and so vis for them would be true and dp would be 0
	// since in maps non existent keys have 0 value by default
	for _, u := range graph[k] {
		if visited[u] == false {
			dfs(u, graph, dp, visited, longestPath)
		}
		dp[k] = max(dp[k], 1+dp[u])
		if dp[k] == 1+dp[u] {
			longestPath[k] = u
		}
	}
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
