package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"git.m0sh1.cc/m0sh1/chart-version-guard/internal/guard"
)

type getenvFunc func(string) string

func Run(ctx context.Context, args []string, getenv getenvFunc, stdout, stderr io.Writer) int {
	if getenv == nil {
		getenv = os.Getenv
	}
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "check":
		return runCheck(ctx, args[1:], getenv, stdout, stderr)
	case "bump":
		return runBump(ctx, args[1:], getenv, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func runCheck(ctx context.Context, args []string, getenv getenvFunc, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts guard.Options
	var ci, format string
	var staged bool
	fs.StringVar(&opts.Repo, "repo", ".", "repository path")
	fs.StringVar(&opts.Base, "base", "", "base git ref")
	fs.StringVar(&opts.Head, "head", "HEAD", "head git ref")
	fs.StringVar(&ci, "ci", "", "CI environment resolver, supported: woodpecker")
	fs.StringVar(&format, "format", "text", "output format: text or json")
	fs.BoolVar(&staged, "staged", false, "compare HEAD against the staged index")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if staged {
		if opts.Base == "" {
			opts.Base = "HEAD"
		}
		opts.Head = guard.IndexRef
	}

	if ci != "" {
		var enforce bool
		var err error
		opts, enforce, err = resolveCIOptions(ctx, opts, ci, getenv)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if !enforce {
			fmt.Fprintln(stdout, "chart version guard skipped for this CI event")
			return 0
		}
	}

	result, err := guard.Check(ctx, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := printResult(stdout, result, format); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !result.OK() {
		return 1
	}
	return 0
}

func runBump(ctx context.Context, args []string, getenv getenvFunc, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bump", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts guard.Options
	var ci string
	var write bool
	fs.StringVar(&opts.Repo, "repo", ".", "repository path")
	fs.StringVar(&opts.Base, "base", "", "base git ref")
	fs.StringVar(&opts.Head, "head", "HEAD", "head git ref")
	fs.StringVar(&ci, "ci", "", "CI environment resolver, supported: woodpecker")
	fs.BoolVar(&write, "write", false, "write patch bumps to Chart.yaml files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if ci != "" {
		var enforce bool
		var err error
		opts, enforce, err = resolveCIOptions(ctx, opts, ci, getenv)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if !enforce {
			fmt.Fprintln(stdout, "chart version guard skipped for this CI event")
			return 0
		}
	}
	bumped, err := guard.BumpPatch(ctx, opts, write)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if len(bumped) == 0 {
		fmt.Fprintln(stdout, "no chart versions need bumping")
		return 0
	}
	for _, chart := range bumped {
		fmt.Fprintf(stdout, "%s: %s -> %s\n", displayChart(chart.Chart), chart.OldVersion, chart.NewVersion)
	}
	return 0
}

func printResult(w io.Writer, result guard.Result, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "text":
		if result.OK() {
			_, err := fmt.Fprintln(w, "all changed Helm chart behavior has a chart version bump")
			return err
		}
		for _, failure := range result.Failures {
			fmt.Fprintf(w, "%s: watched chart files changed without a top-level Chart.yaml version bump (%q -> %q)\n", displayChart(failure.Chart), failure.BaseVersion, failure.HeadVersion)
			for _, file := range failure.Files {
				fmt.Fprintf(w, "  - %s\n", file)
			}
			fmt.Fprintf(w, "  suggested: chart-version-guard bump --base <base> --head HEAD --repo . --write\n")
		}
		return nil
	default:
		return fmt.Errorf("unsupported --format value %q", format)
	}
}

func envMap(getenv getenvFunc) map[string]string {
	keys := []string{
		"CI_PIPELINE_EVENT",
		"CI_COMMIT_TARGET_BRANCH",
		"CI_REPO_DEFAULT_BRANCH",
		"CI_COMMIT_PREV_SHA",
	}
	env := make(map[string]string, len(keys))
	for _, key := range keys {
		env[key] = getenv(key)
	}
	return env
}

func resolveCIOptions(ctx context.Context, opts guard.Options, ci string, getenv getenvFunc) (guard.Options, bool, error) {
	if ci != "woodpecker" {
		return opts, false, fmt.Errorf("unsupported --ci value %q", ci)
	}
	env := envMap(getenv)
	if target, ok := guard.WoodpeckerFetchTarget(env); ok {
		if err := fetchTarget(ctx, opts.Repo, target); err != nil {
			return opts, false, err
		}
	}
	base, enforce, err := guard.ResolveWoodpeckerBase(env)
	if err != nil {
		return opts, false, err
	}
	if enforce {
		opts.Base = base
	}
	return opts, enforce, nil
}

func displayChart(chart string) string {
	if chart == "" {
		return "."
	}
	return chart
}

func fetchTarget(ctx context.Context, repo, target string) error {
	return runGitCommand(ctx, repo, "fetch", "--no-tags", "origin", target)
}

func runGitCommand(ctx context.Context, repo string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: chart-version-guard <check|bump> [options]")
}
