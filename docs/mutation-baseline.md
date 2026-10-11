# Mutation baseline, 2026-10-10

One mutation run over all of `cmd` and `internal`, made for T-39 so that itos#41 (a tiered testing
strategy: fast local, judging CI, thorough nightly) can be answered from numbers. It records what
the tests kill and what they leave. Nothing was killed or excepted for it: no test was added, no
code changed, and `itos-cc.yaml` holds the same three exceptions it held before.

The records left the repository with decision 40 (T-45): the code proof and the nightly now judge
a few fresh mutants with no cache, so `.metrics/mutate/` is neither read nor committed. The tables
below stay as the last full measurement of `cmd` and `internal`, the survivors T-43 lists among
them.

## How it was run

- Commit measured: `d1db20ee1c1bd0c80f84d664609392dfecc9dfd9` (`docs: take T-39`), a clean tree.
- itos-cc 0.5.0, as `tools/bin/pinned` pins it; Go 1.27.2, linux/amd64; an Intel i7-4790K (4
  cores, 8 threads, 30 GB), itos-cc's default 4 workers, nothing else running in the checkout.
- `.metrics/mutate/` as committed at that commit: 24 files' records, 496 mutants, all decided
  with `--all-tests` by earlier items.
- The samples below ran at `7712c3a` (`docs: add record-answer-tab`), which changed only
  `tasks/work-items.yaml` after the commit measured. Their seed is that commit's id.

Run 1, every file, each mutant against its own package's tests (the fast tier):

```
tools/bin/pinned itos-cc mutation run --fail-uncovered --no-annotate cmd internal
```

Run 2, only the 18 files run 1 left with a survivor no exception covers, against the whole suite
(`go test ./...`: the godog scenarios in `features/` and every package's tests):

```
tools/bin/pinned itos-cc mutation run --fail-uncovered --all-tests --no-annotate \
  internal/answer/answer.go internal/cli/cli.go internal/disk/disk.go \
  internal/git/checkout.go internal/git/commit.go internal/git/git.go internal/git/repo.go \
  internal/manifest/decode.go internal/manifest/manifest.go internal/prompt/prompt.go \
  internal/release/build.go internal/release/release.go internal/render/leftover.go \
  internal/render/render.go internal/tempdir/tempdir.go internal/template/check.go \
  internal/template/choose.go internal/template/port/porttest/porttest.go
```

The counts below come from the records each run wrote to `.metrics/mutate/`, each mutant matched
across the two runs by its line, column and operator.

## Wall time

| Run | Wall time | Mutants run | Reused from the records | Exit |
|---|---|---|---|---|
| 1, own tests, all files | 179 s (2 min 59 s) | 704 | 189 | 1 |
| 2, `--all-tests`, 18 files | 666 s (11 min 6 s) | 115 | 465 | 1 |

Run 1 ran about 0.25 s of wall time per mutant, run 2 about 5.8 s.

## Per package

Columns:

- **Killed, own tests:** killed (or timed out) in run 1 by the package's own tests.
- **Killed earlier, reused:** killed by `--all-tests` in an earlier item's run, in a function
  unchanged since, so run 1 reused the outcome without running it. Whether the package's own tests
  kill these is not known from this run.
- **Killed only with the whole suite:** survived or was uncovered in run 1, killed in run 2. The
  whole suite is the scenarios and every other package's tests, so a kill here is not always a
  scenario's.
- **Survived:** survived the last run that judged it (run 2 for the 18 files, run 1 for the rest),
  excepted survivors left out.
- **Excepted:** a survivor `itos-cc.yaml` excepts (all three predate this run).
- **Uncovered:** no test executes its line. For the 18 files of run 2 this is under the whole
  suite (22 mutants); for the others it is under their own tests only, since run 2 did not cover
  them (44 mutants).

| Package | Mutants | Killed, own tests | Killed earlier, reused | Killed only with the whole suite | Survived | Excepted | Uncovered |
|---|---|---|---|---|---|---|---|
| cmd/itos-template | 18 | 7 | 10 | 0 | 0 | 0 | 1 |
| internal/answer | 11 | 10 | 0 | 1 | 0 | 0 | 0 |
| internal/caseform | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| internal/check | 34 | 0 | 27 | 0 | 0 | 0 | 7 |
| internal/cli | 104 | 90 | 10 | 0 | 1 | 1 | 2 |
| internal/disk | 13 | 10 | 0 | 1 | 0 | 0 | 2 |
| internal/git | 118 | 42 | 29 | 25 | 16 | 0 | 6 |
| internal/manifest | 252 | 226 | 0 | 6 | 9 | 1 | 10 |
| internal/newproject | 25 | 4 | 21 | 0 | 0 | 0 | 0 |
| internal/program | 7 | 0 | 0 | 0 | 0 | 0 | 7 |
| internal/project | not judged (below) | | | | | | |
| internal/prompt | 17 | 15 | 0 | 0 | 2 | 0 | 0 |
| internal/release | 61 | 54 | 0 | 2 | 5 | 0 | 0 |
| internal/release/releasetest | 10 | 0 | 0 | 0 | 0 | 0 | 10 |
| internal/render | 45 | 42 | 0 | 2 | 1 | 0 | 0 |
| internal/tempdir | 14 | 10 | 0 | 1 | 3 | 0 | 0 |
| internal/template | 195 | 118 | 55 | 2 | 2 | 1 | 17 |
| internal/template/port | 4 | 0 | 3 | 0 | 0 | 0 | 1 |
| internal/template/port/porttest | 63 | 0 | 24 | 34 | 2 | 0 | 3 |
| internal/version | 14 | 4 | 10 | 0 | 0 | 0 | 0 |
| **Total, project left out** | **1021** | **648** | **189** | **74** | **41** | **3** | **66** |

internal/project was not judged. In run 1's coverage step its property
`TestARecordWrittenAndReadBackIsTheSameRecord` failed after 90 of rapid's default 100 cases (itos-cc
does not set `RAPID_CHECKS`, which CI sets to 1). The failing record has the answer `"\t\n"`, and
it does not read back: `yaml: line 12: found a tab character where an indentation space is
expected`. rapid saved the case to a failfile, which made the package's baseline fail, so itos-cc
judged none of `project.go`'s mutants. Its record keeps the 5 mutants of 2 functions decided
before; its other 5 functions have none. In 30 reruns of that property at 100 cases it failed 0
times. The failure is the idea `record-answer-tab`.

