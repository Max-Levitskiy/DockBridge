# Implementation Plan

- [x] 1. Set up project structure and core infrastructure
  - Create Go module with proper directory structure following monorepo pattern
  - Initialize cmd/client and cmd/server entry points with basic CLI scaffolding
  - Set up internal package structure for client, server, shared, and pkg components
  - Configure go.mod with initial dependencies for Cobra, Viper, and testing frameworks
  - _Requirements: 8.1, 8.2_

- [x] 2. Implement configuration management system
  - Create configuration structs with YAML tags and validation in internal/client/config/
  - Implement Viper-based configuration loading with environment variable support
  - Write configuration validation functions with clear error messages for invalid settings
  - Create default configuration files (configs/client.yaml, configs/server.yaml)
  - Write unit tests for configuration loading and validation scenarios
  - _Requirements: 4.1, 4.2, 4.4_

- [x] 3. Build CLI framework and command structure
  - Implement Cobra-based CLI with init, start, config, server, and logs subcommands
  - Create CLI command handlers with proper flag definitions and help text
  - Implement configuration initialization workflow with guided setup
  - Add CLI commands for server management (create, destroy, status)
  - Write unit tests for CLI command parsing and execution
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [x] 4. Implement structured logging and error handling
  - Create custom error types with categories, codes, and retry flags in pkg/errors/
  - Implement structured logging system with configurable levels in pkg/logger/
  - Create error handling utilities with retry strategies and exponential backoff
  - Add logging integration throughout the system with proper context
  - Write unit tests for error handling and logging functionality
  - _Requirements: 7.1, 7.2, 7.3, 9.1, 9.3_

- [x] 5. Create SSH client wrapper and key management
  - Implement SSH client wrapper using golang.org/x/crypto/ssh in internal/client/ssh/
  - Create SSH key generation functions with RSA 4096-bit key support
  - Implement secure key storage and loading with proper file permissions
  - Add SSH tunnel creation and management for Docker API forwarding
  - Write unit tests for SSH operations and key management
  - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [x] 6. Build Hetzner Cloud API client
  - Implement Hetzner API client wrapper using hcloud-go library in internal/client/hetzner/
  - Create server provisioning functions with Docker CE image and cloud-init scripts
  - Implement volume creation, attachment, and management operations
  - Add SSH key upload and management via Hetzner API
  - Create server lifecycle management with proper cleanup procedures
  - Write unit tests with mocked Hetzner API responses
  - _Requirements: 1.3, 4.1, 4.2, 4.3, 6.1, 6.2, 6.3, 6.4_

- [x] 7. Refactor to Docker Go client approach
- [x] 7.1 Remove overcomplicated HTTP proxy layer
  - Delete proxy.go, connection_manager.go, request_handler.go files
  - Remove complex connection pooling and HTTP request forwarding logic
  - Eliminate server_manager.go complexity and simplify server management
  - Clean up unused interfaces and abstraction layers
  - _Requirements: 10.1, 10.2, *10*.3, 10.4_

- [x] 7.2 Implement Docker Go client manager
  - Create simplified DockerClientManager using github.com/docker/docker/client
  - Implement direct Docker API calls over SSH tunnel using client.Client
  - Add context-aware operations for proper cancellation and timeouts
  - Create simple connection management without complex pooling
  - _Requirements: 1.1, 1.2, 1.4_

- [x] 7.3 Fix docker run streaming issues
  - Implement proper streaming support for docker run commands using Docker client
  - Add real-time output streaming without buffering or freezing
  - Handle container attach operations with proper TTY support
  - Test docker run with interactive and non-interactive containers
  - _Requirements: 1.1, 1.4_

- [x] 7.4 Simplify server provisioning integration
  - Update server provisioning to work with Docker client instead of HTTP proxy
  - Remove server-side HTTP handler and use direct Docker daemon access
  - Simplify SSH tunnel creation for Docker client connections
  - Add automatic server provisioning when Docker client connection fails
  - _Requirements: 1.3_

- [ ] 8. Build cross-platform screen lock detection
  - Create base lock detector interface in internal/client/lockdetection/
  - Implement Linux-specific lock detection using D-Bus screensaver monitoring
  - Implement macOS-specific lock detection using Core Graphics session state
  - Implement Windows-specific lock detection using Win32 API desktop switching
  - Add lock event channel and proper event handling
  - Write unit tests for each platform-specific implementation
  - _Requirements: 2.1, 2.2, 2.4_

