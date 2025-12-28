# Feature Specification: Desktop GUI

**Feature Branch**: `001-desktop-gui`  
**Created**: 2025-12-28  
**Status**: Draft  
**Input**: User description: "create a simple and beautiful cross-platform gui interface, where we will be able to make configurations (the full set of configuration should be supported), show in tray with status, start/stop vm, etc.? it should be a part of our client app. Fyne can be used for it. We should have a packages for a multiple platforms, like apple, windows, linux."

## User Scenarios & Testing

### User Story 1 - System Tray Integration (Priority: P1)

As a user, I want the application to run in the system tray so that I can quickly check its status or access main controls without cluttering my taskbar.

**Why this priority**: Essential for a background service/proxy application to be unobtrusive but accessible.

**Independent Test**: Can be tested by launching the app and verifying the tray icon appears, shows status, and responds to clicks.

**Acceptance Scenarios**:

1. **Given** the app is running, **When** I look at the system tray, **Then** I see the DockBridge icon.
2. **Given** the app is connected, **When** the status changes, **Then** the tray icon visually indicates the "Connected" state (e.g., green dot).
3. **Given** the app is running, **When** I right-click the tray icon, **Then** I see a context menu with options like "Open Window" and "Quit".

---

### User Story 2 - Configuration Management (Priority: P1)

As a user, I want to configure all connection settings through a GUI so that I don't have to edit YAML files manually.

**Why this priority**: Core usability requirement; manual config editing is error-prone and unfriendly.

**Independent Test**: Can be tested by opening the settings window, changing values, saving, and verifying the config is updated.

**Acceptance Scenarios**:

1. **Given** the settings window is open, **When** I view the configuration form, **Then** I see fields for Local Socket, SSH User, SSH Host, SSH Key Path, Remote Socket, and Timeout.
2. **Given** I have modified a setting, **When** I click "Save", **Then** the configuration is persisted to disk and applied.
3. **Given** I enter invalid data (e.g., missing SSH Host), **When** I try to save, **Then** the system shows an error message.

---

### User Story 3 - VM and Proxy Control (Priority: P1)

As a user, I want to start/stop the remote VM and the proxy connection from the GUI so that I can manage resources easily.

**Why this priority**: The primary function of the client is to facilitate these connections.

**Independent Test**: Can be tested by clicking "Start" and verifying the VM boots/proxy connects.

**Acceptance Scenarios**:

1. **Given** the VM is stopped, **When** I click "Start VM", **Then** the system initiates the VM boot sequence and updates status to "Starting".
2. **Given** the proxy is disconnected, **When** I click "Connect", **Then** the SSH tunnel is established.
3. **Given** the VM is running, **When** I click "Stop VM", **Then** the VM is shut down.

---

### User Story 4 - Cross-Platform Experience (Priority: P2)

As a user, I want the app to look and feel native on my operating system (Windows, macOS, or Linux).

**Why this priority**: The user explicitly requested cross-platform support with a "beautiful" interface.

**Independent Test**: Build and run the artifact on different OS VMs.

**Acceptance Scenarios**:

1. **Given** I am on macOS, **When** I launch the app, **Then** it respects macOS window management and menu bar standards.
2. **Given** I am on Windows, **When** I launch the app, **Then** it minimizes to the system tray correctly.

## Requirements

### Functional Requirements

- **FR-001**: System MUST display an icon in the OS system tray/menu bar.
- **FR-002**: System MUST support a context menu on the tray icon with at least "Show App" and "Quit" options.
- **FR-003**: System MUST provide a main window with comprehensive configuration fields for:
    - Local Socket Path
    - SSH User
    - SSH Host
    - SSH Key Path
    - Remote Socket Path
    - Connection Timeout
- **FR-004**: System MUST validate configuration inputs before saving.
- **FR-005**: System MUST allow users to Start and Stop the remote Hetzner VM via the GUI.
- **FR-006**: System MUST allow users to Connect and Disconnect the SSH proxy via the GUI.
- **FR-007**: System MUST display real-time status of the connection (Connecting, Connected, Disconnected, Error).
- **FR-008**: System MUST persist configuration to the standard config file location.
- **FR-009**: System MUST be buildable for macOS, Windows, and Linux.
- **FR-010**: UI MUST be implemented using the Fyne toolkit.

### Key Entities

- **Config**: Stores connection details (SSH, Sockets).
- **ConnectionState**: Represents the current status of the proxy link.
- **InstanceStatus**: Represents the state of the remote VM (Running, Stopped).

## Success Criteria

### Measurable Outcomes

- **SC-001**: Application builds successfully on macOS (Arm/Intel), Windows (amd64), and Linux (amd64).
- **SC-002**: Configuration changes made in GUI are reflected in the `config.yaml` file on disk.
- **SC-003**: System tray icon updates status within 2 seconds of state change.
- **SC-004**: User can initiate a VM start command within 2 clicks from the main window.
