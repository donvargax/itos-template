---
status: accepted
date: 2026-10-10
---

# update leaves its merge staged and commits nothing, so the project's own gates judge it

## Context and Problem Statement

Does update make the commit itself? Decision 3 has it land as an ordinary commit through the project's gates, and a made itos project's chore needs a Task footer itos-template cannot know. Recommended: update leaves the merge and the new record in the working tree and the index, staged, and prints a message naming the old and new releases for the person to commit through their own gates, as adopt leaves its record. Or: update commits when nothing conflicts, its message from a manifest key as first_commit; or update always commits, conflict markers included.

Asked as q-46, about update-merges.

## Considered Options

- Leave it staged, a message printed for the person to commit
- Commit when nothing conflicts, the message from a manifest key
- Always commit, conflict markers included

## Decision Outcome

Leave it staged: update writes the merge and the new record to the working tree and the index, commits nothing, and prints a message naming the old and new releases for the person to commit through their own gates. The person's call, 2026-10-10.

### Consequences

The made project's commit rules, itos's footers among them, judge an update as any other commit, and itos-template needs to know none of them. adopt leaves its record the same way (slice-12). sync, which works in a template rather than a made project, is free to choose otherwise, but should say why.
