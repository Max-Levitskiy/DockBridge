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
---


## DockBridge - Nil Pointer Dereference in convertServer

**Date**: 2025-08-03  
**Problem**: DockBridge client crashes with panic when provisioning new server

**Error encountered**:
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x2 addr=0x48 pc=0x102fb303c]
goroutine 50 [running]:
github.com/dockbridge/dockbridge/client/hetzner.convertServer(0x0)
/Users/max/git/Max-Levitskiy/DockBridge/client/hetzner/utils.go:12 +0x1c
```

**Root cause**: The `convertServer` function in both `client/hetzner/utils.go` and `internal/client/hetzner/utils.go` didn't check for nil input before accessing server fields. When the Hetzner API returns a nil server (which can happen during server provisioning), the function would panic trying to access `server.PublicNet.IPv4.IP`.

**What was fixed**:
1. Added nil check at the beginning of `convertServer` function in both locations
2. Added proper error handling in `ProvisionServer` method to check for nil server before conversion
3. Updated `ListServers` method to filter out any nil servers defensively
4. Added test cases to verify nil handling works correctly

**Files modified**:
- `client/hetzner/utils.go` - Added nil check in `convertServer`
- `internal/client/hetzner/utils.go` - Added nil check in `convertServer`  
- `client/hetzner/client.go` - Added nil checks in `ProvisionServer` and `ListServers`
- `internal/client/hetzner/client.go` - Added nil checks in `ProvisionServer` and `ListServers`
- `client/hetzner/client_test.go` - Added `TestConvertServerNil` test case
- `internal/client/hetzner/client_test.go` - Added `TestConvertServerNil` test case

**Lesson learned**: 
- Always validate input parameters, especially pointers, before dereferencing them
- API responses can be nil even when the call succeeds, especially during resource provisioning
- Defensive programming prevents crashes and provides better error messages
- Add test cases for edge cases like nil inputs