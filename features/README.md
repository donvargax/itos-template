# Feature files

Every `feat:` and `fix:` commit is driven by scenarios in this folder. Nothing
else belongs here. The harness is harvested from itos's
(github.com/donvargax/itos, `features/`).

## What goes in a feature file

Only behaviour a user of itos-template can observe: what a command does to a
folder or a repository, its exit code, and what it prints.

Never here:

- code structure, dead code, dependencies, formatting (golangci-lint,
  govulncheck, `gofmt`, `go vet`);
- refactors and performance budgets (tasks in `tasks/`);
- internal logic (unit tests, next to the code).

## Black-box boundary

The steps (`*_test.go`, Go, run by [godog](https://github.com/cucumber/godog))
treat itos-template as a black box. `TestFeatures` builds the binary once a
run from this tree (`itos-template.exe` on Windows), stamped with a known
version (`stampedVersion` in `features_test.go`) as a release's build is
stamped with the release's. Each scenario gets a scratch git repository in a
temporary folder, runs the binary in it and reads its exit code, its output
and the files it leaves. A step judges itos-template by its command line
alone: it never reads its source or calls its code to decide a result.

Commands run in a clean environment: no `GIT_*`, `ITOS_*` (so no
`ITOS_TEMPLATE_*` either), `GITHUB_*`, `GH_*` or `CI` variable of the
caller's, no global or system git config, a fixed author, and none of the
caller's claude, itos or itos extensions (`itos-*`) on the PATH (itos's
T-087): each PATH folder holding one is replaced by a folder of links to the
rest, or left out where links cannot be made. A git that is an itos (the
link `itos git-shim install` makes, or a copy of an itos) is hidden the same
way, and the git the steps run themselves is the real one: the one `ITOS_GIT`
names, else the first git on the PATH that is not an itos (`git_test.go`,
itos's rule, its T-104).

## Running them

```sh
go test ./features -count=1                          # every live scenario
go test ./features -count=1 -scenarios='^@ID-CLI-'   # the live scenarios with a matching tag
GOCOVERDIR=$d go test ./features -count=1            # itos-template built with -cover, its coverage in $d
itos tests smoke run scenario                        # exactly the smoke set
```

With `GOCOVERDIR` set (an existing folder), `TestFeatures` builds
itos-template with `-cover -covermode=atomic -coverpkg=./...`, and every
command a scenario starts writes what it ran there, for itos-cc to merge with
go test's (T-19); `go tool covdata` reads it. The build keys on `GOCOVERDIR`
alone, never on `ITOS_CC_TEST_COVERDIR`: a `-cover` binary run without
`GOCOVERDIR` warns on its stderr, which would break the scenarios reading it
(`features_test.go` says why). Without it, nothing changes.

`-scenarios` takes a regular expression over each tag, `@` included, and
runs the live scenarios with a tag it matches; one that matches no tag fails
the run, so a selection never passes by running nothing. It exists because
godog's own tag filter takes exact tags joined by commas, while itos merges
selections into one regular expression joined by `|` (`tests.scenario.run` in
`itos.yaml`). Each scenario is a subtest of `TestFeatures`, named after the
scenario: `go test -v` lists them, and a failure reads
`--- FAIL: TestFeatures/<name>`.

## Tags

| Tag               | Meaning                                                                              |
| ----------------- | ------------------------------------------------------------------------------------ |
| `@phase-<n>`      | The phase a scenario belongs to.                                                     |
| `@<item>`         | The work item that turns it green (`@T-3`, `@new-renders`): `itos work show` lists it. |
| `@ID-<AREA>-<nn>` | Stable scenario ID. Commits reference these. Never reuse or renumber them.           |
| `@bug-<n>`        | Reproduces a fixed bug. Added by `fix:` commits.                                     |
| `@wip`            | Written, not yet implemented. Excluded from every run, and a `feat` may not name it. |

**IDs.** `<AREA>` is one word in capitals for the area of behaviour, and one
feature file holds one area: `CLI` is `cli.feature` (the command line's
rules, `docs/CLI.md`). `<nn>` counts up within the area from `01`; a removed
scenario's number is not given again. A new area gets a new word and a new
file.

## Commit rules

- `feat:` must add or change scenarios, or reference `@wip` ones it turns
  green. Footer: `Scenarios: ID-CLI-01` (`itos commit --scenarios`).
- `fix:` must add a `@bug-<n>` scenario that failed before the fix, or
  reference an existing scenario that was failing. Same footer.
- The commit-msg hook checks that the referenced IDs exist and are live at the
  commit.
- Red first: a slice's steps go alone in a `test` commit, its scenarios still
  `@wip` (`itos commit --item <id>`).

## The smoke set

The smoke set is the list in `smoke.yaml`: the scenarios every push runs,
beside the ones its commits name. The rule, which `itos tests smoke check
scenario` checks:

- **Every feature file with a live scenario has at least one smoke
  scenario**, listed under the file with the reason it was chosen. A file has
  more only when the list says why (`more`).
- Every ID in the list is a live scenario of the file it is listed under.
- A smoke scenario is fast and central to its file.

So a `feat` that adds a feature file, or makes a `@wip` one live, picks its
smoke scenario in the same commit.

## Moving scenarios between files

A `test` commit may move scenarios between feature files, keeping each
moved scenario's ID, name, tags and steps exactly, and moving its smoke
entry with it; itos's built-in moves rule
(`tests.scenario.range_checks` in `itos.yaml`) holds every other type to
that. Comment lines (`#`) are dropped before the comparison: a scenario's
reason is written as a comment above its tag line.

## Writing a scenario

- **Give a check its own wording.** godog matches a step's text whatever its
  keyword, so a `Then` phrased like an existing `Given` runs the setter and
  cannot fail.
- **A step is one line**, however long.
- **Assert what a user reads**: the exit code, a sentence of the output.
  Never more of the output than the behaviour is about.
- **Never start a description line with `@`.** Gherkin reads it as tags, and
  godog then refuses the file, failing every run of the features.
- **Say why beside the scenario** when a setup would make a reader ask: a
  comment above the tag line, which the moving rule ignores.