## Per file

The same columns, in the same order. A file marked * was in run 2.

| File | Mutants | Own | Reused | Whole suite only | Survived | Excepted | Uncovered |
|---|---|---|---|---|---|---|---|
| cmd/itos-template/main.go | 18 | 7 | 10 | 0 | 0 | 0 | 1 |
| internal/answer/answer.go * | 11 | 10 | 0 | 1 | 0 | 0 | 0 |
| internal/caseform/caseform.go | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| internal/check/check.go | 34 | 0 | 27 | 0 | 0 | 0 | 7 |
| internal/cli/cli.go * | 7 | 5 | 0 | 0 | 1 | 1 | 0 |
| internal/cli/codes.go | 12 | 1 | 10 | 0 | 0 | 0 | 1 |
| internal/cli/completion.go | 73 | 72 | 0 | 0 | 0 | 0 | 1 |
| internal/cli/flags.go | 12 | 12 | 0 | 0 | 0 | 0 | 0 |
| internal/disk/disk.go * | 13 | 10 | 0 | 1 | 0 | 0 | 2 |
| internal/git/checkout.go * | 17 | 0 | 6 | 11 | 0 | 0 | 0 |
| internal/git/commit.go * | 11 | 8 | 0 | 0 | 3 | 0 | 0 |
| internal/git/errors.go | 3 | 3 | 0 | 0 | 0 | 0 | 0 |
| internal/git/git.go * | 24 | 13 | 0 | 0 | 7 | 0 | 4 |
| internal/git/repo.go * | 63 | 18 | 23 | 14 | 6 | 0 | 2 |
| internal/manifest/decode.go * | 19 | 17 | 0 | 1 | 1 | 0 | 0 |
| internal/manifest/manifest.go * | 194 | 170 | 0 | 5 | 8 | 1 | 10 |
| internal/manifest/setup.go | 23 | 23 | 0 | 0 | 0 | 0 | 0 |
| internal/manifest/shell.go | 16 | 16 | 0 | 0 | 0 | 0 | 0 |
| internal/newproject/newproject.go | 25 | 4 | 21 | 0 | 0 | 0 | 0 |
| internal/program/program.go | 7 | 0 | 0 | 0 | 0 | 0 | 7 |
| internal/prompt/prompt.go * | 17 | 15 | 0 | 0 | 2 | 0 | 0 |
| internal/release/build.go * | 35 | 31 | 0 | 1 | 3 | 0 | 0 |
| internal/release/release.go * | 26 | 23 | 0 | 1 | 2 | 0 | 0 |
| internal/release/releasetest/releasetest.go | 10 | 0 | 0 | 0 | 0 | 0 | 10 |
| internal/render/leftover.go * | 16 | 14 | 0 | 1 | 1 | 0 | 0 |
| internal/render/render.go * | 29 | 28 | 0 | 1 | 0 | 0 | 0 |
| internal/tempdir/tempdir.go * | 14 | 10 | 0 | 1 | 3 | 0 | 0 |
| internal/template/check.go * | 15 | 14 | 0 | 1 | 0 | 0 | 0 |
| internal/template/choose.go * | 28 | 24 | 0 | 1 | 2 | 0 | 1 |
| internal/template/errors.go | 19 | 0 | 3 | 0 | 0 | 0 | 16 |
| internal/template/name.go | 72 | 71 | 0 | 0 | 0 | 1 | 0 |
| internal/template/port/port.go | 4 | 0 | 3 | 0 | 0 | 0 | 1 |
| internal/template/port/porttest/porttest.go * | 63 | 0 | 24 | 34 | 2 | 0 | 3 |
| internal/template/redact.go | 9 | 9 | 0 | 0 | 0 | 0 | 0 |
| internal/template/release.go | 39 | 0 | 39 | 0 | 0 | 0 | 0 |
| internal/template/template.go | 13 | 0 | 13 | 0 | 0 | 0 | 0 |
| internal/version/version.go | 14 | 4 | 10 | 0 | 0 | 0 | 0 |

