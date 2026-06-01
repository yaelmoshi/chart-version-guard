# chart-version-guard

`chart-version-guard` enforces the GitOps rule that Helm chart behavior changes must bump the chart version.

It watches chart-local `templates/`, `values.yaml`, `values.yml`, `values-*.yaml`, and `values-*.yml` files. If any of those files change, the same chart's top-level `Chart.yaml` `version` must change between the base and head refs.

## Usage

Check a local branch:

```sh
chart-version-guard check --base origin/main --head HEAD --repo .
```

Check from Woodpecker:

```sh
chart-version-guard check --ci woodpecker --repo .
```

Patch-bump missing chart versions locally:

```sh
chart-version-guard bump --base origin/main --head HEAD --repo . --write
```

## Rules

- Chart roots are discovered by `Chart.yaml`.
- Vendored dependency charts under a chart's `charts/` directory are ignored.
- Dependency version changes inside `Chart.yaml` do not satisfy or require a chart version bump.
- New charts pass when their new `Chart.yaml` has a top-level `version`.
- Deleted charts pass.
- `bump --write` only handles strict `x.y.z` versions.

## CI Image

Woodpecker publishes `ghcr.io/yaelmoshi/chart-version-guard` from this repository's `.woodpecker/release.yaml` pipeline.

## Repository

Forgejo is the source of truth:

- `https://git.m0sh1.cc/m0sh1/chart-version-guard`

GitHub is maintained as a public push mirror:

- `https://github.com/yaelmoshi/chart-version-guard`
