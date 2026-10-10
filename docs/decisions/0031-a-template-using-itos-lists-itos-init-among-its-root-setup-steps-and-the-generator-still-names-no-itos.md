---
status: superseded by ADR-0034
date: 2026-10-09
---

# A template using itos lists itos init among its root setup steps, and the generator still names no itos

## Context and Problem Statement

A template that uses itos lists itos init among its root setup steps, so the person runs it themselves and the generator still names no itos. ADR-0007 already decides that setup steps come from the manifest, are shown and asked, or run under --trust, and that the generator knows no itos; what it does not say is what an itos-using template should list. Recommended: the root lists itos init --agent-rules, beside itos hook install, so the itos setup a made project needs is the template's declaration rather than a message new prints. Or: new prints an unconditional hint naming the command, which is generator code naming itos and reverses ADR-0007. Or: the manifest's first_commit carries the footer as it does today, naming a task the made project does not have yet. Which?

Asked as q-40, about new-itos-setup.

## Considered Options

- The template lists itos init --agent-rules as a root setup step, per ADR-0007's mechanism
- new prints an unconditional hint naming the command, which reverses ADR-0007
- The manifest's first_commit keeps its Task footer

## Decision Outcome

The template's declaration, through the mechanism ADR-0007 already decided: a template using itos lists itos init --agent-rules among its root setup steps, beside itos hook install, so the person runs it themselves and sees it first, and the generator still names no itos. new-setup has to be built before that can be said, since the mechanism is an idea today. ADR-0007 is not superseded and this record does not amend it. What it adds is what an itos-using template lists, and the consequence that the target's manifest drops the Task footer from its first_commit: the render is committed before the project has a ledger, so a footer there names nothing, and T-1 in a made project means Adopt itos.

### Consequences

The itos setup a made project needs is the template's own declaration, so it is reviewed with the template, shown to the person before it runs, and absent from a template that does not use itos. The generator keeps naming no itos, which is ADR-0007's rule and the reason itos-template can be run on its own. new-setup is the mechanism and it is an idea today, so this cannot be built before it is promoted and specified; the slice that depends on it proves only what new-setup does not, that a made project left with the step run is a sound itos project and what happens when the step cannot run. The target's manifest drops the Task footer from first_commit with T-28, since a render is committed before the project has a ledger and a footer there names nothing. A step is a shell command, so under --trust on a machine without itos the step fails; the render stands and new says so, which the slice pins. Nothing here requires a template to use itos: a template that does not lists no such step and its made projects get no itos at all.
