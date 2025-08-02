Feature: SSH Docker Proxy
  As a developer
  I want to run Docker commands locally that execute on a remote server via SSH
  So that I can leverage remote compute resources while maintaining my familiar local workflow

  Scenario: Basic proxy functionality
    Given I have a valid SSH configuration
    And the remote Docker daemon is accessible
    When I start the proxy
    Then the proxy should create a local Unix socket
    And the proxy should perform a health check
    And the proxy should be ready to accept connections

  Scenario: Docker command forwarding
    Given the proxy is running
    When I run "docker ps" against the proxy socket
    Then the command should be forwarded to the remote Docker daemon
    And I should receive the response unchanged

  Scenario: Configuration validation
    Given I have invalid SSH configuration
    When I try to start the proxy
    Then the proxy should return a configuration error
    And the proxy should not start

  Scenario: SSH connection failure
    Given I have valid configuration but SSH server is unreachable
    When I try to start the proxy
    Then the proxy should return an SSH connection error
    And the proxy should not start

  Scenario: Docker daemon health check failure
    Given I have valid SSH configuration
    But the remote Docker daemon is not accessible
    When I try to start the proxy
    Then the proxy should return a Docker health check error
    And the proxy should not start