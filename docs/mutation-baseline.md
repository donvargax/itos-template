# Mutation baseline, 2026-10-10

One mutation run over all of `cmd` and `internal`, made for T-39 so that itos#41 (a tiered testing
strategy: fast local, judging CI, thorough nightly) can be answered from numbers. It records what
the tests kill and what they leave. Nothing was killed or excepted for it: no test was added, no
code changed, and `itos-cc.yaml` holds the same three exceptions it held before.

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

On these numbers, the 74 kills only the whole suite makes are between 71 and 92, and the 41
survivors between 23 and 41. None of the changed outcomes was a timeout. Without any mutant, the
whole suite (`go test -count=1 ./...`) passed 8 times out of 8, run four at a time as the four
workers run it. The cause was not found; it is the idea `mutation-suite-repeats`.

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
