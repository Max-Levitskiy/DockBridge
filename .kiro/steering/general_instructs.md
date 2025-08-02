<!------------------------------------------------------------------------------------
   Add Rules to this file or a short description and have Kiro refine them for you:   
-------------------------------------------------------------------------------------> 
Add this always.
Be lazy, don't create unnesessary code, logs, scripts, etc. Only if it's really nececcary.
If you try some approach and it failed, append information about  it in docs/ai-fails.md with --- new section delimeter. And revert failed approach.
Track success and unsuccess commands, and add it in the in docs/cli-commands.md like "do" and "don't". For don't, increment amount of times it happened.
Be DRY and SOLID.
Search for existing code, before implement anything, what looks general.
ALWAYS think how human can test your task. Start with defining the "demo" strategy, and finish with telling how to validate your work.
## 
CLI Commands - Do's and Don'ts

### Do:
- `make build` - Simple, reliable build command
- `go test ./...` - Run all tests efficiently  
- `unset DOCKER_HOST` - Reset Docker host before Docker Compose operations
- Use environment variables for test configuration (SSH_HOST, SSH_USER, SSH_KEY)
- Test SSH connection separately before testing proxy: `ssh -i key user@host 'docker ps'`
- Handle `~` path expansion in file paths using `os.UserHomeDir()` and `filepath.Join()`
- Use unit tests + simple manual instructions instead of complex test infrastructure
- Always add default ports to network addresses (e.g., SSH host without port should default to :22)
- Use `unix://` prefix for Docker Unix sockets: `export DOCKER_HOST=unix:///path/to/socket`

### Don't:
- Don't rely on Docker-in-Docker for SSH forwarding tests (Alpine SSH server limitations)
- Don't use complex Docker Compose setups for simple proxy testing
- Don't assume SSH Unix socket forwarding works in all environments
- Don't create overly complex test infrastructure when simple approaches work better
- Don't over-engineer test setups when unit tests + manual instructions suffice
- Don't delete existing test files, even if they're placeholder stubs - they show intended test structure
- Recreate accidentally deleted test files with proper placeholder implementations and clear TODOs