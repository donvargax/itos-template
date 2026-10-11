---
status: accepted
date: 2026-10-11
---

# Secure by design: input checked at its edge into value objects, trusted inside, never checked again

## Context and Problem Statement

Where is input checked, and what do inner layers trust: secure by design for itos-template?

Asked as q-56.

## Considered Options

The options are those the question names.

## Decision Outcome

Secure by design. Every input from outside is checked where it enters, at the outer layers that meet other systems: the command line and the terminal, the template repository (its manifest, paths and contents), the project's record, git's output, and the environment. There it becomes a checked value, a value object with unexported fields whose one constructor checks it, so holding one proves it was checked. Inner layers trust those values and never check them again. Bad input is refused with an error naming it; a lossless normalization (path separators, case forms) is fine, a silent repair is not. It holds because everything lives under internal/, which no other module can import: nothing outside reaches an inner layer. The person's call, 2026-10-11.

### Consequences

T-52 removes the one re-check that could panic (Replacements) and forbids panic in production code. T-53 audits every edge (an input reaching inside unchecked is a security finding; a check made twice is a smell) and maps edge, input, checked value and where it is checked in docs/design-rules.md. No linter decides checked from unchecked, so the rule is mostly judgement, held in part by the strict YAML decoder rule and the panic rule. A practice for any Go CLI, so it goes to go-template-itos by issue once it lands.
