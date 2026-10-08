@phase-1
Feature: The command line
  The rules docs/CLI.md sets for every command, held by the built binary.
  The harness stamps the binary it builds with a known version and commit,
  as a release's build is stamped with the release's, so a scenario can
  tell the stamp from the dev version of a build without one.

  @ID-CLI-01 @T-3
  Scenario: --version prints the version stamped at build
    When itos-template runs with "--version"
    Then it exits with code 0
    And the first line of its standard output is "itos-template" and the stamped version

  # slice-9 (version-command): docs/CLI.md's rule 11 asks for version beside
  # --version, and docs/template-contents.md's CLI contract the build's commit
  # with its version. The commit is the one a release's build stamps
  # (GoReleaser's FullCommit, set with -X as the version is), else the
  # vcs.revision Go records for a build in a checkout, else there is none and
  # no second line. The harness stamps a known commit, as it stamps the
  # version. version takes no argument and prints exactly what --version does.
  @ID-CLI-02 @slice-9
  Scenario: version prints what --version prints
    When itos-template runs with "version"
    Then it exits with code 0
    And the first line of its standard output is "itos-template" and the stamped version
    And the second line of its standard output is "commit" and the stamped commit

  @ID-CLI-03 @slice-9
  Scenario: --version prints the build's commit on its second line
    When itos-template runs with "--version"
    Then it exits with code 0
    And the second line of its standard output is "commit" and the stamped commit

  # slice-10 (usage-errors): docs/CLI.md's rules 12, 20 and 21. A usage error
  # exits 2 and names what was wrong, on the error output, in words a person
  # reads (never kong's own, such as "EOL"). kong gives an unknown command
  # with the one meant, an unknown flag and a flag's value never read as a
  # flag already (@ID-CLI-04, 05 and 07 pass before the work); a flag given
  # no value says kong's words, and a once-only flag given twice or a switch
  # given a value (--defaults=yes) are taken, the last value winning. A flag
  # that may repeat (--feature, --answer) still does.
  @ID-CLI-04 @slice-10 @wip
  Scenario: an unknown command exits 2, naming it and the command meant
    When itos-template runs with "chek"
    Then it exits with code 2
    And its error output says "chek"
    And its error output says "check"

  @ID-CLI-05 @slice-10 @wip
  Scenario: an unknown flag exits 2, naming it
    When itos-template runs with "check --bogus"
    Then it exits with code 2
    And its error output says "--bogus"

  @ID-CLI-06 @slice-10 @wip
  Scenario: a flag given no value exits 2, naming it in a person's words
    When itos-template runs with "new acme made --stack"
    Then it exits with code 2
    And its error output says "--stack"
    And its error output does not say "EOL"

  @ID-CLI-07 @slice-10 @wip
  Scenario: a flag's value is never read as a flag
    When itos-template runs with "new acme made --stack --defaults"
    Then it exits with code 2
    And its error output says "--stack"

  @ID-CLI-08 @slice-10 @wip
  Scenario: a switch given a value exits 2, naming it
    When itos-template runs with "new acme made --stack go --defaults=yes"
    Then it exits with code 2
    And its error output says "--defaults"

  @ID-CLI-09 @slice-10 @wip
  Scenario: a once-only flag given twice exits 2, naming it
    When itos-template runs with "new acme made --stack go --stack python"
    Then it exits with code 2
    And its error output says "--stack"
