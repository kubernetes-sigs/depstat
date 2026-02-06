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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// ArchivedDep represents an archived dependency in the output.
type ArchivedDep struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	Repo    string `json:"repo"`
	RepoURL string `json:"repoUrl"`
}

// ArchivedResult holds the complete result of the archived check.
type ArchivedResult struct {
	Archived   []ArchivedDep `json:"archived"`
	Unresolved []string      `json:"unresolved,omitempty"`
}

// goModule represents a Go module dependency from `go list -m -json`.
type goModule struct {
	Path    string `json:"Path"`
	Version string `json:"Version,omitempty"`
	Main    bool   `json:"Main,omitempty"`
}

// graphQL types for GitHub API responses.
type graphQLRequest struct {
	Query string `json:"query"`
}

type graphQLResponse struct {
	Data   map[string]*repoInfo `json:"data"`
	Errors []graphQLError       `json:"errors"`
}

type repoInfo struct {
	NameWithOwner string `json:"nameWithOwner"`
	IsArchived    bool   `json:"isArchived"`
}

type graphQLError struct {
	Message string `json:"message"`
}

var githubURLRe = regexp.MustCompile(`https?://github\.com/([^/\s]+)/([^/\s"'<>]+)`)

// knownGitHubMirrors maps vanity URL prefixes to GitHub owner/repo.
// These are domains whose go-import meta tags don't point to GitHub
// but are known to be mirrored there.
var knownGitHubMirrors = map[string]func(string) string{
	"golang.org/x/": func(mod string) string {
		parts := strings.Split(mod, "/")
		// golang.org/x/tools/go/expect -> golang/tools
		if len(parts) >= 3 {
			return "golang/" + parts[2]
		}
		return ""
	},
}

// skipPrefixes are module path prefixes known to not be on GitHub.
var skipPrefixes = []string{
	"bitbucket.org/",
}

var githubTokenPath string

var archivedCmd = &cobra.Command{
	Use:   "archived",
	Short: "Check if any Go module dependencies are archived on GitHub",
	Long: `Checks all dependencies (direct and transitive) of a Go module to determine
if any of the upstream GitHub repositories have been archived.

Resolves vanity URLs (k8s.io/*, golang.org/x/*, go.etcd.io/*, etc.) to their
actual GitHub repositories using the go-import meta tag protocol.

Uses the GitHub GraphQL API for efficient batch checking (50 repos per query).

Requires a GitHub token via --github-token-path or the GITHUB_TOKEN environment variable.`,
	RunE: runArchived,
}

