# Quickstart: GUI Development

## Prerequisites

1.  **Go 1.24+**: Ensure Go is installed.
2.  **Fyne Dependencies**:
    -   **macOS**: XCode Command Line Tools (`xcode-select --install`)
    -   **Linux**: `sudo apt-get install gcc libgl1-mesa-dev xorg-dev`
    -   **Windows**: MSYS2 or TDM-GCC (for CGo)

## Running the GUI

> **Note**: Until the `gui` subcommand is merged, you may need to run the GUI entry point directly or use the `dockbridge gui` command if implemented.

```bash
# Run from source
go run cmd/dockbridge/main.go gui

# Or if developing within the gui package specifically (if it had a main)
# But currently it is a library package.
```

## TDD Example

Fyne provides a `test` package for headless verification.

```go
package gui

import (
    "testing"
    "fyne.io/fyne/v2/test"
    "github.com/stretchr/testify/assert"
)

func TestStatusText_Updates(t *testing.T) {
    // 1. Create headless app
    a := test.NewApp()
    
    // 2. Setup SUT (System Under Test) with mocks
    state := NewAppState()
    controller := &Controller{state: state} // Mock/Stub as needed
    tray := NewTray(nil, controller, state)
    tray.Setup(a)
    
    // 3. Act
    state.SetStatus(StatusRunning)
    
    // 4. Assert
    // Note: Checking specific menu items depends on your implementation
    // This is just a conceptual example.
    assert.Equal(t, StatusRunning, state.GetStatus())
}
```

## Testing

```bash
# Run unit tests
go test ./gui/...
```

## Building

```bash
# Build binary
go build -o bin/dockbridge ./cmd/dockbridge

# Run
./bin/dockbridge gui
```

## Key Files

- `gui/app.go`: Main composition root.
- `gui/window_main.go`: The dashboard UI.
- `gui/window_settings.go`: The configuration form.
