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
