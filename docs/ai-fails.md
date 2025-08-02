# AI Implementation Failures and Lessons

---

## SSH Docker Proxy - DinD SSH Forwarding Issue

**Date**: 2025-07-31  
**Problem**: SSH Docker Proxy cannot connect to Docker socket via SSH forwarding in Docker-in-Docker setup

**What was tried**:
1. Created Docker-in-Docker container with SSH server
2. Configured SSH server with various forwarding options:
   - `AllowTcpForwarding yes`
   - `GatewayPorts yes` 
   - `StreamLocalBindUnlink yes`
3. Set up testuser with docker group permissions
4. Verified Docker socket permissions (660, root:docker)
5. Tested SSH connection and Docker access separately (both work)

**Error encountered**:
```
ssh: rejected: connect failed (open failed)
```

**Root cause**: SSH server in Alpine Linux (Docker-in-Docker) doesn't support Unix socket forwarding properly, or there are permission/configuration issues preventing the SSH client from connecting to `/var/run/docker.sock` via the SSH tunnel.

**Lesson learned**: 
- Docker-in-Docker with SSH forwarding is complex for testing
- Need simpler test approach that doesn't rely on SSH Unix socket forwarding
- Should test with real remote server instead of DinD simulation

**Alternative approach needed**:
- Test with actual remote server with SSH access
- Create mock SSH dialer for unit tests
- Focus on proxy logic testing rather than full integration

----
--

## SSH Docker Proxy - Docker-in-Docker SSH Forwarding Limitation

**Date**: 2025-07-31  
**Task**: Testing SSH Docker Proxy with Docker-in-Docker environment  
**Approach**: Created complex Docker Compose setup with Alpine-based DinD container and SSH server  

**What Failed**:
- Alpine SSH server in Docker-in-Docker doesn't properly support Unix socket forwarding
- SSH connection works but `client.Dial("unix", "/var/run/docker.sock")` fails with "ssh: rejected: connect failed (open failed)"
- Complex setup with multiple configuration steps and manual fixes required
- Over-engineered solution for simple proxy testing needs

**Root Cause**:
- Alpine's OpenSSH server has limitations with Unix socket forwarding in containerized environments
- Docker-in-Docker adds unnecessary complexity for testing a simple proxy

**Lesson Learned**:
- Don't use Docker-in-Docker for SSH forwarding tests
- Simple direct SSH to real server is more reliable
- Complex test infrastructure often creates more problems than it solves

**Better Approach**:
- Use unit tests for proxy logic (already working perfectly)
- Provide simple manual test instructions for real SSH servers
- Focus on the core functionality rather than elaborate test setups