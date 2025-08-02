# Requirements Document

## Introduction

This specification outlines the migration of DockBridge to use the ssh-docker-proxy as a library and refactoring the project structure to follow Go best practices. The goal is to replace the current Docker daemon implementation with the proven ssh-docker-proxy library while improving the overall project structure and maintainability.

## Requirements

### Requirement 1

**User Story:** As a developer, I want DockBridge to use the ssh-docker-proxy library with lazy connection handling, so that I can leverage the tested SSH proxy functionality without failing when servers don't exist yet.

#### Acceptance Criteria

1. WHEN DockBridge starts THEN it SHALL create a local proxy socket immediately without requiring a remote server to exist
2. WHEN a user runs `dockbridge start` THEN the system SHALL start listening for Docker connections but NOT establish SSH connection until needed
3. WHEN the first Docker client connects THEN DockBridge SHALL attempt to establish the SSH proxy connection
4. WHEN DockBridge shuts down THEN it SHALL properly clean up the SSH proxy connection and socket
5. WHEN SSH connection fails THEN DockBridge SHALL trigger server provisioning hooks before retrying

### Requirement 2

**User Story:** As a developer, I want the project structure to follow Go best practices, so that the codebase is maintainable and follows community standards.

#### Acceptance Criteria

1. WHEN examining the project structure THEN it SHALL follow standard Go project layout conventions
2. WHEN looking at package names THEN they SHALL be descriptive and follow Go naming conventions (not generic names like "internal")
3. WHEN reviewing the code organization THEN related functionality SHALL be grouped in appropriate packages
4. WHEN checking imports THEN they SHALL follow Go best practices for internal vs external packages

### Requirement 3

**User Story:** As a developer, I want to remove the old Docker daemon implementation, so that the codebase is clean and doesn't have duplicate functionality.

#### Acceptance Criteria

1. WHEN reviewing the codebase THEN the old Docker daemon proxy implementation SHALL be removed
2. WHEN checking for unused code THEN all legacy Docker connection code SHALL be cleaned up
3. WHEN examining dependencies THEN unused Docker-related dependencies SHALL be removed
4. WHEN running tests THEN all tests SHALL pass with the new implementation

### Requirement 4

**User Story:** As a developer, I want comprehensive tests for the migration, so that I can ensure the refactoring doesn't break existing functionality.

#### Acceptance Criteria

1. WHEN implementing changes THEN each step SHALL have corresponding tests
2. WHEN refactoring code THEN existing functionality SHALL be preserved and tested
3. WHEN adding new integration THEN it SHALL be covered by unit and integration tests
4. WHEN running the full test suite THEN all tests SHALL pass

### Requirement 5

**User Story:** As a developer, I want the migration to be done incrementally with frequent commits, so that changes are reviewable and reversible.

#### Acceptance Criteria

1. WHEN making changes THEN each logical step SHALL be committed separately
2. WHEN refactoring THEN commits SHALL be small and focused on single concerns
3. WHEN implementing new features THEN they SHALL be added incrementally with tests
4. WHEN reviewing git history THEN the progression SHALL be clear and logical

### Requirement 6

**User Story:** As a developer, I want a connection lifecycle hook system that separates concerns between SSH proxy and server provisioning, so that the SSH proxy library remains focused and DockBridge handles server management.

#### Acceptance Criteria

1. WHEN SSH proxy encounters connection errors THEN it SHALL trigger registered connection failure hooks
2. WHEN connection is successfully established THEN it SHALL trigger connection success hooks
3. WHEN connection is lost during operation THEN it SHALL trigger connection lost hooks
4. WHEN hooks are triggered THEN they SHALL run without blocking the SSH proxy operation
5. WHEN multiple hooks are registered THEN they SHALL be executed in registration order
6. WHEN hooks fail THEN the failure SHALL be logged but not affect SSH proxy operation

### Requirement 7

**User Story:** As a user, I want DockBridge to automatically provision Hetzner servers on-demand using connection lifecycle hooks, so that I only pay for compute resources when I'm actually using them.

#### Acceptance Criteria

1. WHEN SSH proxy connection fails THEN DockBridge SHALL trigger connection failure hooks
2. WHEN connection failure hooks detect missing server THEN DockBridge SHALL automatically provision a new server
3. WHEN connection failure hooks detect destroyed server THEN DockBridge SHALL recreate the server
4. WHEN the server is provisioned THEN DockBridge SHALL retry the SSH proxy connection
5. WHEN multiple Docker clients connect concurrently THEN they SHALL be handled in separate goroutines
6. WHEN server provisioning is in progress THEN subsequent Docker connections SHALL wait for completion
7. WHEN the server is no longer needed THEN DockBridge SHALL automatically destroy it to save costs

### Requirement 8

**User Story:** As a user, I want DockBridge functionality to remain the same after the migration, so that my workflow is not disrupted.

#### Acceptance Criteria

1. WHEN running `dockbridge start` THEN it SHALL work exactly as before
2. WHEN using DockBridge commands THEN they SHALL have the same behavior and output
3. WHEN checking configuration options THEN they SHALL remain compatible
4. WHEN using existing config files THEN they SHALL continue to work without modification