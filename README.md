# coverpkg

CoverPkg uses the semi-recent `go test -coverpkg` option to gather cross-package test coverage information, then parses and aggregates it into consumable reports.

[![Coverage](https://github.com/mutility/coverpkg/actions/workflows/cover.yaml/badge.svg)](https://github.com/mutility/coverpkg/actions/workflows/cover.yaml)

## Command-line use

On the command line, use `coverpkg calc` or `coverpkg diff` to calculate and display coverage. Configuration options can be provided by flag, and sometimes by environment variable.

```bash
% go run ./cmd/coverpkg calc -g root -f ascii
github.com/mutility/coverpkg/cmd/...:        0.00%    0 of 281
github.com/mutility/coverpkg/internal/...:  37.88%  150 of 396
<all>:                                      22.16%  150 of 677
```

### Installation

`% go install github.com/mutility/coverpkg/cmd/coverpkg@latest`

## GitHub Actions

As an action, coverpkg will store coverage information in [git notes](https://git-scm.com/docs/git-notes). This requires an extra pull and push during the default `push` support, and an extra pull during the `pull_request` support. Pull requests can be commented on to reveal their state of coverage, and if base information is available, the changes. See `comment` and `token` in *Options* below.

We suggest a workflow similar to the following. In particular, coverpkg supports running on `push` and `pull_request`, and it requires go and a copy of your code checked out to what you want to test. For pull requests, it requires a token such as the `${{ github.token }}` to create and/or update comments. For pushes, it requires the ability to push back to your repository, so avoid specifying `persist-credentials: false` on your `actions/checkout@v2`. Without push support, the pull_request comment can only report the current coverage.

```yaml
name: 'Coverage'
on:
  push:
    branches:
      - 'main'
  pull_request:
    types: [opened, synchronize, reopened]

permissions:
  pull-requests: write

jobs:
  test:
    runs-on: 'ubuntu-latest'
    name: "Test code"
    steps:
    - uses: actions/checkout@v2
      with:
        ref: ${{ github.head_ref }}
    - uses: actions/setup-go@v2
      with:
        go-version: '^1.16'

    - name: Calculate Coverage
      id: coverpkg
      uses: mutility/coverpkg@v1
      with:
        comment: replace
```

### Events

The coverpkg action primarily supports `push` and `pull_request` events. In addition, it supports `pull_request_target` as an alias to `pull_request`, and `workflow_dispatch` and `repository_dispatch` act like `push`. All other events log a debug message and succeed so you don't absolutely have to filter when you invoke coverpkg.

### Options

You can specify the following inputs to coverpkg, under `with`:

Option | Default | Description
-|-|-
excludes | `gen` | Excludes packages with a folder matching any of these comma-separated names
packages | `.` | Makes sure to include the listed packages, or all if `.`
groupby | `package` | Group coverage by `file`, `package`, `root` package, or `module`
nopull | `false` | Skip pulling notes; prevents deltas from functioning
nopush | `false` | Skip pushing notes; prevents deltas from functioning
remote | `origin` | Override the git remote used for pushing and pulling
coverpkgref | `coverpkg` | Override the notes namespace used for tracking coverage
token | - | Provide to enable PR comments
comment | `none` | Set to `append`, `replace`, or `update` to create, delete, and/or update a comment on a PR

### Public forks

PRs from public forks receive a token without enough privileges to create comments on PRs. This can be worked around with additional caveats by using `pull_request_target` instead of `pull_request`, but we cannot recommend this. Coverpkg is hoping for a better solution from GitHub.

At least Dependabot dependency update PRs can be addressed by adding `permissions.pull-requests=write` as shown above, so that is now the recommended fix. See earlier revisions of this file for other approaches.
