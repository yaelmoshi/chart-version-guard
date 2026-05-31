package guard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const IndexRef = ":index"

type Options struct {
	Repo string
	Base string
	Head string
}

type Result struct {
	Failures []Failure `json:"failures"`
}

type Failure struct {
	Chart       string   `json:"chart"`
	BaseVersion string   `json:"baseVersion"`
	HeadVersion string   `json:"headVersion"`
	Files       []string `json:"files"`
}

type BumpedChart struct {
	Chart      string
	OldVersion string
	NewVersion string
}

func (r Result) OK() bool {
	return len(r.Failures) == 0
}

func Check(ctx context.Context, opts Options) (Result, error) {
	opts = normalizeOptions(opts)
	if opts.Base == "" || opts.Head == "" {
		return Result{}, errors.New("base and head refs are required")
	}

	roots, err := chartRoots(ctx, opts)
	if err != nil {
		return Result{}, err
	}

	changes, err := changedFiles(ctx, opts)
	if err != nil {
		return Result{}, err
	}

	byChart := map[string][]string{}
	for _, change := range changes {
		chart, ok := owningChart(change.Path, roots)
		if !ok || !isWatchedChartPath(chart, change.Path) {
			continue
		}
		byChart[chart] = append(byChart[chart], change.Path)
	}

	var result Result
	for chart, files := range byChart {
		baseVersion, baseOK, err := chartVersionAt(ctx, opts.Repo, opts.Base, chart)
		if err != nil {
			return Result{}, err
		}
		headVersion, headOK, err := chartVersionAt(ctx, opts.Repo, opts.Head, chart)
		if err != nil {
			return Result{}, err
		}

		switch {
		case !baseOK && headOK && headVersion != "":
			continue
		case baseOK && !headOK:
			continue
		case baseOK && headOK && baseVersion != "" && baseVersion != headVersion:
			continue
		}

		slices.Sort(files)
		result.Failures = append(result.Failures, Failure{
			Chart:       chart,
			BaseVersion: baseVersion,
			HeadVersion: headVersion,
			Files:       slices.Compact(files),
		})
	}

	slices.SortFunc(result.Failures, func(a, b Failure) int {
		return strings.Compare(a.Chart, b.Chart)
	})
	return result, nil
}

func BumpPatch(ctx context.Context, opts Options, write bool) ([]BumpedChart, error) {
	opts = normalizeOptions(opts)
	result, err := Check(ctx, opts)
	if err != nil {
		return nil, err
	}

	var bumped []BumpedChart
	for _, failure := range result.Failures {
		next, err := bumpPatchVersion(failure.HeadVersion)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", displayChart(failure.Chart), err)
		}
		if write {
			if err := rewriteChartVersion(filepath.Join(opts.Repo, filepath.FromSlash(chartFile(failure.Chart))), failure.HeadVersion, next); err != nil {
				return nil, err
			}
		}
		bumped = append(bumped, BumpedChart{
			Chart:      failure.Chart,
			OldVersion: failure.HeadVersion,
			NewVersion: next,
		})
	}
	return bumped, nil
}

type fileChange struct {
	Path string
}

func normalizeOptions(opts Options) Options {
	if opts.Repo == "" {
		opts.Repo = "."
	}
	if opts.Head == "" {
		opts.Head = "HEAD"
	}
	return opts
}

func changedFiles(ctx context.Context, opts Options) ([]fileChange, error) {
	args := []string{"diff", "--name-status", "-M"}
	if opts.Head == IndexRef {
		args = append(args, "--cached", opts.Base, "--")
	} else {
		args = append(args, opts.Base, opts.Head, "--")
	}
	out, err := git(ctx, opts.Repo, args...)
	if err != nil {
		return nil, err
	}
	var changes []fileChange
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		path := fields[len(fields)-1]
		changes = append(changes, fileChange{Path: filepath.ToSlash(path)})
	}
	return changes, nil
}

