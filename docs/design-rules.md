# Code design rules, and how each is held

Every code design rule of `AGENTS.md` and the live decisions, and what holds it (T-49). A rule a
linter can decide without false alarms is held by golangci-lint, among the linters the release
`tools/bin/pinned` pins already ships, configured in `.golangci.yml`, whose comments name the
decision each rule holds. The hooks and CI run it on every commit and push. A rule no linter can
decide is judgement, for the author and the reviewer, and says why. What the code does, as against
how it is built, is the scenarios' to hold (`features/`), and is not listed here.

| Rule | Decision | How it is held |
| --- | --- | --- |
| A slice imports no other slice | 17 | depguard, rule `slices` |
| The domain and its ports import no infra, slice, UI or kong | 17 | depguard, rule `domain` |
| The domain's code does no I/O: no `os`, `net`, `syscall` or `log`, so it reads no environment and prints nothing | 17 | depguard, rule `domain-io` (its tests may read a fixture) |
| Infra imports nothing of ours but the ports it implements | 17 | depguard, rule `infra` |
| kong stays in the UI: `cmd`, `internal/cli` and the slices | 4, 17 | depguard, rules `kong` and `domain` |
| Every error kind is given its exit code, in one switch per sealed set, a default not counting | 17, `docs/CLI.md` rule 31 | gochecksumtype; whether a new error joins a sealed set is judgement, an unclassified one exiting 70 |
| Output goes through the writers the UI hands down, never to the process's stdout behind them | 17 | forbidigo: `fmt.Print`, `fmt.Printf`, `fmt.Println`, `print`, `println` |
| main alone exits, with the code the slice returned | 17 | forbidigo: `os.Exit`, `syscall.Exit`, `log.Fatal*`, `cmd/itos-template/main.go` excepted, and tests' `os.Exit` and `syscall.Exit`: TestMain's `os.Exit(m.Run())` and a helper process exiting with a code (`internal/git`'s stub git) are standard Go, and the rule is about production code (T-48) |
| A bug exits 70, as an error nobody classified, never as a panic | 17, 41, `docs/CLI.md` rule 31 | forbidigo: `panic`, tests left out, as a test's panic is its own failure, not the binary's exit (T-52) |
| A slice stays in one file until it grows large: 600 lines of code | 17 | revive, `file-length-limit` |
| Commands carry domain objects: no DTO or options struct copying a type | 17 | Judgement: whether a struct copies a type is what its fields stand for, which no linter reads; Run assembling a command from kong's flags is the one copy allowed |
| An anti-corruption layer only where an outside model would leak in | 17 | Judgement: whether a model would leak is a question about the outside, not a shape in the code |
| A handler only wires; the work is in the domain | 17 | Judgement: what counts as work rather than wiring is a reading of the code, though depguard keeps the domain from reaching the slices |
| No mocking library, in any package or test | 19 | depguard, rule `mocks` |
| Tests use our fakes and stubs, never a mock written by hand, and assert outcomes, never calls | 19 | Judgement: a hand-written mock is a type of ours like a fake, told apart only by scripting and checking calls; a name proves nothing either way |
| The domain is unit-tested in full | 19 | `tools/bin/domain-coverage`, a floor of 80% per domain package, in CI |
| A test outside the domain exists only for what a scenario cannot reach, saying so | 19 | Judgement: whether a scenario can reach something is not in the code |
| rapid only in the domain packages' own tests | 20 | depguard, rule `rapid` |
| Code that differs by OS takes the OS as a value: `runtime.GOOS` is read only at the edge, handed in as a value | 26 | forbidigo: every read of `runtime.GOOS` refused, the edges excepted by their source, the slices' `template.SystemOf(runtime.GOOS)` and `internal/git`'s `bin(runtime.GOOS)` (T-48), which infra hands in having no `template` to import; tests left out (below) |
| The domain differs by OS through no package: no `path/filepath`, no `runtime` | 26 | depguard, rule `domain-io` |
| A build-tagged file only for a call one OS lacks, holding no mutation site | 26 | Judgement: the decision allows such files, and whether a call is one an OS lacks is not a shape; none exists today |
| Configuration is read strictly, refusing what JSON cannot say | 22 | forbidigo: `yaml.Unmarshal` and `yaml.NewDecoder` refused outside `manifest.Decode`, tests left out (below); what the reader refuses, its unit tests and scenarios |
| A library is weighed before our own code, and the choice is the person's | 13 | Judgement: whether a library likely solves a problem is a question about the world |
| The libraries the decisions set aside stay out: cobra and urfave/cli, go-git, gopter and `testing/quick`, gitleaks as a library, the completion libraries | 4, 19, 20, 21, 25, 38 | depguard, rule `set-aside` |
| A provider is chosen when a project is made, never by runtime indirection | 2 | Judgement: what a template's code is for is not a shape a linter sees |
| A changed function's mutants are killed, or excepted with a reason | 27, 40 | The code proof, itos-cc's mutation run, in CI over a push's range |
| An exception is one finding, on its line, naming its linter and saying why, and goes with its finding | T-22, gosec's comment | nolintlint: `require-specific`, `require-explanation`, unused ones reported |

## What the linters leave to judgement

- **The edge line is allowed whole.** forbidigo excepts the line that hands the OS in, by its source,
  `SystemOf(runtime.GOOS)`, or `bin(runtime.GOOS)` in `internal/git/git.go`, so a comparison
  written on that same line, `template.SystemOf(runtime.GOOS) == template.Windows`, passes. A slice
  that branches on the System it made is the review's to catch. A new edge is added to the
  exception by name, as T-48 added `internal/git`'s, so each is a reviewed change.
- **Tests may ask which OS they run on.** A test of the real OS, an adapter's or a scenario's step,
  skips or expects what that OS does, and the code proof decision 26 serves mutates no test, so the
  `runtime.GOOS` rule holds production code only. A test copying production logic that branches
  on the OS (the scenario harness's own `executable` in `features/git_test.go`) is the review's.
- **Tests read YAML loosely.** They read what the binary wrote to check it; the strict reader is
  for the input a template brings.
- **tools/bin's programs** are the repository's own scripts, each a main printing to its stdout and
  exiting with its code, outside the layers decision 17 draws, so the printing and exiting rules
  leave them out.

## What the linters found

When T-49 turned the rules on, they found:

- `internal/git/git.go`: `names` and `executable` compared `runtime.GOOS` inside their logic,
  which decision 26 recorded as moving when an item next touched them. T-49 left each a
  `//nolint:forbidigo` above it; T-48 had them take the OS as a value, handed in by `Bin` as
  `bin(runtime.GOOS)`, the edge `.golangci.yml` excepts, and deleted both.
- `internal/manifest/manifest.go`, `Replacements`: a `panic` when an answer `Check` did not take
  reaches it. A panic exits 2 with a trace, where rule 31 has a bug exit 70. Returning an error
  instead changes `Replacements` and its three callers in `internal/template` (a render's plan,
  `Check`, and `Setup`, which returns no error today, so its callers too) and needs tests for each
  new path, so it is not this task's: the panic rule stays off until the
  person decides, the idea `no-panic`. T-52 settled it as decision 41 has it: `Question.Take` keeps
  the words it parsed in a `manifest.Answer`, `Replacements` takes those and cannot fail, and
  forbidigo refuses `panic`.
- `tools/bin/release-notes` and `tools/bin/release-version` print and exit in their main, which is
  theirs to do (above).

## Secure by design: where each input is checked

The rule, decision 41 (T-53): secure by design. Every input from outside is checked where it
enters, at the outer layers that meet other systems, and turned into a checked value, one whose
type proves it was checked; inner layers trust it and never check it again. Bad input is refused
with an error naming it; a lossless normalization is fine, a silent repair is not. It holds because
everything lives under `internal/`, which no other module imports, so nothing outside reaches an
inner layer. The map below is the audit: for each edge, the input, the value it becomes, and where
it is checked. The ideal T-52 set is a value object with unexported fields and one constructor
(`manifest.Answer`, taken only by `Question.Take`); an input still carried inward as a raw string or
map is noted, with the idea that would wrap it.

| Edge | Input | Checked value | Where checked |
| --- | --- | --- | --- |
| Command line / terminal | the template name (`new`/`adopt`/`check` arg, the record's in `update`) | a raw string (idea `p1-template-name-value`) | `template.CheckName` (`manifest.Refused`), in each slice's `Handle` before anything is cloned |
| Command line / terminal | the project's folder | `project.Folder` | `project.Writer.Look` (missing or empty, else `*NotFolder`/`*NotEmpty`) |
| Command line / terminal | the stack and the features (`--stack`, `--feature`, the record's) | `manifest.Combination` | `template.Choose` against the manifest (unknown stack or feature, another stack's, needs not chosen, unsupported) |
| Command line / terminal | an answer (`--answer name=answer`, typed on the terminal, the record's) | `manifest.Answers` of `manifest.Answer` | `manifest.Question.Take`, via `answer.Read` and `template.Choose` (decision 41, T-52) |
| Command line / terminal | the release version (`--ref`, `--to`) | a raw string, used only after `semver.IsValid` (idea `p1-version-value`) | `template.find` / `Cuttable` (`golang.org/x/mod/semver`) |
| Command line / terminal | the declined paths (`adopt --decline`) | `[]string`, each a tree path (idea `p1-tree-path-value`) | `project.Decline` (`manifest.IsTreePath`, else `*BadDeclined`) |
| Command line / terminal | a flag given twice, a switch given a value, a flag given none | kong's parsed value | `internal/cli/flags.go` (kong mappers and the before-reset hook) |
| Command line / terminal | the completion shell (`__complete`, `completion <shell>`) | a key of `Scripts()` | kong's `enum`, and `Scripts()` lookup refusing an unknown shell |
| Template repository | the manifest (`itos-template.yaml`) | `*manifest.Manifest` | `manifest.Parse`: `manifest.Decode` strict (decision 22), then `(*Manifest).check` |
| Template repository | a tree path | `port.File.Path`, each element a name a file can have | `render.Path`, in `render.Plan`, before anything is written (empty, `.`, `..`, a separator, NUL, or a spelling NTFS, FAT or HFS+ would resolve to the `.git` folder, from those systems' own naming rules (Microsoft's file-naming docs and Apple's TN1150), is a `*render.BadName`); `render.Plan` also refuses two names that are one file once case and Unicode normalization are folded, a `*render.CaseClash`, on every disk (decision 3), so a case-folding one (NTFS, HFS+, APFS) cannot write one over the other; `internal/disk`'s `Write` is the second layer, refusing to write through a symbolic link in a parent folder (`*Through`) or over or through an entry already at a file's own path, which it creates with `O_EXCL` (`*Occupied`) (bug-8, bug-9) |
| Template repository | a file's contents | the replaced bytes; a binary file copied as it is | `render.Replacer.Contents` (text by `isText`, git's NUL rule); a symlink's target is replaced and written as given (idea `p1-symlink-target`) |
| Template repository | a check's or a setup step's words | `manifest.Check` / `manifest.Step` | `(*Manifest).checkChecks` / `checkSetup` (no program, a bad scan, a control or drawn-as-nothing character) |
| Project record | `.itos-template.yaml` | `project.Record` with exported fields (idea `p1-record-value`) | `project.ReadRecord`: `manifest.Decode` strict, then the version, the named fields and `declinedProblems` |
| Project record | the record's branch commits (`commits`) | a `map[string]string` reaching git unchecked (idea `p1-record-commits-checked`) | not checked by us; git refuses a bad object name (`rev-parse`, `merge-tree`, `cat-file`) |
| git output and exit codes | git's stdout (`ls-tree`, `cat-file`, `for-each-ref`, `merge-tree`, `status`) | `port.File`, `port.Conflict`, `port.Tag`, `port.CheckoutState` | `internal/git`'s parsers, its anti-corruption layer (decision 17); output it cannot parse is a `*git.Failed`, an internal error |
| git output and exit codes | git's exit codes | the sealed `git.Error` set | `internal/git` (`exitCode`, `*Failed`/`*Missing`); `internal/cli` gives each a code |
| Environment | `ITOS_GIT`, `PATH`, `PATHEXT` | the git binary path (`git.Bin`) | `git.bin`/`names`/`executable`/`IsItos`: `ITOS_GIT` ignored when it names an itos, a `PATH` entry only when absolute |
| Environment | the repository-pointing `GIT_*` vars | the environment git runs in (`git.Environ`) | `git.Environ` drops `ownRepository` (a hook's `GIT_DIR`, etc.) |
| Environment | `runtime.GOOS` | `template.System` / the git binary | `template.SystemOf` / `git.bin`, the one edge read of it (decision 26) |

### Half (2): a check an inner layer makes again

A double check is a smell, not a security finding. Two stand:

- `internal/cli`'s `asker.Ask` has a `default` returning an error for a `port.Question` that is
  neither a `StackChoice` nor an `Answer`: a can't-happen branch on a sealed set Go cannot exhaust
  in a value switch returning a value. It cannot become a `panic` (rule 31 forbids one), so the
  error stays; the smell is inherent to the shape.
- A question's default is taken once when the manifest is read (`Question.DefaultAnswer`), and taken
  again by the asker's own check when a person accepts it on the terminal (`template.Choose` through
  `prompt`). Interactive only; the idea `p1-default-taken-once` would pass the taken default through.
