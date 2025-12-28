# Tasks: Desktop GUI

**Feature**: `001-desktop-gui`
**Status**: Generated
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Implementation Strategy

- **Phase 1: Setup**: Initialize package structure and state management.
- **Phase 2: Foundational**: Core application logic and CLI integration.
- **Phase 3: System Tray (US1)**: Implement tray icon and menu (Priority P1).
- **Phase 4: Configuration (US2)**: Implement settings window and config persistence (Priority P1).
- **Phase 5: Control (US3)**: Implement main window and VM/Proxy controls (Priority P1).
- **Phase 6: Cross-Platform (US4)**: Ensure consistent experience across OSs (Priority P2).
- **Phase 7: Polish**: Final verification and cleanup.

**Note**: This plan emphasizes TDD using `fyne.io/fyne/v2/test` as requested.

## Phase 1: Setup (Project Initialization)

*Goal: Establish the directory structure and shared data models.*

- [ ] T001 Create/Verify `gui/` directory structure and `gui/resources.go` for assets
- [ ] T002 Initialize `gui/state.go` with `AppState` and `ServerStatus` definitions

## Phase 2: Foundational (Blocking Prerequisites)

*Goal: Core app wiring and controller logic.*

- [ ] T003 Create `gui/app.go` with `FyneApp` struct and constructor
- [ ] T004 Implement `gui/controller.go` with gRPC connection logic and mock interface
- [ ] T005 Create `client/cli/gui.go` to integrate GUI launch into main CLI

## Phase 3: System Tray (US1)

*Goal: Application runs in background with status indication.*

**Dependencies**: Phase 2

- [ ] T006 [P] [US1] Create `gui/tray_test.go` to verify menu item creation and status updates
- [ ] T007 [US1] Implement `gui/tray.go` with `SetupTray` function and menu definitions
- [ ] T008 [US1] Integrate `Tray` into `gui/app.go` lifecycle

## Phase 4: Configuration Management (US2)

*Goal: GUI-based configuration editing.*

**Dependencies**: Phase 2

- [ ] T009 [P] [US2] Create `gui/window_settings_test.go` to verify form validation and save logic
- [ ] T010 [US2] Implement `gui/window_settings.go` with Fyne form widgets
- [ ] T011 [US2] Wire `SettingsWindow` to `client/config` for persistence in `gui/controller.go`

## Phase 5: VM and Proxy Control (US3)

*Goal: Main dashboard for controlling the remote environment.*

**Dependencies**: Phase 4 (Config needed for connection)

- [ ] T012 [P] [US3] Create `gui/window_main_test.go` to verify button states and status display
- [ ] T013 [US3] Implement `gui/window_main.go` with Start/Stop/Connect buttons
- [ ] T014 [US3] Connect `MainWindow` actions to `Controller` methods

## Phase 6: Cross-Platform Experience (US4)

*Goal: Native look and feel on target OSs.*

**Dependencies**: Phase 3, 5

- [ ] T015 [US4] Verify build tags and flags in `Taskfile.yaml` for Windows/Linux/macOS
- [ ] T016 [US4] Update `.github/workflows/release.yml` to include Windows build

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T017 Run full test suite `go test ./gui/...` and fix any regressions
- [ ] T018 Manual verification of full flow: Config -> Start -> Connect -> System Tray -> Quit
