---
status: accepted
date: 2026-10-07
---

# Configuration is JSON data written as YAML, read strictly, its conventions shown

## Context and Problem Statement

How are configuration formats designed, the manifest and the record among them? Recommended: convention over configuration made visible (defaults written down and printable) and explicit over implicit; JSON as the data model and YAML as its syntax in one .yaml file, so a tool may write JSON there; a strict reader refusing what JSON cannot say (custom tags, several documents, anchors, aliases, merge keys, non-string keys, duplicate keys), since a tag can build any object, an alias can expand without bound, and a manifest comes from any template git can clone; no language of our own, a configuration language writing JSON on the author's side meeting repetition; rules gathered in docs/CONFIG.md from the Kubernetes API conventions, Google's AIPs, CLIG, JSON and YAML 1.2, and JSON Schema. Or: allow anchors and aliases with expansion limits, to spare repetition.

Asked as q-22.

## Considered Options

- JSON data, YAML syntax, a strict reader
- Allow anchors and aliases with expansion limits

## Decision Outcome

As recommended. Convention over configuration belongs in code; a configuration file is explicit, its conventions shown. YAML for its being JSON's superset, read strictly; repetition is a configuration language's job, the author's own if they like, never ours. The user's calls, 2026-10-07.

### Consequences

docs/CONFIG.md holds the rules, each naming its source, as docs/CLI.md holds the CLI's. The ideas strict-yaml (the reader, best before or with slice-4's manifest version 3), config-key-paths, config-print-defaults and config-schema carry the rules not yet followed.
