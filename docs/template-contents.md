# What a made project contains

What a project started from the user's template has on day one, gathered on 2026-10-06 from two
briefs the user wrote for an earlier Python service (a FastAPI Copier template, and dependency
auto-merge). The briefs are gone; what holds beyond that project is here. CLI comes first; the
service sections wait. How the template is built and updated is `PLAN.md`. Item IDs below (`p1-…`, `p3-…`, `T-108`,
`q-13`) are itos's (github.com/donvargax/itos), where that work is tracked.

## Principles

- **Selection at generation time, never runtime indirection.** A provider (package index,
  registry, cloud, deployment, CI) is chosen when the project is made, as a feature branch of the
  template. No generated code exists only to tell one provider from another; the application reads
  ordinary environment variables and standard URLs.
- **The template is a real project.** Every branch builds and passes its own checks; answers
  replace literals last.
- **Few combinations, all tested.** The template's CI renders every combination it allows and
  verifies each; the rest are listed as unsupported, never claimed.
- **No credentials** in answers, generated files, image layers, lockfiles, fixtures or logs. Every
  render is scanned for leaks.
- **The docs say what is.** Each doc separates what is implemented, what is an optional provider,
  what is the platform's job, and what is future work; nothing claims a scan, SBOM or attestation
  the workflow does not produce.
- **Standard protocols over vendor SDKs; simple projects over a clever template.**

## Every project

- **itos from the first commit:** the config, the hooks, the ledger and registry, scenarios as the
  acceptance tests, decision records. itos's scenario rules are the guard against weakening an
  acceptance test, a workflow control, not a security boundary.
- **Acceptance tests drive the public entry points only** (the binary, the HTTP API), never the
  internals. Unit tests beneath them, behaviour first, mock-free where a real dependency is cheap.
- **Locked installs everywhere:** the lockfile committed and checked (`go mod verify` and
  `-mod=readonly`; `uv sync --locked`); scaffolding fails clearly when no correct lockfile can be
  made.
- **Pinned tools:** explicit versions, never `latest`, each download checked against a SHA-256 or
  signed provenance, with where the checksum came from and how to upgrade it written down.
