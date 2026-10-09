# First template Go foundation

Source task T-26 delivered target task T-2 on `stack/go`. The target is at
`ce622fbe7898fac6b15a785fad1781930ba0fb00`; T-2 is done, and CI passed on that exact head:
<https://github.com/donvargax/go-template-itos/actions/runs/37896765904>.

The branch adds module `github.com/itos-corp/go-template-itos` with toolchain Go 1.27.2. Its
private counting domain reads through a typed `Files` port; `internal/count/disk` reads real files,
and `internal/count/port/porttest` supplies an in-memory fake. Tests cover LF and CRLF logical lines,
unterminated input, Unicode whitespace, invalid UTF-8, punctuation, long lines and newline-terminated
additive pieces. The project uses Rapid for the property and does not include the later Kong CLI or
Godog scenarios.

The source check `itos task T-26` passed against the public branch. It verified the target config,
documentation caps, module and toolchain pins, module verification and tidy state, all Go tests, and
100% coverage for `internal/count/domain`. The implementation head `ba59df203deb38e5bb1a57d21b2e9eac76a021a4`
passed CI's Go build, vet, lint, vulnerability, test, coverage and mutation-sampling steps, with
tests on Linux, macOS and Windows:
<https://github.com/donvargax/go-template-itos/actions/runs/37896331336>.

Mutation proof used base `ed5798615ccbb98e405f3f45e62a465b99027fb6`, the parent of T-2's first commit.
It records 11 killed mutation sites, zero survivors, zero uncovered sites and zero exceptions. Ten
mutants ran in the final all-tests pass; one unchanged disk-read mutant was reused with a matching
code and test hash. The proof covers `internal/`, the Go code present on this branch. `cmd/` joins
when the CLI exists; no empty placeholder was added.

The template remains unfinished. The public CLI and scenario runner, rendered-combination checks,
and credential scanning belong to later work; the `first-template` idea stays open.