In run 2, 4 mutants uncovered under their own tests became covered by the whole suite and
survived it: one each in git/commit.go, prompt/prompt.go and tempdir/tempdir.go, and two in
template/port/porttest/porttest.go. They are counted as survived.

## Repeatability of the whole-suite outcomes

With run 2's records in place, every mutant of the 18 files that had run was run again, each
with the scope its outcome was recorded under. This took 721 s of wall time:

```
tools/bin/pinned itos-cc mutation sample --count 1000 --json <the 18 files of run 2>
```

It sampled 580 mutants; the seed was `7712c3a903ff59eeb0133ec2a304418c3909a9e8`. All 412 outcomes
decided by the files' own tests came out the same. Of the 168 decided with `--all-tests`, 20 came
out different: 18 recorded survivors were killed, and 2 recorded kills survived. One more
whole-suite kill, `internal/manifest/decode.go:77:17` `1` → `0`, survived in the
`sample --count 10` below and was killed again in this resample. So 21 mutants changed outcome
at least once:

| Package | Survivors killed on rerun | Whole-suite kills that survived on rerun |
|---|---|---|
| internal/cli | 1 | 0 |
| internal/git | 6 | 0 |
| internal/manifest | 3 | 1 |
| internal/prompt | 1 | 0 |
| internal/release | 3 | 0 |
| internal/render | 1 | 1 |
| internal/tempdir | 2 | 1 |
| internal/template/port/porttest | 1 | 0 |
| **Total** | **18** | **3** |

None of the changed outcomes was a timeout. Without any mutant, the whole suite
(`go test -count=1 ./...`) passed 8 times out of 8, run four at a time as the four workers run it.

### The cause (T-42)

T-42 found two causes, both since removed from main, and reran the check there: every outcome now
repeats. Each cause makes the whole suite fail without the mutant having anything to do with it, so
it can only add false kills: a recorded survivor killed on rerun, or a recorded kill that was
false and survives on rerun.

