package guard

import "testing"

func TestWoodpeckerBaseForPullRequest(t *testing.T) {
	env := map[string]string{
		"CI_PIPELINE_EVENT":       "pull_request",
		"CI_COMMIT_TARGET_BRANCH": "sm-moshi/main",
	}
	base, enforce, err := ResolveWoodpeckerBase(env)
	if err != nil {
		t.Fatal(err)
	}
	if !enforce {
		t.Fatal("expected pull request enforcement")
	}
	if base != "origin/sm-moshi/main" {
		t.Fatalf("base = %q, want origin/sm-moshi/main", base)
	}
}

func TestWoodpeckerBaseForPushPrefersPreviousSHA(t *testing.T) {
	env := map[string]string{
		"CI_PIPELINE_EVENT":  "push",
		"CI_COMMIT_PREV_SHA": "abc123",
	}
	base, enforce, err := ResolveWoodpeckerBase(env)
	if err != nil {
		t.Fatal(err)
	}
	if !enforce {
		t.Fatal("expected push enforcement")
	}
	if base != "abc123" {
		t.Fatalf("base = %q, want abc123", base)
	}
}

func TestWoodpeckerBaseForManualSkipsWithoutExplicitBase(t *testing.T) {
	env := map[string]string{
		"CI_PIPELINE_EVENT": "manual",
	}
	base, enforce, err := ResolveWoodpeckerBase(env)
	if err != nil {
		t.Fatal(err)
	}
	if enforce {
		t.Fatal("expected manual event to skip")
	}
	if base != "" {
		t.Fatalf("base = %q, want empty", base)
	}
}

func TestWoodpeckerTargetBranch(t *testing.T) {
	env := map[string]string{
		"CI_PIPELINE_EVENT":       "pull_request",
		"CI_COMMIT_TARGET_BRANCH": "main",
	}
	target, ok := WoodpeckerFetchTarget(env)
	if !ok {
		t.Fatal("expected fetch target")
	}
	if target != "main" {
		t.Fatalf("target = %q, want main", target)
	}
}
