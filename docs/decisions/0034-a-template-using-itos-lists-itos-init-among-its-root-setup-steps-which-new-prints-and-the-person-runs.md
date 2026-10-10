---
status: accepted
date: 2026-10-09
---

# A template using itos lists itos init among its root setup steps, which new prints and the person runs

## Context and Problem Statement

new-itos-setup's ID-NEW-42 and 43 run new with --trust and expect the itos setup step to have run, or to have failed and been named, but new-steps only prints steps and nothing queued runs them. Recommended: the scenarios run the steps new printed themselves, as the person would, and judge the project with itos config check; ID-NEW-43 goes, since new runs nothing, and running under --trust becomes the idea new-trust, waiting until someone needs it. Or: a slice that runs the chosen branches' steps under --trust comes before new-itos-setup, which then keeps both scenarios. Which?

Asked as q-43, about new-itos-setup.

## Considered Options

- new-itos-setup's scenario runs the steps new printed, as the person would, and running them under --trust waits as the idea new-trust
- A slice making new run the chosen branches' steps under --trust comes first, and new-itos-setup keeps both its scenarios

## Decision Outcome

The scenarios run the steps new printed themselves, as the person would, and judge the project with itos config check; ID-NEW-43 goes, and running under --trust becomes the idea new-trust.

### Consequences

This keeps ADR-0031's decision whole and changes one consequence of it. A template using itos lists itos init --agent-rules among its root setup steps, beside itos hook install, so the itos setup a made project needs is the template's own declaration: reviewed with the template, shown to the person, and absent from a template that does not use itos, whose made projects get no itos at all. The generator still names no itos (ADR-0007), which is why itos-template can be run on its own. What changes is how the step reaches the project. new-steps prints the chosen branches' steps and runs none, so the person runs itos init themselves, after the render is committed; new-itos-setup's scenario does what the person would, running the printed steps in the made folder, and itos config check judges the project, its ledger holding T-1 Adopt itos. ADR-0031 said that under --trust a step that cannot run leaves the render and new names it, and that new-itos-setup pins it. That is no longer new's to say: new runs nothing, so a step that fails, on a machine without itos, fails in the person's own shell, and the scenario that pinned it is gone. Running steps under --trust is the idea new-trust, which settles that case if someone needs it. The made project's first commit still carries no footer (ADR-0033).

## More Information

Supersedes ADR-0031.