**What flipped T-39's outcomes was a flaky property and the failfile it left.**
`TestARecordWrittenAndReadBackIsTheSameRecord` (internal/project) failed at random at d1db20e, on
the answer `"\t\n"` above (`record-answer-tab`): 8 runs in 200 at rapid's default 100 checks. rapid
v1.3.0 saves a failing case under the package's `testdata/rapid/`, which git ignores, and replays
every saved case on the next run. itos-cc runs each worker's mutants in one copy of the project kept
for the whole run, so after one such failure every later whole-suite run of that worker fails at the
property (`[rapid] failed after 0 tests`), whatever its mutant. Measured in a scratch worktree at
d1db20e, over six files of the packages that held 13 of T-39's 21 flips (internal/git/git.go and
commit.go, internal/tempdir/tempdir.go, internal/release/build.go and release.go,
internal/prompt/prompt.go): an own-tests run and then an `--all-tests` run, as runs 1 and 2 above,
took 19 s and 282 s and left 23 whole-suite outcomes, 22 survivors and a kill. Three samples of
every mutant (`mutation sample --count 1000`, 280 s, 153 s and 162 s) killed 0, 5 and 0 of the 22
survivors, and three more with T-41's global config put in killed none, which a 4% rate allows: no
fresh failure in their 69 whole-suite runs is about 1 chance in 16. One more sample, with
`--test-command` set to a script that runs `go test -count=1 -failfast ./...` and keeps each run's
output, killed 8 of the 22 in 772 s. Its 124 logs name the cause: in three of the four workers the
property failed afresh once, and the 38 later runs of those workers failed replaying it; no log
names `maintenance.lock` or any other cause. Sixteen whole suites without a mutant, run four at a
time in one checkout over four rounds, show the same: one fresh failure in round 1, then a replay in
each of the 12 runs of rounds 2 to 4. bug-6's fix (a2836b1) made the property draw only answers the
commands take, and at main it failed 0 times in 200. The way one failure becomes many false kills is
itos-cc's, reported as donvargax/itos-cc#35 with a reproduction.

**On main, git's detached auto-maintenance did the same until T-41.** adopt's scenarios, live since
slice-12 and still `@wip` at d1db20e, snapshot all of the made project's `.git`, and a Given step's
commit starts git's maintenance, whose `.git/objects/maintenance.lock` the snapshot meets. T-41
turned it off for every git the scenarios start (cb14822). With that one line undone in the working
tree (`GIT_CONFIG_GLOBAL` back to `os.DevNull`), a sample of the records below killed 10 of 47
recorded whole-suite survivors, and 2 of 12 whole suites run four at a time failed, both at adopt's
snapshot (`removed .git/objects/maintenance.lock`).

**The rerun on main.** At `bccf4e9` (`docs: take T-42`), on the same machine with the same
itos-cc, four workers and nothing else running in the checkout, the 18 files of run 2 were run as
T-39 ran them, then every mutant was run again:

```
tools/bin/pinned itos-cc mutation run --fail-uncovered --no-annotate --json <the 18 files>
tools/bin/pinned itos-cc mutation run --fail-uncovered --all-tests --no-annotate --json \
  <the 18 files>
tools/bin/pinned itos-cc mutation sample --count 1000 --json <the 18 files>
```

The two runs took 150 s (535 mutants run) and 1257 s (149 run, 489 reused). Fresh records were
used, not those committed: the committed ones held whole-suite outcomes for only 8 of the 18 files,
almost all kills, while a flip is mostly a survivor killed. The sample took 1434 s, seed
`bccf4e91364052c54c3a1c26ef065d3a8455df31`, and drew 638 mutants. Every outcome came out the same:
the 477 decided by the files' own tests (471 kills, 2 survivors, 4 timeouts) and the 161 decided
with `--all-tests` (112 kills, 49 survivors). At T-39's rate, 20 in 168, the chance of no flip in
161 is about 1 in 700 million, and the sample with T-41 undone shows the same records do flip when
a cause is put back. The sample with T-41 undone left out porttest.go and took 588 s.

