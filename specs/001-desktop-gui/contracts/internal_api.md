# Internal API: GUI <-> Daemon

The GUI communicates with the local DockBridge Daemon using the existing internal gRPC API.

**Service Definition**: `pkg/api/v1/dockbridge.proto` (implied)

## Methods Used

### `GetStatus`
- **Request**: `{}`
- **Response**: 
    - `server_status` (Enum: STOPPED, PROVISIONING, RUNNING, STOPPING, ERROR)
    - `server_id` (string)
    - `server_ip` (string)
    - `error_message` (string)
- **Usage**: Polled on startup and used for manual refresh.

### `WatchStatus` (Streaming)
- **Request**: `{}`
- **Response**: Stream of `StatusResponse` (same as GetStatus)
- **Usage**: Real-time status updates in the System Tray and Main Window.

### `StartServer`
- **Request**: `{}`
- **Response**: 
    - `success` (bool)
    - `error` (string)
- **Usage**: Triggered by "Start" button.

### `StopServer`
- **Request**: `{}`
- **Response**: 
    - `success` (bool)
    - `error` (string)
- **Usage**: Triggered by "Stop" button.
