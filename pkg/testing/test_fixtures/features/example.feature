Feature: access.smi-spec.io
  In order to test the testing library
  As a developer
  I need to ensure the specification is executed by godog

  Scenario: Test custom step
    Given I have a running blueprint
    Then I expect a step to be called