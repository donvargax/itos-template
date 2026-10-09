# First template CLI feature

Source task T-27 delivered target task T-3 and target slice `count-cli` on `go/cli`, branched from
the verified `stack/go` head without rewriting history. The target is at
`c2da5d2c96823a3b519daf7e9d8d6cfe8cc6e099`; T-3 and `count-cli` are done, and CI passed on that exact
head: <https://github.com/donvargax/go-template-itos/actions/runs/37935932750>.

The branch adds `cmd/go-template-itos`, a thin kong UI over the existing counting domain. The command
carries domain data and reaches the domain through its `Files` port; `internal/count/command` counts
nothing and knows no exit code, and `internal/count/input` is the one adapter that turns the `-` of a
command line into a reader behind that port. Human success prints `<file>: lines N, words M` on stdout
and the use-json hint on stderr and never the input's contents. `--json` prints one object with
`schema`, `ok`, `file`, `lines` and `words`, and is a negatable switch with `GO_TEMPLATE_ITOS_JSON`.
The error kind becomes an exit code in the UI's sealed switch alone, so `COUNT_INPUT_MISSING` and
`COUNT_INPUT_UNREADABLE` exit 3, a usage failure exits 2, and a kind no case classifies is 70.

Twenty-nine black-box scenarios across `features/count.feature`, `features/cli.feature` and
`features/completion.feature` were specified by the coordinator before the work and turned green
red-first: every one failed at the step that checks its behaviour with a stub binary, then passed. The
harness builds and stamps the binary once per run and runs it through godog on Linux, macOS and
Windows. GoReleaser, the reusable release workflow and `internal/release` came with it, retargeted to
this module and binary path, and the release workflow is `workflow_call` only: no job on a template
branch calls it, since a rendered project's main has the complete release plan.

The source check `itos task T-27` passed against the public branch. It verified the target config,
documentation caps, the entry point and release files, the smoke set, module verification and tidy
state, the scenario kind, all Go tests, `internal/count/domain` coverage at 100%, and
`go run ./cmd/go-template-itos count README.md --json`, which answers
`{"schema":1,"ok":true,"file":"README.md","lines":56,"words":414}`.

The code proof covers `cmd` and `internal`. `tools/bin/pinned itos-cc mutation check --fail-uncovered
--json cmd internal` exits 0 with `ok: true`: eleven files, every function `fresh`, 210 killed, 0
survived, 0 uncovered, and 4 exceptions in `itos-cc.yaml`, each holding the hashed function so a reason
cannot outlive the code it was written for. No gate was weakened: `.golangci.yml` is untouched, gosec
still runs, and the harvest's eleven gosec findings were fixed by lowering file modes and giving a
reason to the five sites that name a variable on purpose.

The run was recorded against base `018c227`, the take commit, where `AGENTS.md` asks for the parent of
the item's actual first commit. That deviation changes nothing here and is worth saying why: the two
bases judge the same twenty-two Go files, because every commit between them is the item's
specification and none of them touches Go code. Both bases leave `internal/count/{disk,domain,port}`
unre-judged, which is right, since the CLI item did not change the foundation; its records from
`stack/go` still stand. The check was re-run against the base the rule names, `ce622fb`, and passes
identically.

## The layout this branch assumes, and does not match

ADR-0029 puts a template's itos setup on one branch beside its render root, with no render starting
from it, so a made project runs `itos init` and inherits no itos data. `go-template-itos` does not
have that layout yet: its `main` is the render root and also carries `itos.yaml`, `tasks/` and the
generated rules block, and every branch carries that data. This branch therefore built and proved its
CLI under the old layout. The restructure is separate work and is tracked as its own item; nothing here
claims it is done.

## Still to come

The template is not advertised as ready. The render-check piece, which renders every combination the
manifest allows and runs the checks it names, is still to come, and the manifest still names no
checks. Credential scanning arrives with it. The `first-template` idea stays open.