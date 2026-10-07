---
status: accepted
date: 2026-10-06
---

# Updates are a 3-way merge of the old and new renders

## Context and Problem Statement

How does a project made from the template take a newer template release?

Asked as q-3.

## Considered Options

The options are those the question names.

## Decision Outcome

As copier and cruft do: the project records the tag, the stack, the features and the answers; an update renders the old tag and the new one with the same answers and merges their difference into the project, the old render the base of a 3-way merge, landing as an ordinary commit through the project's gates. The user's call, 2026-10-06.

### Consequences

None recorded.
