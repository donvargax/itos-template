# First template root bootstrap

T-25 bootstrapped only the root of the public `donvargax/go-template-itos` repository. Target task
T-1 is done, and GitHub reports `main` at
`ed5798615ccbb98e405f3f45e62a465b99027fb6`. The CI run for that exact head passed:
<https://github.com/donvargax/go-template-itos/actions/runs/37888658513>.

The root carries fresh itos task data, MIT licensing for the illustrative `itos-corp` owner, the
manifest v4 literals, shared Git files, the documentation cap check, the owner's Renovate base
preset and a root-only workflow. The manifest plans `go` and `go + cli`; its name, owner and module
questions have no defaults. Root CI checks itos configuration, documentation caps and required root
files. The checks do not require the template manifest, which is excluded from made projects.

The target has no Go module or Go code. The Go stack, CLI, template combination checks and pinned
credential scan remain later pieces. README calls this root unfinished; it does not claim that the
template is ready to render or that a scanner is active.

The README and CONTRIBUTORS file record the owner's authorization to distribute their copied
infrastructure under MIT. The original upstream identities and third-party licenses remain intact.