- **Renovate** (itos's `p1-bot-dependency-prs`): semantic `chore(deps):` commits, a release-age cooldown
  agreeing with itos's deps-check, rebase auto-merge only when every required check is green on
  the PR's head, actions pinned by digest.
- **CI in phases:** locked install; lint and format; types; tests and coverage; build; optional
  scans; publishing apart from deployment. Pushes run the fast, deterministic gates; full mutation
  testing, load tests and deep provenance checks are scheduled runs, recommended and not claimed
  until they exist.
- **Quality:** lint, types, tests, coverage, a SAST baseline, complexity (CRAP), mutation testing
  of the changed code (enforced as below), advisory test-structure analysis; extra scanners as documented extension
  points, never an organization's fixed list.
- **A devcontainer**, optional (itos's T-108).

## Code quality, enforced rather than advised

itos-cc (CRAP, mutation testing, duplication; the extension `itos cc`) is advice today, not a gate.
The design that makes it one, agreed in parts (2026-10-03 to 2026-10-06) and still open where
marked, applies to every project the template makes:

- **Mutation on what changed.** The agent runs `itos cc mutate` on the functions it changed. The
  commit hook runs nothing slow: it checks that itos-cc's cache holds results for exactly the staged
  code, by a hash of each changed function and of the tests that ran, refusing missing or stale
  ones. CI re-runs the changed functions' mutants, fully or as a random sample, refusing a commit
  whose recorded results do not hold, so forging them is a bad bet. Per function, since itos-cc's
  annotation comment would change a file's hash. Needs donvargax/itos-cc#8: a check mode that runs
  nothing, the tests' hash in the cache, a sample mode for CI.
- **Debt is a committed baseline.** itos-cc keeps `itos-cc-debt.yaml`; its range check refuses a
  new violation the baseline lacks, so listing debt is cheap and measuring stays a late CI step
  (`p3-debt-role`, through one role protocol for every provider, `p3-role-protocol`).
- **Debt is owned.** A work item claims debt entries (`pays:`, by ID or glob); itos refuses an
  entry no open item claims and an item closed while it still claims one; entries older than a
  start point are exempt, so adopting it in an existing codebase costs nothing up front
  (`p3-debt-claims`).
- **Proof of done by type** (the v7 design, open: q-13): a task is proven by a provider's evidence
  (itos-cc's mutation results for the code it touched, the scenarios it names) rather than a
  hand-written `grep`; `done_when` holds acceptance only, run while the task is open; lasting
  guarantees come from standing rules itos enforces in every project (a coverage threshold, q-6;
  every gate and tool exercised by a self-test; every command, path, scenario and decision the docs
  name resolving).
- **A new project starts clean:** its baseline empty, so every violation from the first commit is
  either fixed or claimed by an item.
- **Docs:** a quick start, a configuration reference, an architecture overview, a security and
  trust-boundary guide, an upgrade guide (versions, digests, checksums), decision records.

## CLI (first)

- **Go:** kong for commands and flags (struct tags, enums, a `--no-` pair for every switch, an
  environment variable per flag); configuration from flags, then environment, then a file, typed
  and validated at start. Python: pydantic-settings' CLI with uv.
- **A CLI contract:** exit codes documented and stable; the main output on stdout, logs and
  errors on stderr; `--json` printing one object, failures included; `--version` with the build's
  version and commit. this repository's `docs/CLI.md` is the starting point.
- **Structured logs** (JSON when not on a terminal), quiet by default.
- **Acceptance tests run the built binary** (godog for Go), on Linux, macOS and Windows in CI.
- **Releases cut by CI** from Conventional Commits: GoReleaser builds, checksums and attests;
  notes generated; the version in one place.
- **Shell completion** generated.
- golangci-lint and govulncheck in the gates.

## Service (later)

- **Application:** an app factory with a lifespan; vertical feature slices; typed settings
  validated for production (CORS among them); consistent error responses; graceful cleanup.
- **Health:** shallow `/health/live` and `/health/ready`; deep diagnostics apart
  (`/health/status`); no shared dependency in continuous readiness.
- **Observability:** JSON logs in production, request IDs, OpenTelemetry resource metadata and
  request spans, 5xx spans marked as errors, the deployed revision in the metadata.
- **Database, when chosen:** bounded pools, an acquisition timeout, recycling, pre-ping, all
  configurable; migrations first, with replay tests and an explicit migration command. The
  application repository owns the revisions; the deployment runs them as an ordered pre-deploy
  job, its mechanism an adapter (no orchestrator assumed in the code).
- **Container:** multi-stage; a minimal runtime with no development tools; a non-root user with a
  fixed UID and GID; read-only root compatible; dependencies installed from the lockfile;
  base images pinned by tag and digest where the registry can resolve them, with an update path,
  never a digest copied from upstream for a mirror.
- **Kubernetes, when chosen:** non-root, RuntimeDefault seccomp, no privilege escalation, every
  capability dropped, read-only root with a bounded writable `/tmp`, requests and limits,
  startup, liveness and readiness probes, service-account token mounting configurable, workload
  identity annotations by provider, an immutable image input; no gateway, ingress, DNS, namespace
  or secret manager assumed. Plain manifests, Kustomize or Helm as branches.
- **Load tests, if scheduled:** warm-up, stable data, concurrency, duration, p95 and p99 latency
  and error-rate thresholds, and who owns the baseline.

## The template's own checks

For every combination it allows: the files present and absent as chosen, the locked install, its
tests, migrations replayed where chosen, the container built or validated, the deployment output
rendered and validated, the leak scan, and the docs' commands run as written. Each render's
required CI workflows are exactly those it contains, so an auto-merge never waits on a workflow
the project lacks.
