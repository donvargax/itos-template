---
status: accepted
date: 2026-10-07
---

# Domain-driven design in vertical slices, with no mappers

## Context and Problem Statement

How is the code put together? Recommended: domain-driven design in vertical slices. Each command (new, check, later update and sync) is a slice, one package and one file holding its kong command, its use case, its output and the I/O only it needs, split into more files only when large or complex; slices never import each other. What slices share is the domain model in the ubiquitous language (manifest, stack, feature, combination, question, answer, literal, case form, render, record), in packages named for what they model, and the adapters several use (git, temporary folders, the terminal). No mappers: the kong command is the slice input, and domain types are read and written as the user sees them (the manifest, the record, the report, --json); an anti-corruption layer only where an outside model would leak in (git output, a scanner findings). Or: layers as folders (domain, app, infra) with mappers at their edges.

Asked as q-16.

## Considered Options

- Vertical slices over a shared domain, no mappers
- Layers as folders, mappers at their edges

## Decision Outcome

Domain-driven design in vertical slices, each in one file until it grows large or complex; no mappers, the domain exposed to the user unless something requires an anti-corruption layer. A gate holds the slices apart: golangci-lint depguard refuses a slice importing another, dropped if it gets in the way too often. The user's call, 2026-10-07.

### Consequences

The code is reshaped to it as task T-8 (folding the flag-to-Options copies of new and check into their kong commands, the record struct into a domain Record, and the render check borrows from new into the shared domain). depguard in .golangci.yml refuses a slice package importing another. A limit on a file's length (revive's file-length-limit) waits until after T-8, to pick a number from the code and to see whether slices want a file per layer. AGENTS.md carries the no-mappers rule, which no command can check.
