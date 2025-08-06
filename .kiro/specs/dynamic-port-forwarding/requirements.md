# Requirements Document

## Introduction

Dynamic Port Forwarding is a feature that automatically detects when Docker containers expose ports and creates SSH port forwards to make those services accessible on localhost. This enables developers to access remote containerized services as if they were running locally, providing a seamless development experience where `curl localhost:80` works against remote nginx containers.

## Requirements

### Requirement 1

**User Story:** As a developer, I want ports exposed by remote Docker containers to be automatically forwarded to my local machine, so that I can access remote services using localhost URLs.

#### Acceptance Criteria

1. WHEN a Docker container is started with exposed ports (e.g., `docker run -p 80:80 nginx`) THEN the system SHALL automatically create local TCP listeners that proxy traffic to the remote container ports via the existing SSH connection
2. WHEN multiple containers expose the same port THEN the system SHALL handle port conflicts by using available local ports and providing clear mapping information
3. WHEN a container exposes multiple ports THEN the system SHALL create proxy listeners for all exposed ports
4. WHEN port forwarding is established THEN the system SHALL log the port mapping (e.g., "localhost:8080 -> remote_container:80")
5. WHEN accessing forwarded ports THEN traffic SHALL be proxied through DockBridge using the existing SSH connection to maintain security

### Requirement 2

**User Story:** As a developer, I want port forwards to be automatically cleaned up when containers stop, so that local ports are freed and don't accumulate over time.

#### Acceptance Criteria

1. WHEN a Docker container stops or is removed THEN the system SHALL automatically close the corresponding SSH port forwards
2. WHEN checking for cleanup THEN the system SHALL verify container status lazily (only when port access is attempted or during periodic checks)
3. WHEN a container is no longer running THEN the system SHALL remove port forwards DockBridge get request to this port
4. WHEN port forwards are removed THEN the system SHALL log the cleanup action
5. WHEN cleanup fails THEN the system SHALL retry with exponential backoff up to 3 times

### Requirement 3

**User Story:** As a developer, I want to see which ports are currently forwarded, so that I can understand how to access my remote services.

#### Acceptance Criteria

1. WHEN running a status command THEN the system SHALL display all active port forwards with container information
2. WHEN displaying port forwards THEN the system SHALL show local port, remote port, container name, and container status
3. WHEN a port forward is inactive THEN the system SHALL indicate the status and reason (e.g., container stopped)
4. WHEN no port forwards are active THEN the system SHALL display a clear "no active forwards" message
5. WHEN port conflicts occur THEN the system SHALL show both requested and actual local ports used

### Requirement 4

**User Story:** As a developer, I want port forwarding to work with Docker Compose services, so that multi-container applications are accessible locally.

#### Acceptance Criteria

1. WHEN Docker Compose services expose ports THEN the system SHALL forward all service ports automatically
2. WHEN Compose services use named networks THEN the system SHALL handle inter-service communication correctly
3. WHEN Compose services are scaled THEN the system SHALL forward ports for all service instances with unique local ports
4. WHEN Compose services are stopped THEN the system SHALL clean up all related port forwards
5. WHEN Compose services restart THEN the system SHALL re-establish port forwards automatically

### Requirement 5

**User Story:** As a developer, I want local port conflicts to be handled gracefully with Docker-compatible error responses, so that I get familiar error messages when ports are already in use.

#### Acceptance Criteria

1. WHEN a requested local port is already in use THEN the system SHALL return a Docker-compatible error response (e.g., "port is already allocated")
2. WHEN detecting port conflicts THEN the system SHALL check if the local port is bound by any process
3. WHEN port conflicts occur THEN the error message SHALL indicate that the port is used on the local machine (not remote)
4. WHEN Docker clients receive port conflict errors THEN they SHALL behave exactly as if running Docker locally with the same port conflict
5. WHEN automatic port assignment is enabled THEN the system SHALL find the next available port and update the container's port mapping

