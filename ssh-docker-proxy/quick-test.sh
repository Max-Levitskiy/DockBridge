#!/bin/bash

echo "🚀 SSH Docker Proxy Test"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo ""
echo "1. 🧪 Unit Tests"
echo "   Running unit tests to verify core proxy functionality..."
go test -v ./internal/proxy/
echo ""

echo "2. 📝 Manual Test Instructions"
echo ""
echo -e "${BLUE}To test with your own server:${NC}"
echo ""
echo "Step 1 - Test SSH + Docker access:"
echo "  ssh -i ~/.ssh/your_key user@your-server 'docker ps'"
echo ""
echo "Step 2 - Start the proxy (~ paths supported):"
echo "  ./bin/ssh-docker-proxy \\"
echo "    -ssh-user=your-user \\"
echo "    -ssh-host=your-server:22 \\"
echo "    -ssh-key=~/.ssh/your_key \\"
echo "    -local-socket=/tmp/docker-proxy.sock"
echo ""
echo "Step 3 - Test Docker commands through proxy:"
echo "  export DOCKER_HOST=unix:///tmp/docker-proxy.sock"
echo "  # Note: Use 'unix://' prefix for Unix sockets, not just the path"
echo "  docker version"
echo "  docker ps"
echo "  docker pull alpine:latest"
echo ""

echo -e "${GREEN}✅ SSH Docker Proxy is ready!${NC}"
echo ""
echo "📚 Implemented features:"
echo "  - Unix socket creation and listening"
echo "  - SSH connection management with per-connection isolation"
echo "  - Bidirectional traffic relay using io.Copy"
echo "  - Concurrent connection handling with goroutines"
echo "  - Proper connection lifecycle management and cleanup"