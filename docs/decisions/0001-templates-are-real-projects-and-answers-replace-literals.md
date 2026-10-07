---
status: accepted
date: 2026-10-06
---

# Templates are real projects, and answers replace literals

## Context and Problem Statement

How does a template hold the parts a project fills in, given that Jinja2 and Go templates make the template something that neither builds nor tests?

Asked as q-1.

## Considered Options

The options are those the question names.

## Decision Outcome

No template language. The template is a real, working project using distinctive literal values (a name, a module path, a type); a manifest maps each literal to a question, and rendering rewrites the answers into file contents and names with their case forms, reading no language's syntax, so it works for any stack. Prior art: dotnet new and gonew, each tied to its stack. The user's calls, 2026-10-06.

### Consequences

None recorded.