// resolveGitHubToken reads the token from --github-token-path if set,
// otherwise falls back to the GITHUB_TOKEN environment variable.
func resolveGitHubToken() (string, error) {
	if githubTokenPath != "" {
		data, err := os.ReadFile(githubTokenPath)
		if err != nil {
			return "", fmt.Errorf("reading github token from %s: %w", githubTokenPath, err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("github token file %s is empty", githubTokenPath)
		}
		return token, nil
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return "", fmt.Errorf("github token is required: use --github-token-path or set GITHUB_TOKEN.\n" +
			"Create a personal access token at https://github.com/settings/tokens\n" +
			"and export it: export GITHUB_TOKEN=ghp_...")
	}
	return token, nil
}

func runArchived(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("archived does not take any arguments")
	}

	token, err := resolveGitHubToken()
	if err != nil {
		return err
	}

	// Phase 1: list all module dependencies
	modules, err := listAllModules()
	if err != nil {
		return fmt.Errorf("listing modules: %w", err)
	}

	// Separate direct github.com paths from vanity URLs
	githubRepos := make(map[string][]goModule) // owner/repo -> modules
	var vanityModules []goModule

	for _, mod := range modules {
		if mod.Main {
			continue
		}
		if strings.HasPrefix(mod.Path, "github.com/") {
			repo := extractGitHubRepo(mod.Path)
			if repo != "" {
				githubRepos[repo] = append(githubRepos[repo], mod)
			}
		} else {
			vanityModules = append(vanityModules, mod)
		}
	}

	fmt.Fprintf(os.Stderr, "  %d direct GitHub repos\n", len(githubRepos))
	fmt.Fprintf(os.Stderr, "  %d vanity/non-GitHub modules to resolve...\n", len(vanityModules))

	// Phase 2: resolve vanity URLs to GitHub repos
	resolved, unresolved := resolveVanityURLs(vanityModules)
	for repo, mods := range resolved {
		githubRepos[repo] = append(githubRepos[repo], mods...)
	}

	fmt.Fprintf(os.Stderr, "  Resolved %d vanity URLs to GitHub repos\n", len(resolved))
	if len(unresolved) > 0 {
		fmt.Fprintf(os.Stderr, "  Could not resolve %d modules (non-GitHub or unavailable)\n", len(unresolved))
		for _, u := range unresolved {
			fmt.Fprintf(os.Stderr, "    - %s\n", u)
		}
	}

	// Phase 3: batch-check archived status via GitHub GraphQL API
	repos := make([]string, 0, len(githubRepos))
	for repo := range githubRepos {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	fmt.Fprintf(os.Stderr, "\nChecking %d unique GitHub repos for archived status...\n", len(repos))
	archivedSet, warnings := checkArchivedRepos(repos, token)

	// Build output
	var archivedDeps []ArchivedDep
	for _, repo := range repos {
		if !archivedSet[repo] {
			continue
		}
		for _, mod := range githubRepos[repo] {
			archivedDeps = append(archivedDeps, ArchivedDep{
				Module:  mod.Path,
				Version: mod.Version,
				Repo:    repo,
				RepoURL: "https://github.com/" + repo,
			})
		}
	}
	sort.Slice(archivedDeps, func(i, j int) bool {
		return archivedDeps[i].Module < archivedDeps[j].Module
	})

	result := ArchivedResult{
		Archived:   archivedDeps,
		Unresolved: unresolved,
	}
	if result.Archived == nil {
		result.Archived = []ArchivedDep{}
	}

	if jsonOutput {
		return outputArchivedJSON(result)
	}
	return outputArchivedText(result, warnings)
}

func outputArchivedJSON(result ArchivedResult) error {
	outputRaw, err := json.MarshalIndent(result, "", "\t")
	if err != nil {
		return err
	}
	fmt.Println(string(outputRaw))
	return nil
}

func outputArchivedText(result ArchivedResult, warnings []string) error {
	fmt.Println()
	if len(result.Archived) > 0 {
		fmt.Printf("ARCHIVED DEPENDENCIES (%d):\n", len(result.Archived))
		currentRepo := ""
		for _, dep := range result.Archived {
			if dep.RepoURL != currentRepo {
				currentRepo = dep.RepoURL
				fmt.Printf("  %s\n", dep.RepoURL)
			}
			fmt.Printf("    <- %s %s\n", dep.Module, dep.Version)
		}
	} else {
		fmt.Println("No archived dependencies found.")
	}

	if len(warnings) > 0 {
		fmt.Printf("\nWARNINGS (%d):\n", len(warnings))
		for _, w := range warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
	return nil
}

// listAllModules runs `go list -m -json all` in the configured directory
// and returns parsed module info.
func listAllModules() ([]goModule, error) {
	goListCmd := exec.Command("go", "list", "-m", "-json", "all")
	if dir != "" {
		goListCmd.Dir = dir
	}
	goListCmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")

	var stdout, stderr bytes.Buffer
	goListCmd.Stdout = &stdout
	goListCmd.Stderr = &stderr

	if err := goListCmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, stderr.String())
	}

	var modules []goModule
	dec := json.NewDecoder(&stdout)
	for {
		var mod goModule
		if err := dec.Decode(&mod); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("parsing go list output: %v", err)
		}
		modules = append(modules, mod)
	}
	return modules, nil
}

// extractGitHubRepo extracts "owner/repo" from a github.com module path.
func extractGitHubRepo(modPath string) string {
	parts := strings.Split(modPath, "/")
	if len(parts) < 3 {
		return ""
	}
	owner := parts[1]
	repo := parts[2]

	// If repo is a version suffix (v2, v3, ...), skip
	if len(repo) > 1 && repo[0] == 'v' && isAllDigits(repo[1:]) {
		return ""
	}
	return owner + "/" + repo
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// resolveVanityURLs resolves non-github.com module paths to GitHub repos
// using the go-import meta tag protocol.
func resolveVanityURLs(mods []goModule) (resolved map[string][]goModule, unresolved []string) {
	resolved = make(map[string][]goModule)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, 20)
	client := &http.Client{Timeout: 10 * time.Second}

	for _, mod := range mods {
		wg.Add(1)
		go func(m goModule) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repo := resolveOneVanityURL(client, m.Path)
			mu.Lock()
			defer mu.Unlock()
			if repo != "" {
				resolved[repo] = append(resolved[repo], m)
			} else {
				unresolved = append(unresolved, m.Path)
			}
		}(mod)
	}
	wg.Wait()

	sort.Strings(unresolved)
	return resolved, unresolved
}

