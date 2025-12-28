# Research & Technical Decisions

## Decisions

### 1. GUI Framework: Fyne v2
- **Decision**: Use `fyne.io/fyne/v2` for the cross-platform GUI.
- **Rationale**: 
    - True cross-platform support (macOS, Windows, Linux) from a single codebase.
    - System tray support provided out-of-the-box.
    - Go-native, fitting the team's expertise and project language.
    - Lightweight compared to Electron.
- **Alternatives**: 
    - **Electron**: Rejected due to binary size and memory overhead.
    - **Wails**: Good alternative (Go backend + Web frontend), but Fyne offers pure Go UI which is simpler for this specific "control panel" type app.

### 2. Configuration Management: Shared `client/config`
- **Decision**: Reuse the existing `client/config` package (Viper-based).
- **Rationale**: 
    - Ensures consistency between CLI and GUI.
    - Leverages existing validation logic.
    - Single source of truth for file paths and defaults.

### 3. Architecture: gRPC Controller
- **Decision**: GUI communicates with the DockBridge Daemon via gRPC.
- **Rationale**: 
    - The daemon runs as a background service; GUI is a client.
    - gRPC provides strong typing and existing definitions (`server/grpc`).
    - Decouples UI from the long-running process logic.

### 4. Integration: `dockbridge gui` Command
- **Decision**: Launch GUI via a new `gui` subcommand in the main `dockbridge` CLI.
- **Rationale**: 
    - Maintains a single binary distribution.
    - Allows user to choose mode (CLI vs GUI).

## Implementation Gaps Identified

### 1. Windows CI/CD
- **Status**: Deployment workflow (`release.yml`) currently builds for Linux and macOS only.
- **Action**: Must add `GOOS=windows` build step to `release.yml`.
- **Note**: Check if `-H=windowsgui` ldflag is needed to hide console window.

### 2. CLI Integration
- **Status**: `cmd/dockbridge` and `client/cli` do not currently have a `gui` command.
- **Action**: Implement `client/cli/gui.go` to wrap `gui.Run()`.

### 3. Fyne Packaging
- **Status**: `Taskfile.yaml` and `release.yml` use standard `go build`.
- **Action**: Research if `fyne package` is strictly required for distribution (e.g. for .app bundles on macOS to support icons/dock correctly). Standard `go build` works but lacks polish.

## Testing Strategy

### 1. TDD with `fyne.io/fyne/v2/test`
- **Decision**: Use `fyne.io/fyne/v2/test` for unit testing GUI components.
- **Rationale**:
    - Allows headless testing of UI logic (vital for CI).
    - Supports simulating events (`Tap`, `Type`) and asserting state (`AssertImageMatches`, `AssertRendersToMarkup`).
    - Enables verifying logic without launching a full window manager.
- **Approach**:
    - Write tests *before* implementation for Widgets and Windows.
    - Use `test.NewApp()` to create lightweight test instances.
    - Mock the `Controller` dependencies (gRPC) to test UI states (Error, Loading, Connected).
