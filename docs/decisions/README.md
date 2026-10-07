# Decisions

The decisions that still stand, one record each. A record superseded keeps its
file and leaves this list, which itos decision record writes.

<!-- itos:decisions:begin -->

- [ADR-0001: Templates are real projects, and answers replace literals](0001-templates-are-real-projects-and-answers-replace-literals.md)
- [ADR-0002: Optional parts are git branches merged when a project is made](0002-optional-parts-are-git-branches-merged-when-a-project-is-made.md)
- [ADR-0003: Updates are a 3-way merge of the old and new renders](0003-updates-are-a-3-way-merge-of-the-old-and-new-renders.md)
- [ADR-0004: Go and kong, an itos extension, its infrastructure harvested from itos](0004-go-and-kong-an-itos-extension-its-infrastructure-harvested-from-itos.md)
- [ADR-0007: The template drives a made project's setup: steps in the manifest, run once trusted](0007-the-template-drives-a-made-project-s-setup-steps-in-the-manifest-run-once-trusted.md)
- [ADR-0008: One manifest, itos-template.yaml, on the root branch and merged down into every branch](0008-one-manifest-itos-template-yaml-on-the-root-branch-and-merged-down-into-every-branch.md)
- [ADR-0009: A template is anything git clone takes, fetched by git](0009-a-template-is-anything-git-clone-takes-fetched-by-git.md)
- [ADR-0010: A made project records its render in .itos-template.yaml, each branch's commit included](0010-a-made-project-records-its-render-in-itos-template-yaml-each-branch-s-commit-included.md)
- [ADR-0011: Missing answers are asked on a terminal, and refused without one unless --defaults](0011-missing-answers-are-asked-on-a-terminal-and-refused-without-one-unless-defaults.md)
- [ADR-0012: new writes into a missing or empty folder and commits the render as its first commit](0012-new-writes-into-a-missing-or-empty-folder-and-commits-the-render-as-its-first-commit.md)
- [ADR-0013: Look for a library before building what one likely already solves](0013-look-for-a-library-before-building-what-one-likely-already-solves.md)
- [ADR-0014: check renders every combination the manifest allows, less those it lists as unsupported](0014-check-renders-every-combination-the-manifest-allows-less-those-it-lists-as-unsupported.md)
- [ADR-0015: A template's checks are lists of words run with no shell](0015-a-template-s-checks-are-lists-of-words-run-with-no-shell.md)
- [ADR-0017: Thin UI and application layers over a domain that does the work, through ports](0017-thin-ui-and-application-layers-over-a-domain-that-does-the-work-through-ports.md)
- [ADR-0019: The domain is unit-tested with fakes and stubs, never mocks; the scenarios hold the rest against the real git](0019-the-domain-is-unit-tested-with-fakes-and-stubs-never-mocks-the-scenarios-hold-the-rest-against-the-real-git.md)
- [ADR-0020: The domain gets property-based tests with rapid: one case per push, many nightly](0020-the-domain-gets-property-based-tests-with-rapid-one-case-per-push-many-nightly.md)
- [ADR-0021: A template's own checks scan its renders for credentials; check scans for leftover literals](0021-a-template-s-own-checks-scan-its-renders-for-credentials-check-scans-for-leftover-literals.md)
- [ADR-0022: Configuration is JSON data written as YAML, read strictly, its conventions shown](0022-configuration-is-json-data-written-as-yaml-read-strictly-its-conventions-shown.md)
- [ADR-0023: A release cuts a patch when the binary's linked modules or its Go toolchain changed](0023-a-release-cuts-a-patch-when-the-binary-s-linked-modules-or-its-go-toolchain-changed.md)

<!-- itos:decisions:end -->