// resolveOneVanityURL resolves a single module path to a GitHub owner/repo.
func resolveOneVanityURL(client *http.Client, modPath string) string {
	// Check known mirrors first
	for prefix, resolver := range knownGitHubMirrors {
		if strings.HasPrefix(modPath, prefix) {
			return resolver(modPath)
		}
	}

	// Skip known non-GitHub domains
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(modPath, prefix) {
			return ""
		}
	}

	// Fetch the go-import meta tag
	fetchURL := "https://" + modPath + "?go-get=1"
	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Go-http-client/1.1")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return ""
	}

	// Collapse whitespace to handle multiline meta tags
	collapsed := regexp.MustCompile(`\s+`).ReplaceAllString(string(body), " ")

	match := githubURLRe.FindStringSubmatch(collapsed)
	if match == nil {
		return ""
	}

	owner := match[1]
	repo := match[2]
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.TrimRight(repo, `"'>`)
	return owner + "/" + repo
}

// checkArchivedRepos uses the GitHub GraphQL API to batch-check repos for
// archived status.
func checkArchivedRepos(repos []string, token string) (archivedSet map[string]bool, warnings []string) {
	archivedSet = make(map[string]bool)
	const batchSize = 50

	for i := 0; i < len(repos); i += batchSize {
		end := i + batchSize
		if end > len(repos) {
			end = len(repos)
		}
		batch := repos[i:end]

		archived, warn := graphQLBatchCheck(batch, token)
		for _, repo := range archived {
			archivedSet[repo] = true
		}
		warnings = append(warnings, warn...)

		fmt.Fprintf(os.Stderr, "  Checked %d/%d repos...\n", end, len(repos))
	}
	return archivedSet, warnings
}

// graphQLBatchCheck checks a batch of repos via a single GraphQL query.
// It maps results back using the query alias index so that renamed repos
// (e.g. flynn/go-shlex -> flynn-archive/go-shlex) are correctly associated.
func graphQLBatchCheck(repos []string, token string) (archived []string, warnings []string) {
	var query strings.Builder
	query.WriteString("{\n")
	for idx, repo := range repos {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 {
			continue
		}
		owner, name := parts[0], parts[1]
		alias := fmt.Sprintf("r%d", idx)
		fmt.Fprintf(&query, "  %s: repository(owner: %q, name: %q) {\n    nameWithOwner\n    isArchived\n  }\n", alias, owner, name)
	}
	query.WriteString("}\n")

	reqBody := graphQLRequest{Query: query.String()}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewReader(bodyBytes))
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to create request: %v", err))
		return nil, warnings
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("GraphQL request failed: %v", err))
		return nil, warnings
	}
	defer resp.Body.Close()

	var result graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to decode GraphQL response: %v", err))
		return nil, warnings
	}

	for _, e := range result.Errors {
		warnings = append(warnings, e.Message)
	}

	// Map results back via alias index to the original repo name
	for idx, repo := range repos {
		alias := fmt.Sprintf("r%d", idx)
		info, ok := result.Data[alias]
		if !ok || info == nil {
			warnings = append(warnings, fmt.Sprintf("Could not query: %s (deleted/renamed/private?)", repo))
			continue
		}
		if info.IsArchived {
			archived = append(archived, repo)
		}
	}
	return archived, warnings
}

func init() {
	rootCmd.AddCommand(archivedCmd)
	archivedCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate. Defaults to the current directory.")
	archivedCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Get the output in JSON format")
	archivedCmd.Flags().StringVar(&githubTokenPath, "github-token-path", "", "Path to a file containing the GitHub API token. If not set, uses GITHUB_TOKEN env var.")
}
