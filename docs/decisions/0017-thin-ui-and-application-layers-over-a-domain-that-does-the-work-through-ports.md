---
status: accepted
date: 2026-10-07
---

# Thin UI and application layers over a domain that does the work, through ports

## Context and Problem Statement

Within the vertical slices, how are the layers held apart, and where do errors become exit codes? Recommended: DDD with thin UI and application layers. A slice is one directory (one file until it grows): its kong struct and Run (the UI) and its CQRS command or query with its handler (the application), the handler only wiring. The shared domain does the work, reaching git and files only through interfaces (ports) in port subpackages; infra implements those ports and imports nothing else of ours. Commands carry domain objects; the only copy left is Run assembling a command from kong flags. The domain errors are sealed sets of plain domain types with no exit code; the UI alone turns them into exit codes, gochecksumtype refusing a switch that leaves one out. depguard holds every arrow. Or: decision 16 as T-8 built it, exit codes set where an error is made, the domain driving no ports.

Asked as q-17.

## Considered Options

- Thin UI and app, the domain behind ports, errors mapped to exit codes in the UI
- Decision 16 as T-8 built it: exit codes set where an error is made

## Decision Outcome

DDD in vertical slices with thin UI and application layers, the work in the domain. A slice is one directory, one file until it grows: its kong struct and Run (UI) and its CQRS command or query and handler (application); kong stays in the UI. The shared domain reaches the outside only through ports in port subpackages; infra implements only those, importing nothing else of ours. Commands carry domain objects; Run assembling a command from kong flags is the one copy left, accepted as kong's cost. Domain errors are part of the domain, sealed sets of domain types with no UI concern; the UI turns them and infra's into exit codes, held by gochecksumtype; only an error nobody classified, a bug, exits 70. The user's call, 2026-10-07.

### Consequences

Supersedes decision 16, keeping its vertical slices and its no-mappers rule: a slice still holds its command end to end, and no slice imports another. The layout: cmd/itos-template assembles; internal/cli is the UI shared by slices (errors to exit codes, printing, --json); a slice holds UI and application; the shared domain packages and their port subpackages; infra (git, tempdir, prompt) implementing ports. depguard holds the arrows: the domain imports no infra, slice, UI, kong or I/O package; infra imports only port packages; a slice no other slice; kong only the UI. gochecksumtype holds the exit-code switches. docs/CLI.md rule 31 changes to match. The code is reshaped as task T-9; how it is tested is decision 18.

## More Information

Supersedes ADR-0016.
