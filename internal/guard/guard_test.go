package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckFailsWhenTemplateChangesWithoutVersionBump(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "charts/app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 0.1.0\n")
	writeFile(t, repo, "charts/app/templates/deployment.yaml", "apiVersion: apps/v1\nkind: Deployment\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")

	writeFile(t, repo, "charts/app/templates/deployment.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n")
	commitAll(t, repo, "template change")

	result, err := Check(t.Context(), Options{Repo: repo, Base: base, Head: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if result.OK() {
		t.Fatal("expected guard failure")
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(result.Failures))
	}
	if result.Failures[0].Chart != "charts/app" {
		t.Fatalf("chart = %q, want charts/app", result.Failures[0].Chart)
	}
	if result.Failures[0].BaseVersion != "0.1.0" || result.Failures[0].HeadVersion != "0.1.0" {
		t.Fatalf("versions = %q -> %q, want 0.1.0 -> 0.1.0", result.Failures[0].BaseVersion, result.Failures[0].HeadVersion)
	}
}

func TestCheckFailsWhenValuesVariantChangesWithoutVersionBump(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 1.2.3\n")
	writeFile(t, repo, "app/values-prod.yaml", "replicas: 1\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")

	writeFile(t, repo, "app/values-prod.yaml", "replicas: 2\n")
	commitAll(t, repo, "values change")

	result, err := Check(t.Context(), Options{Repo: repo, Base: base, Head: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if result.OK() {
		t.Fatal("expected guard failure")
	}
}

func TestCheckPassesWhenWatchedChangeHasTopLevelVersionBump(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 1.2.3\ndependencies:\n  - name: dep\n    version: 9.9.9\n")
	writeFile(t, repo, "app/values.yaml", "replicas: 1\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")

	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 1.2.4\ndependencies:\n  - name: dep\n    version: 9.9.9\n")
	writeFile(t, repo, "app/values.yaml", "replicas: 2\n")
	commitAll(t, repo, "values and version change")

	result, err := Check(t.Context(), Options{Repo: repo, Base: base, Head: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK() {
		t.Fatalf("expected pass, got failures: %#v", result.Failures)
	}
}

func TestCheckIgnoresDependencyOnlyAndTestValuesChanges(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 1.2.3\ndependencies:\n  - name: dep\n    version: 9.9.9\n")
	writeFile(t, repo, "app/ci/test-values.yaml", "replicas: 1\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")

	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 1.2.3\ndependencies:\n  - name: dep\n    version: 9.9.10\n")
	writeFile(t, repo, "app/ci/test-values.yaml", "replicas: 2\n")
	commitAll(t, repo, "ignored changes")

	result, err := Check(t.Context(), Options{Repo: repo, Base: base, Head: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK() {
		t.Fatalf("expected pass, got failures: %#v", result.Failures)
	}
}

func TestCheckHandlesNewAndDeletedCharts(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "old/Chart.yaml", "apiVersion: v2\nname: old\nversion: 0.1.0\n")
	writeFile(t, repo, "old/templates/deployment.yaml", "kind: Deployment\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")

	removePath(t, repo, "old")
	writeFile(t, repo, "new/Chart.yaml", "apiVersion: v2\nname: new\nversion: 0.1.0\n")
	writeFile(t, repo, "new/templates/deployment.yaml", "kind: Deployment\n")
	commitAll(t, repo, "replace chart")

	result, err := Check(t.Context(), Options{Repo: repo, Base: base, Head: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK() {
		t.Fatalf("expected pass, got failures: %#v", result.Failures)
	}
}

func TestBumpPatchUpdatesOnlyMissingChartVersions(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "a/Chart.yaml", "apiVersion: v2\nname: a\nversion: 0.1.9\n")
	writeFile(t, repo, "a/values.yaml", "enabled: true\n")
	writeFile(t, repo, "b/Chart.yaml", "apiVersion: v2\nname: b\nversion: 2.0.0\n")
	writeFile(t, repo, "b/values.yaml", "enabled: true\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")

	writeFile(t, repo, "a/values.yaml", "enabled: false\n")
	writeFile(t, repo, "b/values.yaml", "enabled: false\n")
	writeFile(t, repo, "b/Chart.yaml", "apiVersion: v2\nname: b\nversion: 2.0.1\n")
	commitAll(t, repo, "changes")

	bumped, err := BumpPatch(t.Context(), Options{Repo: repo, Base: base, Head: "HEAD"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(bumped) != 1 || bumped[0].Chart != "a" || bumped[0].NewVersion != "0.1.10" {
		t.Fatalf("unexpected bumped charts: %#v", bumped)
	}
	got := readFile(t, repo, "a/Chart.yaml")
	want := "apiVersion: v2\nname: a\nversion: 0.1.10\n"
	if got != want {
		t.Fatalf("Chart.yaml = %q, want %q", got, want)
	}
}

func TestBumpPatchFailsForNonSemver(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: latest\n")
	writeFile(t, repo, "app/values.yaml", "enabled: true\n")
	commitAll(t, repo, "initial")
	base := gitRev(t, repo, "HEAD")

	writeFile(t, repo, "app/values.yaml", "enabled: false\n")
	commitAll(t, repo, "values change")

	_, err := BumpPatch(t.Context(), Options{Repo: repo, Base: base, Head: "HEAD"}, true)
	if err == nil {
		t.Fatal("expected non-semver error")
	}
}

func TestCheckStagedIndexFailsWithoutVersionBump(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 0.1.0\n")
	writeFile(t, repo, "app/values.yaml", "enabled: true\n")
	commitAll(t, repo, "initial")

	writeFile(t, repo, "app/values.yaml", "enabled: false\n")
	runGit(t, repo, "add", "app/values.yaml")

	result, err := Check(t.Context(), Options{Repo: repo, Base: "HEAD", Head: IndexRef})
	if err != nil {
		t.Fatal(err)
	}
	if result.OK() {
		t.Fatal("expected staged index failure")
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

func removePath(t *testing.T, repo, name string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(repo, filepath.FromSlash(name))); err != nil {
		t.Fatal(err)
	}
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
