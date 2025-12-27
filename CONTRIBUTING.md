# Contributing to DockBridge

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

```bash
git clone https://github.com/dockbridge/dockbridge
cd dockbridge
go mod download
go test ./...
```

## Making Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Add tests for new functionality
- Keep commits atomic and well-described

## Areas for Contribution

- **Cloud Providers**: Add support for AWS, GCP, Azure
- **Idle Detection**: Improve activity tracking (macOS lock detection, etc.)
- **Windows Support**: Port Unix socket handling
- **Documentation**: Examples, tutorials, troubleshooting guides
- **Testing**: Integration tests, edge cases

## Questions?

Open an issue for discussion before starting major work.