func chartRoots(ctx context.Context, opts Options) ([]string, error) {
	seen := map[string]bool{}
	for _, ref := range []string{opts.Base, opts.Head} {
		if ref == "" {
			continue
		}
		var out string
		var err error
		if ref == IndexRef {
			out, err = git(ctx, opts.Repo, "ls-files")
		} else {
			out, err = git(ctx, opts.Repo, "ls-tree", "-r", "--name-only", ref, "--")
		}
		if err != nil {
			return nil, err
		}
		for _, path := range strings.Split(strings.TrimSpace(out), "\n") {
			if !strings.HasSuffix(path, "/Chart.yaml") && path != "Chart.yaml" {
				continue
			}
			root := strings.TrimSuffix(path, "/Chart.yaml")
			if path == "Chart.yaml" {
				root = ""
			}
			if isInfrastructureIgnored(path) {
				continue
			}
			seen[root] = true
		}
	}

	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	slices.SortFunc(roots, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return strings.Compare(a, b)
	})

	filtered := roots[:0]
	for _, root := range roots {
		if isVendoredDependencyChart(root, roots) {
			continue
		}
		filtered = append(filtered, root)
	}
	return filtered, nil
}

func owningChart(path string, roots []string) (string, bool) {
	for _, root := range roots {
		if root == "" {
			return "", true
		}
		if path == root || strings.HasPrefix(path, root+"/") {
			return root, true
		}
	}
	return "", false
}

func isWatchedChartPath(chart, path string) bool {
	rel := path
	if chart != "" {
		rel = strings.TrimPrefix(path, chart+"/")
	}
	if rel == path && chart != "" {
		return false
	}
	if strings.HasPrefix(rel, "charts/") {
		return false
	}
	if strings.HasPrefix(rel, "templates/") {
		return true
	}
	base := filepath.Base(rel)
	if strings.Contains(rel, "/") {
		return false
	}
	if base == "values.yaml" || base == "values.yml" {
		return true
	}
	return strings.HasPrefix(base, "values-") && (strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml"))
}

func isInfrastructureIgnored(path string) bool {
	return strings.HasPrefix(path, ".git/") ||
		strings.HasPrefix(path, "node_modules/") ||
		strings.HasPrefix(path, "vendor/") ||
		strings.Contains(path, "/node_modules/") ||
		strings.Contains(path, "/vendor/") ||
		strings.Contains(path, "/.ci/rendered/")
}

func isVendoredDependencyChart(root string, roots []string) bool {
	for _, parent := range roots {
		if parent == root {
			continue
		}
		prefix := "charts/"
		if parent != "" {
			prefix = parent + "/charts/"
		}
		if strings.HasPrefix(root, prefix) {
			return true
		}
	}
	return false
}

func chartVersionAt(ctx context.Context, repo, ref, chart string) (string, bool, error) {
	spec := ref + ":" + chartFile(chart)
	if ref == IndexRef {
		spec = ":" + chartFile(chart)
	}
	out, err := git(ctx, repo, "show", spec)
	if err != nil {
		if strings.Contains(err.Error(), "exists on disk, but not in") ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "Path ") {
			return "", false, nil
		}
		return "", false, err
	}
	version, err := topLevelVersion([]byte(out))
	if err != nil {
		return "", true, fmt.Errorf("%s at %s: %w", chartFile(chart), ref, err)
	}
	return version, true, nil
}

func topLevelVersion(content []byte) (string, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(content, &node); err != nil {
		return "", err
	}
	if len(node.Content) == 0 || node.Content[0].Kind != yaml.MappingNode {
		return "", nil
	}
	mapping := node.Content[0]
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "version" {
			return mapping.Content[i+1].Value, nil
		}
	}
	return "", nil
}

func chartFile(chart string) string {
	if chart == "" {
		return "Chart.yaml"
	}
	return chart + "/Chart.yaml"
}

func displayChart(chart string) string {
	if chart == "" {
		return "."
	}
	return chart
}

var semverPatch = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)$`)

func bumpPatchVersion(version string) (string, error) {
	matches := semverPatch.FindStringSubmatch(version)
	if matches == nil {
		return "", fmt.Errorf("chart version %q is not strict x.y.z semver", version)
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s.%d", matches[1], matches[2], patch+1), nil
}

func rewriteChartVersion(path, oldVersion, newVersion string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.SplitAfter(content, []byte("\n"))
	for i, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "version:") {
			prefixLen := len(line) - len(bytes.TrimLeft(line, " \t"))
			lineEnding := []byte{}
			if bytes.HasSuffix(line, []byte("\n")) {
				lineEnding = []byte("\n")
			}
			lines[i] = append(bytes.Repeat([]byte(" "), prefixLen), []byte("version: "+newVersion)...)
			lines[i] = append(lines[i], lineEnding...)
			return os.WriteFile(path, bytes.Join(lines, nil), 0o644)
		}
	}
	return fmt.Errorf("%s: top-level version %q not found", path, oldVersion)
}

func git(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
