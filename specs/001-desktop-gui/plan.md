# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

**Language/Version**: Go 1.24 (current project version)
**Primary Dependencies**: Fyne v2 (GUI), Cobra (CLI), Viper (Config), gRPC (Daemon communication)
**Storage**: YAML config file (`client/config`), persistent via Viper
**Testing**: `go test` for logic, Fyne test utilities for UI
**Target Platform**: macOS (Arm/Intel), Windows (amd64), Linux (amd64)
**Project Type**: Desktop GUI (integrated into existing CLI Client)
**Performance Goals**: UI responsiveness < 16ms (60fps), Daemon status updates < 1s
**Constraints**: Must run as a system tray application; Must integrate with existing `client/config` logic
**Scale/Scope**: ~10 screens/dialogs (Main Window, Settings, Tray Menu)

## Constitution Check

> [!NOTE]
> The `constitution.md` file is currently a template. Proceeding with standard Go and Fyne best practices as per the "Library-First" and "Test-First" implied principles.

*GATE: Implied best practices check passed.*

## Project Structure

### Documentation (this feature)

```text
specs/001-desktop-gui/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # API/Interface definitions
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
gui/
├── app.go               # Main Fyne application entry point
├── controller.go        # Logic bridging GUI and Daemon (gRPC)
├── resources.go         # Icons and static assets
├── state.go             # Application state management
├── tray.go              # System tray implementation
├── window_main.go       # Main dashboard window
└── window_settings.go   # Settings configuration window

client/
├── cli/
│   ├── gui.go           # [NEW] CLI command to launch GUI
│   └── ...
└── config/              # Existing config manager (used by GUI)

cmd/dockbridge/
└── main.go              # Main entry point
```

**Structure Decision**: The GUI logic resides in a dedicated `gui/` package at the root (following existing pattern) to separate UI concerns from the `client/` core logic. Integration occurs via `client/cli` which imports `gui`.

## Verification Plan

### Automated Tests
- **Unit Tests (TDD)**:
    - Use `fyne.io/fyne/v2/test` to verify UI components without a display server.
    - Test `Tray` menu updates by mocking `AppState` changes.
    - Test `SettingsWindow` form validation and persistence.
    - Run `go test ./gui/...` in CI to ensure headless compatibility.
- **Integration Tests**:
    - Verify `Controller` <-> `Daemon` gRPC communication using a mock daemon server.

### Manual Verification
- **Cross-Platform**:
    - Build and run on macOS (verify Dock icon and Menu Bar).
    - Build and run on Windows (verify System Tray and Minimize behavior).
- **Functional**:
    - Change settings in GUI -> Check `client.yaml`.
    - Click "Start Server" -> Verify daemon starts provisioning.
    - Quit app -> Verify process termination.