The whole suite has grown since d1db20e: a whole-suite mutant took 30 s to 50 s here, against about
23 s in T-39's runs.

**What changes in the baseline.** Both causes only add kills, so T-39's 41 survivors are
survivors, and its three recorded kills that survived on rerun (the table above) were false kills.
A survivor falsely killed in both run 2 and the resample stays hidden: the resample killed 18 of the
41 survivors, so run 2's false kills were about 5, of which 3 showed, leaving about 2 hidden. So at
d1db20e the kills only the whole suite makes were between 68 and 71, not between 71 and 92, and the
survivors between 44 and 47, not between 23 and 41. The tables above keep what the runs recorded.

The records T-42's runs wrote were not committed: with them, `mutation check` reports the 49
whole-suite survivors, so it cannot stay green (a bare `mutation check` already exits 1 at HEAD, on
284 functions stale or missing elsewhere), and `.metrics/mutate/` was restored to HEAD.

## The records

The records these runs wrote were not committed: `.metrics/mutate/` was restored to the commit
measured. With them in place:

- `tools/bin/pinned itos-cc mutation check` exits 1. It reports the 41 survivors, plus 26 functions
  missing results: the 5 of `internal/project/project.go` above, and 21 in
  `tools/bin/release-notes/main.go` (14) and `tools/bin/release-version/main.go` (7), which a bare
  `mutation check` covers but `cmd internal` does not.
- `tools/bin/pinned itos-cc mutation sample --count 10` exits 1, in 30 s, seed
  `7712c3a903ff59eeb0133ec2a304418c3909a9e8`. Of the 10 mutants it sampled, 9 agreed with the
  records (the recorded survivor among them, `internal/git/git.go:115:94` `0` → `1`, survived
  again). One did not: `mismatch internal/manifest/decode.go:77:17` `1` → `0`
  `in github.com/donvargax/itos-template/internal/manifest#strict: recorded killed, now survived`.

## The records refreshed under itos-cc 0.12.0 (T-43)

T-43 pinned itos-cc 0.12.0 and wrote a fresh record of every function of `cmd` and `internal`, so
that the code proof and CI's sample trust records that match the code. Before it, at `19a5a87`
(`docs: take T-43`), a check under 0.5.0 found 148 functions stale and 115 missing. After it, a
check finds none stale, missing, coverage-missing or coverage-stale: all 505 functions of the 49
files are fresh, 301 of them holding mutants. The check still exits 1, on the survivors, the
uncovered mutants and the uncovered statements below, which stay: nothing was killed, covered or
excepted, no test was added and no code changed, since what to kill waits on itos#41's tiers.
`itos-cc.yaml` holds the same three exceptions.

### How it was run

- The run started at `de8be5f`, which pinned 0.11.0, on a clean tree; Go 1.27.2, linux/amd64, the
  same i7-4790K (4 cores, 8 threads, 30 GB), itos-cc's default 4 workers. The machine was not
  quiet: other projects' work kept its load average between 20 and 45 the whole run, so the wall
  times are high.
- 0.12.0 came out during the run. Pinned at `36d0674`, its check read every record of the 0.11.0
  run fresh, coverage evidence included, so nothing was run again: 0.12.0 makes only Python and
  TypeScript evidence stale once.
