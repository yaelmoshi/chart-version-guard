package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCommandReportsFailures(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 0.1.0\n")
	writeFile(t, repo, "app/values.yaml", "enabled: true\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")
	writeFile(t, repo, "app/values.yaml", "enabled: false\n")
	commitAll(t, repo, "values change")

	var stdout, stderr bytes.Buffer
	code := Run(t.Context(), []string{"check", "--repo", repo, "--base", base, "--head", "HEAD"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "app") || !strings.Contains(stdout.String(), "app/values.yaml") {
		t.Fatalf("stdout did not describe failure: %q", stdout.String())
	}
}

func TestBumpCommandWritesPatchBump(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 0.1.0\n")
	writeFile(t, repo, "app/values.yaml", "enabled: true\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")
	writeFile(t, repo, "app/values.yaml", "enabled: false\n")
	commitAll(t, repo, "values change")

	var stdout, stderr bytes.Buffer
	code := Run(t.Context(), []string{"bump", "--repo", repo, "--base", base, "--head", "HEAD", "--write"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	got := readFile(t, repo, "app/Chart.yaml")
	if !strings.Contains(got, "version: 0.1.1") {
		t.Fatalf("Chart.yaml was not bumped: %q", got)
	}
}

func TestBumpCommandSupportsWoodpeckerCI(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 0.1.0\n")
	writeFile(t, repo, "app/values.yaml", "enabled: true\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")
	writeFile(t, repo, "app/values.yaml", "enabled: false\n")
	commitAll(t, repo, "values change")

	env := map[string]string{
		"CI_PIPELINE_EVENT":  "push",
		"CI_COMMIT_PREV_SHA": base,
	}
	getenv := func(key string) string {
		return env[key]
	}

	var stdout, stderr bytes.Buffer
	code := Run(t.Context(), []string{"bump", "--ci", "woodpecker", "--repo", repo, "--write"}, getenv, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	got := readFile(t, repo, "app/Chart.yaml")
	if !strings.Contains(got, "version: 0.1.1") {
		t.Fatalf("Chart.yaml was not bumped: %q", got)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	return repo
}

func writeFile(t *testing.T, repo, name, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, repo, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func commitAll(t *testing.T, repo, message string) {
	t.Helper()
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", message)
}

func gitRev(t *testing.T, repo, rev string) string {
	t.Helper()
	return runGit(t, repo, "rev-parse", rev)
}

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