- [ ] 9. Implement client-side keep-alive system
  - Create keep-alive client in internal/client/keepalive/ with periodic heartbeat sending
  - Implement exponential backoff retry logic for network failures
  - Add integration with lock detector for status reporting
  - Create connection recovery mechanisms with proper error handling
  - Implement graceful shutdown coordination with server
  - Write unit tests for keep-alive client functionality
  - _Requirements: 3.1, 3.4, 9.1, 9.2_

- [ ] 10. Simplify server-side components
  - Remove complex server-side Docker handler and HTTP server implementation
  - Create simple bash script for keep-alive monitoring instead of Go service
  - Update cloud-init scripts to only setup Docker daemon and keep-alive script
  - Eliminate server-side Go code and complex request handling
  - Test direct Docker daemon access via SSH tunnel without proxy layer
  - Write integration tests for simplified server setup
  - _Requirements: 1.1, 1.2, 10.1, 10.2, 10.3, 10.4_

- [ ] 11. Implement simplified keep-alive monitoring
  - Create simple bash script for server-side keep-alive monitoring
  - Implement file-based heartbeat mechanism instead of complex HTTP endpoints
  - Add timeout detection with simple file timestamp checking
  - Create integration with server shutdown via systemd or cron
  - Deploy keep-alive script via cloud-init during server provisioning
  - Write integration tests for simplified keep-alive mechanism
  - _Requirements: 3.2, 3.3_

- [ ] 12. Simplify server lifecycle management
  - Remove complex server-side lifecycle manager and implement client-side management
  - Implement volume detachment procedures from client before server termination
  - Add server destruction capability via Hetzner API calls from client
  - Create simple shutdown mechanism triggered by keep-alive timeout
  - Implement proper cleanup procedures managed from client side
  - Write unit tests for simplified lifecycle management scenarios
  - _Requirements: 2.2, 2.3, 2.4, 3.2, 3.3, 6.2, 6.3_

- [ ] 13. Add network failure recovery and resilience
  - Implement connection retry logic with exponential backoff throughout the system
  - Create offline mode functionality with operation queuing
  - Add circuit breaker patterns for external service calls
  - Implement connection health checks and automatic recovery
  - Create network failure detection and graceful degradation
  - Write unit tests for network failure scenarios and recovery
  - _Requirements: 9.1, 9.2, 9.3, 9.4_

- [ ] 14. Build comprehensive integration tests
  - Create integration test suite that provisions real Hetzner servers
  - Implement end-to-end Docker command testing (run, build, pull, push)
  - Add tests for complete user workflows from initialization to cleanup
  - Create performance benchmarks for Docker command latency
  - Implement concurrent operation testing with multiple Docker commands
  - Add security tests for SSH key management and encrypted communication
  - _Requirements: 1.1, 1.2, 5.2, 5.3, 10.1, 10.3_

- [ ] 15. Implement persistent volume management
  - Add volume persistence logic that survives server recreations
  - Create volume reattachment procedures for new server instances
  - Implement volume expansion support and management
  - Add proper error handling for volume operations with retry logic
  - Create volume cleanup procedures for cost optimization
  - Write unit tests for volume management scenarios
  - _Requirements: 6.1, 6.2, 6.3, 6.4_

- [ ] 16. Add comprehensive logging and monitoring
  - Implement real-time log streaming via CLI commands
  - Create system health check endpoints and status reporting
  - Add performance metrics collection for Docker operations
  - Implement log aggregation and structured output formatting
  - Create debugging utilities and diagnostic information collection
  - Write unit tests for logging and monitoring functionality
  - _Requirements: 7.1, 7.2, 7.3, 7.4_

- [ ] 17. Create final integration and system testing
  - Implement complete system integration tests with all components
  - Create user acceptance test scenarios covering all major workflows
  - Add stress testing for concurrent operations and resource limits
  - Implement security validation tests for all communication channels
  - Create performance regression tests and benchmarking
  - Add final end-to-end validation of all requirements
  - _Requirements: All requirements validation_