- In CI the same records read coverage-stale, all 505 functions, on itos-cc's `@go-env` input:
  itos-cc fingerprints the raw value of `GOTOOLCHAIN`, which actions/setup-go sets to `local` and a
  shell here leaves unset (donvargax/itos-cc#36). Here, `GOTOOLCHAIN=auto`, Go's own default, reads
  all 505 stale too. Since `6e0d92d`, `tools/bin/pinned` runs itos-cc with `GOTOOLCHAIN=local`, as
  CI does, and the run was made again under it. It reused every outcome but the 50 survivors, which
  it ran again (all 50 survived again), and measured the coverage again.
- Those records read coverage-stale in CI too, for another reason: itos-cc takes every `.go` file
  on disk under the module as an input, tracked or not, `.git/`, `testdata` and ignored folders
  included (donvargax/itos-cc#37), and a `.go` file kept for reading under `.git/` had become one.
  With it gone, the run was made a third time, again reusing all but the 50 survivors, which all
  survived again, and a fresh clone with only `HOME`, `PATH` and `GOTOOLCHAIN=local` set read all
  505 functions fresh. Until #37 is fixed, a checkout that records coverage holds no `.go` file git
  does not track.

```
tools/bin/pinned itos-cc mutation run --fail-uncovered --all-tests --no-annotate --json cmd internal
tools/bin/pinned itos-cc mutation check --fail-uncovered --json cmd internal
tools/bin/pinned itos-cc mutation sample --count 1 cmd internal
```

| Command | Wall time | Exit |
|---|---|---|
| `mutation run`, 0.11.0 | 11001 s (3 h 3 min), 1484 mutants run, 20 uncovered, none reused | 1 |
| `mutation run`, 0.12.0, `GOTOOLCHAIN=local` | 413 s, 50 mutants run, 1434 reused | 1 |
| `mutation run`, 0.12.0, no stray `.go` file | 406 s, 50 mutants run, 1434 reused | 1 |
| `mutation check`, 0.12.0 | 2 s | 1 |
| `mutation sample --count 1`, 0.12.0 | 32 s, and 24 s after the second run | 0 |

Every mutant ran against the whole suite: under 0.11.0 and 0.12.0, `--all-tests` runs the whole
suite for every mutant it runs, and reuses only fresh results, which 0.5.0's records were not. A
mutant took 24 s at the median and 632 s at most. The tier AGENTS.md names, own tests first and
the whole suite for what they leave, holds only as two runs: tried in a scratch worktree, an
`--all-tests` run of `internal/answer/answer.go` after an own-tests run reused its 10 own kills and
ran 1 mutant. Done that way here, it would have run an own-tests pass (313 s, below) and then about
345 mutants with the whole suite rather than 1484.

After the runs no rapid failfile was left in the tree (no `testdata/rapid` anywhere), and before the
records were committed one mutant was sampled again each time (the person's choice of a local
sample of 1; CI samples 10): `internal/answer/answer.go:33` `0` → `1`, seed `36d06748`, and
`internal/template/template.go:161` `!=` → `==`, seed `6e0d92d7`, each killed again, no mismatch.

### Per package

The records hold every outcome with scope `all-tests`. Which kills the package's own tests make
comes from a second run, in a scratch worktree at `36d0674` with no records, without `--all-tests`
(313 s): a kill there is **Killed, own tests**, and a whole-suite kill it left survived or uncovered
is **Killed only with the whole suite**. No mutant its own tests killed survived the whole suite.
**Survived** leaves out the three excepted survivors, counted under **Excepted**. **Uncovered** is
a mutant no test of the whole suite executes; **Uncovered statements** are the
`mutation.uncovered-statement` blocks, mutants or not, strict coverage reports. One block moved
between the two coverage measurements: `internal/manifest/decode.go:48` (`Decode`, a second
document that does not parse) was uncovered in the first and covered in the second, since only a
property's random input reaches it, so the count is 187 here (in the third measurement too) and
was 188 under 0.11.0.

| Package | Mutants | Killed, own tests | Killed only with the whole suite | Survived | Excepted | Uncovered | Uncovered statements |
|---|---|---|---|---|---|---|---|
| cmd/itos-template | 18 | 11 | 7 | 0 | 0 | 0 | 2 |
| internal/adopt | 39 | 22 | 17 | 0 | 0 | 0 | 7 |
| internal/adoption | 42 | 41 | 1 | 0 | 0 | 0 | 2 |
| internal/answer | 11 | 10 | 1 | 0 | 0 | 0 | 0 |
| internal/caseform | 16 | 16 | 0 | 0 | 0 | 0 | 1 |
| internal/check | 18 | 0 | 18 | 0 | 0 | 0 | 4 |
| internal/cli | 135 | 117 | 16 | 1 | 1 | 0 | 6 |
| internal/cutrelease | 25 | 0 | 25 | 0 | 0 | 0 | 7 |
| internal/disk | 13 | 10 | 1 | 0 | 0 | 2 | 6 |
| internal/git | 270 | 214 | 33 | 17 | 0 | 6 | 72 |
| internal/manifest | 255 | 229 | 3 | 12 | 1 | 10 | 12 |
| internal/newproject | 26 | 13 | 13 | 0 | 0 | 0 | 2 |
| internal/program | 7 | 0 | 7 | 0 | 0 | 0 | 0 |
| internal/project | 30 | 28 | 2 | 0 | 0 | 0 | 6 |
| internal/prompt | 17 | 15 | 0 | 2 | 0 | 0 | 3 |
| internal/release | 61 | 54 | 1 | 6 | 0 | 0 | 18 |
| internal/release/releasetest | 10 | 0 | 10 | 0 | 0 | 0 | 8 |
| internal/releasing | 44 | 43 | 1 | 0 | 0 | 0 | 0 |
| internal/render | 45 | 42 | 1 | 2 | 0 | 0 | 1 |
| internal/tempdir | 14 | 9 | 1 | 4 | 0 | 0 | 5 |
| internal/template | 206 | 180 | 20 | 4 | 1 | 1 | 5 |
| internal/template/port | 5 | 4 | 0 | 0 | 0 | 1 | 2 |
| internal/template/port/porttest | 115 | 41 | 72 | 2 | 0 | 0 | 4 |
| internal/update | 21 | 0 | 21 | 0 | 0 | 0 | 11 |
| internal/updating | 47 | 46 | 1 | 0 | 0 | 0 | 2 |
| internal/version | 14 | 14 | 0 | 0 | 0 | 0 | 1 |
| **Total** | **1504** | **1159** | **272** | **50** | **3** | **20** | **187** |

Under its own tests alone the project leaves 73 survivors, 272 uncovered mutants and 653 uncovered
statements; the whole suite kills 272 of those 345 mutants and leaves 50 survivors, 3 excepted
and 20 uncovered.

### The survivors

Each survivor, by file, line and column, with its mutation and function, for the kill items to be
split from:

**internal/cli** (1)

- `internal/cli/cli.go:126:20` `false` → `true` in `cli.UI#JSON`

**internal/git** (17)

- `internal/git/commit.go:28:10` `!=` → `==` in `git.Committer#Identity`
- `internal/git/commit.go:55:22` `>` → `>=` in `git.Committer#Commit`
- `internal/git/commit.go:55:24` `0` → `1` in `git.Committer#Commit`
- `internal/git/git.go:98:18` `!=` → `==` in `git#names`
- `internal/git/git.go:115:33` `&&` → `||` in `git#executable`
- `internal/git/git.go:115:50` `==` → `!=` in `git#executable`
- `internal/git/git.go:115:63` `||` → `&&` in `git#executable`
- `internal/git/git.go:115:91` `!=` → `==` in `git#executable`
- `internal/git/git.go:115:94` `0` → `1` in `git#executable`
- `internal/git/git.go:197:10` `1` → `0` in `git#exitCode`
- `internal/git/repo.go:178:10` `||` → `&&` in `git.Repo#files`
- `internal/git/repo.go:214:18` `0` → `1` in `git.Repo#blobs`
- `internal/git/repo.go:218:27` `1` → `0` in `git.Repo#blobs`
- `internal/git/repo.go:233:24` `!=` → `==` in `git.Repo#blobs`
- `internal/git/repo.go:233:31` `&&` → `||` in `git.Repo#blobs`
- `internal/git/repo.go:233:42` `==` → `!=` in `git.Repo#blobs`
- `internal/git/repo.go:245:22` `||` → `&&` in `git#readBlob`

**internal/manifest** (12)

- `internal/manifest/decode.go:77:17` `1` → `0` in `manifest#strict`
- `internal/manifest/decode.go:77:19` `<` → `<=` in `manifest#strict`
- `internal/manifest/manifest.go:94:16` `1` → `0` in `manifest.Check#UnmarshalYAML`
- `internal/manifest/manifest.go:94:18` `<` → `<=` in `manifest.Check#UnmarshalYAML`
- `internal/manifest/manifest.go:343:18` `true` → `false` in `manifest#collidingForms`
- `internal/manifest/manifest.go:349:19` `>` → `>=` in `manifest#collidingForms`
- `internal/manifest/manifest.go:357:33` `>=` → `>` in `manifest#collidingForms`
- `internal/manifest/manifest.go:357:40` `&&` → `||` in `manifest#collidingForms`
- `internal/manifest/manifest.go:357:49` `<=` → `<` in `manifest#collidingForms`
- `internal/manifest/manifest.go:392:96` `+` → `-` in `manifest.Manifest#checkChecks`
- `internal/manifest/manifest.go:392:97` `1` → `0` in `manifest.Manifest#checkChecks`
- `internal/manifest/manifest.go:566:79` `&&` → `||` in `manifest.Manifest#FeaturesNamed`

**internal/prompt** (2)

- `internal/prompt/prompt.go:47:45` `||` → `&&` in `prompt.Lines#Ask`
- `internal/prompt/prompt.go:77:45` `&&` → `||` in `prompt#Terminal`

**internal/release** (6)

- `internal/release/build.go:157:10` `!=` → `==` in `release#goIn`
- `internal/release/build.go:183:14` `!=` → `==` in `release#extract`
- `internal/release/build.go:230:34` `!=` → `==` in `release#write`
- `internal/release/release.go:75:58` `>` → `>=` in `release#Newest`
- `internal/release/release.go:86:16` `<=` → `<` in `release#compareTags`
- `internal/release/release.go:91:12` `!=` → `==` in `release#compareTags`

**internal/render** (2)

- `internal/render/leftover.go:81:38` `0` → `1` in `render.Scan#find`
- `internal/render/leftover.go:84:68` `-` → `+` in `render.Scan#find`

**internal/tempdir** (4)

- `internal/tempdir/tempdir.go:48:17` `*` → `/` in `tempdir#Remove`
- `internal/tempdir/tempdir.go:48:56` `<=` → `<` in `tempdir#Remove`
- `internal/tempdir/tempdir.go:58:29` `!=` → `==` in `tempdir#Discard`
- `internal/tempdir/tempdir.go:74:29` `!=` → `==` in `tempdir.Renders#Remove`

**internal/template** (4)

- `internal/template/choose.go:29:52` `>` → `>=` in `template#Choose`
- `internal/template/choose.go:29:54` `0` → `1` in `template#Choose`
- `internal/template/choose.go:73:13` `&&` → `||` in `template#Choose`
- `internal/template/release.go:131:11` `false` → `true` in `template.Template#UseNewest`

**internal/template/port/porttest** (2)

- `internal/template/port/porttest/porttest.go:154:13` `0` → `1` in `porttest#file`
- `internal/template/port/porttest/porttest.go:549:19` `&&` → `||` in `porttest.Terminal#Ask`

### A test file's change makes every whole-suite outcome stale

Tried in a scratch worktree at `36d0674` with these records: one comment added at the end of
`internal/caseform/caseform_test.go`, and a bare check (`mutation check --fail-uncovered --json cmd
internal`) reports all 301 functions holding mutants `mutation.stale` ("whole-suite results made
before inputs changed") and all 505 `mutation.coverage-stale`. A new, empty test file does the
same. Every outcome here is whole-suite, so any change to any `_test.go` file under the module
makes the whole record stale until a full run, and a bare check of the records cannot stay green
between items. Strict Go coverage evidence goes stale more often still: a comment added to
`internal/caseform/caseform.go` left every mutant outcome fresh but made all 505 functions
coverage-stale. The proof's form is not affected: with only the test file changed,
`mutation check --since HEAD --fail-uncovered --json cmd internal` judges no source file and passes.
A change to a scenario's `.feature` file makes nothing stale, since `itos-cc.yaml` names no
`mutation.tests.support` files, so a whole-suite kill a scenario made stays fresh when the scenario
changes (the idea `mutation-support-features`). For itos#41: own-scope outcomes go stale only when
their package's tests change, so a record whose kills are own tests where they can be, the whole
suite deciding only the rest, would stay fresh across most items, though its coverage evidence
would not.