### Requirement 6

**User Story:** As a developer, I want to configure port forwarding behavior, so that I can customize which ports are forwarded and how conflicts are handled.

#### Acceptance Criteria

1. WHEN configuring the system THEN I SHALL be able to specify port ranges for automatic forwarding (e.g., 8000-9000)
2. WHEN configuring the system THEN I SHALL be able to exclude specific ports from forwarding
3. WHEN configuring the system THEN I SHALL be able to set custom local port mappings for specific containers
4. WHEN port conflicts occur THEN the system SHALL use the configured conflict resolution strategy (increment, fail, or custom mapping)
5. WHEN configuration changes THEN the system SHALL apply changes to new containers without requiring restart

### Requirement 7

**User Story:** As a developer, I want port forwarding to handle connection failures gracefully, so that temporary network issues don't break my development workflow.

#### Acceptance Criteria

1. WHEN proxy connections fail THEN the system SHALL retry with exponential backoff using the existing SSH connection
2. WHEN the SSH connection is lost THEN the system SHALL attempt to re-establish port proxies when connectivity is restored
3. WHEN local ports become unavailable THEN the system SHALL find alternative ports and update mappings
4. WHEN remote containers become unreachable THEN the system SHALL mark proxies as inactive but retain configuration for reconnection
5. WHEN connection issues persist THEN the system SHALL provide clear error messages and suggested actions

### Requirement 8

**User Story:** As a developer, I want port forwarding to integrate seamlessly with the existing DockBridge lifecycle, so that it works automatically without additional setup.

#### Acceptance Criteria

1. WHEN DockBridge starts THEN port forwarding SHALL be enabled by default
2. WHEN servers are provisioned THEN port forwarding SHALL be ready to handle container ports immediately
3. WHEN servers are destroyed THEN all port forwards SHALL be cleaned up automatically
4. WHEN DockBridge is idle THEN port forwarding SHALL not prevent server shutdown
5. WHEN DockBridge resumes THEN port forwarding SHALL re-establish forwards for running containers

### Requirement 9

**User Story:** As a developer, I want efficient port forwarding that doesn't impact performance, so that my development workflow remains fast and responsive.

#### Acceptance Criteria

1. WHEN establishing port proxies THEN the overhead SHALL be minimal (under 50ms per proxy)
2. WHEN proxying traffic THEN latency SHALL be comparable to direct SSH port forwarding
3. WHEN monitoring containers THEN the system SHALL use efficient polling (maximum every 30 seconds)
4. WHEN handling multiple proxies THEN the system SHALL process them concurrently
5. WHEN the system is idle THEN port forwarding SHALL use minimal CPU and memory resources

### Requirement 10

**User Story:** As a developer, I want port forwarding to work with interactive and streaming Docker commands, so that development tools like live reload work correctly.

#### Acceptance Criteria

1. WHEN containers serve WebSocket connections THEN port proxying SHALL maintain persistent connections
2. WHEN containers serve Server-Sent Events THEN streaming SHALL work without buffering delays
3. WHEN containers serve file uploads THEN large transfers SHALL complete successfully
4. WHEN containers require bidirectional communication THEN full-duplex proxying SHALL be maintained
5. WHEN containers use HTTP/2 or HTTP/3 THEN protocol features SHALL work transparently

### Requirement 11

**User Story:** As a developer, I want to manually control port forwarding when needed, so that I can override automatic behavior for specific use cases.

#### Acceptance Criteria

1. WHEN running CLI commands THEN I SHALL be able to manually add port proxies for specific containers
2. WHEN running CLI commands THEN I SHALL be able to remove specific port proxies
3. WHEN running CLI commands THEN I SHALL be able to temporarily disable automatic port forwarding
4. WHEN manual proxies are created THEN they SHALL persist until explicitly removed or container stops
5. WHEN manual and automatic proxies conflict THEN manual proxies SHALL take precedence