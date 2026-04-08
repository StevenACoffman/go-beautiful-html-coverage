<!-- markdownlint-disable MD041 MD033 -->
# `go-beautiful-html-coverage`

A GitHub Action and Go CLI to track code coverage in pull requests, with a
beautiful HTML preview, for free. Ported to Go from the original
[gha-common/go-beautiful-html-coverage](https://github.com/gha-common/go-beautiful-html-coverage/)
by [@kilianc](https://github.com/kilianc) who made
[a beautiful HTML preview ↗](https://kilianc.github.io/pretender/head/head.html#file0).

<a href="https://www.buymeacoffee.com/kilianciuffolo" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" ></a>

Note: If you like the original project, buy [@kilianc](https://github.com/kilianc) a coffee above.

## How it works

The tool expects a `cover.out` coverage profile, produced by:

```sh
go test -coverprofile=cover.out ./...
```

When triggered (via the action or the CLI), it:

1. Builds the Go binary from `$GITHUB_ACTION_PATH` (action) or runs the
   pre-installed binary (CLI).
2. Checks out or creates a dedicated **coverage branch** (default: `cover`) as a
   separate clone at `go-cover/` in the workspace.
3. Runs `go tool cover` to produce an HTML report and a text summary for the
   current revision SHA.
4. Computes **incremental coverage**: lines covered in this revision that were
   *not* already covered on `main`, producing a second pair of `{sha}-inc.*`
   reports.
5. Writes the embedded static assets (`index.css`, `index.js`, `index.html`)
   into the coverage branch root and injects a versioned `<script>` tag into
   each HTML report.
6. Commits and pushes to the coverage branch, triggering a GitHub Pages
   deployment.
7. On pull requests: posts (or updates) a PR comment with the text summary and
   a link to the HTML report.
8. If a threshold is configured: exits non-zero when total coverage falls below
   it.

### Coverage branch layout

```
{path}/
  revisions/
    {sha}.html          full coverage HTML report
    {sha}.txt           full coverage text summary
    {sha}.out           full coverage profile
    {sha}-inc.html      incremental coverage HTML report
    {sha}-inc.txt       incremental coverage text summary
    {sha}-inc.out       incremental coverage profile
  head/
    head.html           latest main-branch HTML report
    head.txt            latest main-branch text summary
    head.out            latest main-branch profile (diff base for incremental)
  index.html            waiting page — redirects to ?hash=<sha>
  index.css             shared styles (dark/light theme, coverage highlights)
  index.js              shared scripts (syntax highlight, theme toggle, line numbers)
```

`{path}` is empty for single-module repos. For monorepos it equals the
normalized module subdirectory (see `path` input / `--path` flag).

### Screenshots

<br>
<img alt="PR Comment" src="https://github.com/StevenACoffman/go-beautiful-html-coverage/assets/385716/e155c0aa-14ec-4740-9824-f00399e6b170">
<img alt="HTML Preview (Dark)" src="https://github.com/StevenACoffman/go-beautiful-html-coverage/assets/385716/154f0af6-f5a9-4eb5-bc3a-721bab2e4263">
<img alt="HTML Preview (Light)" src="https://github.com/StevenACoffman/go-beautiful-html-coverage/assets/385716/11256803-59c5-45c4-8ad0-e83ac3374388">
<br><br>

---

## GitHub Actions usage

> [!IMPORTANT]
> The action builds the Go binary from source as its first step, so your
> workflow must include `actions/setup-go` **before** this action.

> [!NOTE]
> Configure **GitHub Pages** in your repository (*Settings → Pages*) to deploy
> from the `cover` branch (or whichever branch you set with `branch:`).
> Without this the HTML links in PR comments will 404.
>
> ![GitHub Pages Setup](https://github.com/StevenACoffman/go-beautiful-html-coverage/assets/385716/a14f4df6-6263-4ae3-8685-e7901a1dbbe2)

### Minimal example

```yaml
name: Go

on:
  push:
    branches: ["main"]
  pull_request:
    branches: ["main"]

jobs:
  test:
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write   # required: post PR comments
      contents: write        # required: push to coverage branch
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5   # required: action builds the binary

      - name: Test
        run: go test -coverprofile=cover.out ./...

      - uses: StevenACoffman/go-beautiful-html-coverage@v1
```

### Action inputs

| Input | Default | Description |
|---|---|---|
| `repository` | `${{ github.repository }}` | Owner/repo that hosts the coverage branch. Override to centralise coverage across repos. |
| `branch` | `cover` | Branch to create or push coverage files to. Must match the branch set in GitHub Pages. |
| `token` | `${{ github.token }}` | Token used for git push and PR comments. Needs `contents:write` on the coverage repo and `pull-requests:write` on the tested repo. |
| `path` | `./` | Relative path to the Go module. `cover.out` must exist here after the test step. Useful for monorepos. |
| `threshold` | `0` | Minimum coverage percentage. The action fails when coverage falls below this value. `0` disables the check. |

### Examples

**Custom branch name**

```yaml
- uses: StevenACoffman/go-beautiful-html-coverage@v1
  with:
    branch: my-coverage
```

Update the GitHub Pages deployment setting to match.

**Enforce a coverage threshold**

```yaml
- uses: StevenACoffman/go-beautiful-html-coverage@v1
  with:
    threshold: 80
```

The action step fails when total coverage is below `threshold`, blocking merge.

**Centralised coverage in a separate repo**

```yaml
- uses: StevenACoffman/go-beautiful-html-coverage@v1
  with:
    repository: yourorg/coverage
    token: ${{ secrets.COVERAGE_TOKEN }}
```

`COVERAGE_TOKEN` must have `contents:write` on `yourorg/coverage`. The PR
comment is always posted to the current repo's pull request regardless of
where the coverage files are stored.

**Monorepo with multiple modules**

Each module needs its own test step that writes `cover.out` into that module's
directory, then a separate action step with the matching `path:`.

```yaml
- name: Test go-app-01
  working-directory: go-app-01
  run: go test -coverprofile=cover.out ./...

- uses: StevenACoffman/go-beautiful-html-coverage@v1
  with:
    path: ./go-app-01

- name: Test go-app-02
  working-directory: go-app-02
  run: go test -coverprofile=cover.out ./...

- uses: StevenACoffman/go-beautiful-html-coverage@v1
  with:
    path: ./go-app-02
```

---

## CLI usage

Every action step is also a standalone CLI subcommand. Use this to run the
full workflow locally, inspect intermediate outputs, or integrate with a CI
system other than GitHub Actions.

### Install

```sh
go install github.com/StevenACoffman/go-beautiful-html-coverage@latest
```

Or build from source:

```sh
git clone https://github.com/StevenACoffman/go-beautiful-html-coverage
cd go-beautiful-html-coverage
go build -o go-beautiful-html-coverage .
```

### Manual end-to-end walkthrough

Run the following from your workspace root — the directory that contains your
Go module and `cover.out`. Adjust the shell expressions for your environment.

**1. Prepare the coverage branch**

Clone the repo that will hold coverage files into `go-cover/` (this can be the
same repo or a dedicated one). Then fetch or create the coverage branch:

```sh
git clone https://github.com/yourorg/yourrepo go-cover

go-beautiful-html-coverage pull \
  --dir    go-cover \
  --branch cover \
  --path   ""
```

`pull` checks whether `origin/cover` exists. If it does, it checks out and
pulls it. If not, it creates an orphan branch and writes empty
`{path}/head/head.{html,txt,out}` placeholder files so the first `push` has a
valid incremental diff base.

**2. Generate and push coverage files**

```sh
go test -coverprofile=cover.out ./...

go-beautiful-html-coverage push \
  --workspace "$(pwd)" \
  --revision  "$(git rev-parse HEAD)" \
  --branch    cover \
  --ref-name  "$(git branch --show-current)" \
  --path      ""
```

`push` reads `cover.out` from `{workspace}/{path}/cover.out`, generates full
and incremental HTML/text reports via `go tool cover`, writes the embedded
assets into `go-cover/{path}/`, beautifies the HTML files, and commits and
pushes to `go-cover/`.

When `--ref-name` is `main` the generated files are also copied to
`{path}/head/`, updating the baseline used for future incremental diffs.

**3. Post a PR comment** *(pull requests only)*

```sh
go-beautiful-html-coverage comment \
  --owner        yourorg \
  --repo         yourrepo \
  --issue-number 42 \
  --revision     "$(git rev-parse HEAD)" \
  --token        "$GITHUB_TOKEN" \
  --threshold    80 \
  --path         ""
```

`comment` reads `go-cover/{path}/revisions/{sha}.txt`, builds a Markdown
comment body, and creates or updates the PR comment identified by the marker
`<!-- coverage ({path})-->` at its first line.

**4. Check the coverage threshold** *(pull requests only)*

```sh
go-beautiful-html-coverage check-threshold \
  --revision  "$(git rev-parse HEAD)" \
  --threshold 80 \
  --path      ""
```

Reads `go-cover/{path}/revisions/{sha}.txt`. Exits 0 when coverage meets or
exceeds the threshold; exits 1 with a red `✘` message otherwise.

---

### Subcommand reference

#### `pull`

Fetches the coverage branch from origin. Checks it out and pulls if it
exists; creates an orphan branch with empty `head/` placeholder files if it
does not.

```
go-beautiful-html-coverage pull [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--dir` | `go-cover` | Path to the coverage repository checkout. |
| `--branch` | `cover` | Coverage branch name. |
| `--path` | `""` | Module sub-path used when writing the initial `head/` structure on a new branch (monorepos). |

#### `push`

Runs `go tool cover` on `cover.out` to produce HTML and text reports, computes
incremental coverage against the head baseline, writes the embedded static
assets, beautifies the HTML files, and commits and pushes to the coverage branch.

```
go-beautiful-html-coverage push --revision <sha> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--revision` | *(required)* | Git revision SHA. Used as the filename stem for all generated files. |
| `--branch` | `cover` | Coverage branch to push to. |
| `--ref-name` | `""` | Current branch name. When `main`, head files are updated after push. |
| `--path` | `""` | Module sub-path within the workspace (monorepos). |
| `--workspace` | `$GITHUB_WORKSPACE`, then cwd | Workspace root. `cover.out` is read from `{workspace}/{path}/cover.out`. The coverage checkout is expected at `{workspace}/go-cover/`. |

#### `comment`

Creates or updates a PR comment containing the coverage summary and a link to
the HTML report on GitHub Pages. The comment is keyed on the marker
`<!-- coverage ({path})-->` so re-runs update rather than duplicate it.

```
go-beautiful-html-coverage comment \
  --owner <owner> --repo <repo> --issue-number <n> \
  --revision <sha> --token <tok> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--owner` | *(required)* | GitHub repository owner of the PR. |
| `--repo` | *(required)* | GitHub repository name of the PR. |
| `--issue-number` | *(required)* | Pull request number. |
| `--revision` | *(required)* | Git revision SHA. |
| `--token` | *(required)* | GitHub API token with `pull-requests:write`. |
| `--threshold` | `0` | Minimum coverage percentage. Shown in the comment body when non-zero. |
| `--path` | `""` | Normalized module sub-path (monorepos). |

Reads `go-cover/{path}/revisions/{revision}.txt` relative to the current
working directory.

#### `check-threshold`

Reads the total coverage percentage from the text summary and exits non-zero
if it is below `--threshold`.

```
go-beautiful-html-coverage check-threshold --revision <sha> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--revision` | *(required)* | Git revision SHA. |
| `--threshold` | `0` | Minimum coverage percentage. `0` always passes. |
| `--path` | `""` | Normalized module sub-path (monorepos). |

Reads `go-cover/{path}/revisions/{revision}.txt` relative to the current
working directory. Prints a green `✔` or red `✘` line with the actual
percentage.

#### `beautify`

Post-processes the HTML files produced by `go tool cover`. Strips the inline
`<style>` and `<script>` blocks, adds cache-control meta tags, and injects
`<script src="../index.js?{hash}">` where `{hash}` is the MD5 of the embedded
`index.css` and `index.js` bytes for cache busting.

Called automatically by `push`. Use it standalone to reprocess an HTML file
without re-running the full push pipeline.

```
go-beautiful-html-coverage beautify --revision <sha> [--dir <dir>]
```

| Flag | Default | Description |
|---|---|---|
| `--revision` | *(required)* | Git revision SHA. Modifies `{dir}/revisions/{sha}.html` and `{dir}/revisions/{sha}-inc.html` in place. |
| `--dir` | `.` | Coverage directory containing the `revisions/` subdirectory. |

#### `normalize-path`

Normalizes a directory path to the canonical sub-path form used by the other
subcommands: resolves `.` and `..`, strips leading `./` and `/`, strips
trailing `/`. Returns an empty string for root-equivalent inputs (`""`, `"."`,
`"/"`, `"./"`).

```
go-beautiful-html-coverage normalize-path [<path>]
```

Useful when assembling `--path` values in shell scripts.

#### `version`

Prints the binary version and exits.

```
go-beautiful-html-coverage version
```

---

## License

MIT License, see [LICENSE](./LICENSE.md)
