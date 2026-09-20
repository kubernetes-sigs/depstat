package cmd

import "testing"

func TestResolveRepoFromGoImport(t *testing.T) {
	body := `<html><head>
<meta name="go-import" content="k8s.io/klog git https://github.com/kubernetes/klog.git">
</head></html>`
	got := resolveRepoFromGoImport(body, "k8s.io/klog/v2")
	if got != "kubernetes/klog" {
		t.Fatalf("expected kubernetes/klog, got %q", got)
	}
}

func TestResolveRepoFromGoImportMostSpecificPrefix(t *testing.T) {
	body := `<html><head>
<meta name="go-import" content="example.com git https://github.com/acme/root">
<meta name="go-import" content="example.com/sub git https://github.com/acme/subrepo">
</head></html>`
	got := resolveRepoFromGoImport(body, "example.com/sub/pkg")
	if got != "acme/subrepo" {
		t.Fatalf("expected acme/subrepo, got %q", got)
	}
}

func TestResolveRepoFromGoImportIgnoresNonMatchingOrNonGit(t *testing.T) {
	body := `<html><head>
<meta name="go-import" content="example.com hg https://github.com/acme/wrong-vcs">
<meta name="go-import" content="other.io/mod git https://github.com/acme/other">
</head></html>`
	got := resolveRepoFromGoImport(body, "example.com/mod")
	if got != "" {
		t.Fatalf("expected empty repo, got %q", got)
	}
}

func TestExtractGitHubRepo(t *testing.T) {
	cases := []struct {
		modPath string
		want    string
	}{
		{"github.com/kubernetes/klog", "kubernetes/klog"},
		{"github.com/kubernetes/klog/v2", "kubernetes/klog"},
		{"github.com/kubernetes/klog/v10", "kubernetes/klog"},
		{"github.com/kubernetes", ""},
		{"github.com", ""},
		{"github.com/kubernetes/klog/subpkg", "kubernetes/klog"},
		{"github.com/owner/v3", ""},
	}
	for _, c := range cases {
		got := extractGitHubRepo(c.modPath)
		if got != c.want {
			t.Errorf("extractGitHubRepo(%q) = %q, want %q", c.modPath, got, c.want)
		}
	}
}

func TestIsAllDigits(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"2", true},
		{"10", true},
		{"2a", false},
		{"v2", false},
	}
	for _, c := range cases {
		got := isAllDigits(c.in)
		if got != c.want {
			t.Errorf("isAllDigits(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
