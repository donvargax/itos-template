@phase-1
Feature: The command line
  The rules docs/CLI.md sets for every command, held by the built binary.
  The harness stamps the binary it builds with a known version, as a
  release's build is stamped with the release's, so a scenario can tell
  the stamp from the dev version of a build without one.

  @ID-CLI-01 @T-3 @wip
  Scenario: --version prints the version stamped at build
    When itos-template runs with "--version"
    Then it exits with code 0
    And the first line of its standard output is "itos-template" and the stamped version
