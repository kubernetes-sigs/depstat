package cmd

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// perform depth first search from current dependency
func dfs(k string, graph map[string][]string, dp map[string]int, visited map[string]bool, longestPath map[string]string) {
	visited[k] = true
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
