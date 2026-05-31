package guard

import (
	"errors"
	"strings"
)

func ResolveWoodpeckerBase(env map[string]string) (base string, enforce bool, err error) {
	switch env["CI_PIPELINE_EVENT"] {
	case "pull_request":
		target := env["CI_COMMIT_TARGET_BRANCH"]
		if target == "" {
			target = env["CI_REPO_DEFAULT_BRANCH"]
		}
		if target == "" {
			return "", false, errors.New("pull_request event is missing CI_COMMIT_TARGET_BRANCH and CI_REPO_DEFAULT_BRANCH")
		}
		return "origin/" + target, true, nil
	case "push":
		prev := env["CI_COMMIT_PREV_SHA"]
		if prev != "" && !allZeroSHA(prev) {
			return prev, true, nil
		}
		if target := env["CI_REPO_DEFAULT_BRANCH"]; target != "" {
			return "origin/" + target, true, nil
		}
		return "HEAD~1", true, nil
	default:
		return "", false, nil
	}
}

func WoodpeckerFetchTarget(env map[string]string) (string, bool) {
	if env["CI_PIPELINE_EVENT"] != "pull_request" && env["CI_PIPELINE_EVENT"] != "push" {
		return "", false
	}
	target := env["CI_COMMIT_TARGET_BRANCH"]
	if target == "" {
		target = env["CI_REPO_DEFAULT_BRANCH"]
	}
	return target, target != ""
}

func allZeroSHA(s string) bool {
	return strings.Trim(s, "0") == ""
}
