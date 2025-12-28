# Data Model

## Configuration Entities

The GUI interacts primarily with the `ClientConfig` model defined in `client/config`.

### `HetznerConfig`
| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| `api_token` | string | Hetzner Cloud API Token | Required |
| `server_type` | string | Instance type (e.g., cpx21) | Enumerated (cx11..cx51) |
| `location` | string | Datacenter location | Enumerated (fsn1, nbg1...) |
| `volume_size` | int | Size of persistent volume in GB | 10-10000 |

### `SSHConfig`
| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| `key_path` | string | Path to SSH private key | Directory must exist |
| `port` | int | SSH port | 1-65535 |
| `timeout` | duration | Connection timeout | Min 1s |

### `DockerConfig`
| Field | Type | Description | Validation |
|-------|------|-------------|------------|
| `socket_path` | string | Local socket path | Directory writable |

## Application State (`AppState`)

The GUI maintains ephemeral state for the view layer.

### `ServerStatus` (Enum)
- `Stopped`: Daemon is not running or server is down.
- `Provisioning`: Server is starting.
- `Running`: Server is active and connected.
- `Stopping`: Server is shutting down.
- `Error`: Connection failed.

### `State` Struct
| Field | Type | Description |
|-------|------|-------------|
| `Status` | ServerStatus | Current connection status |
| `DaemonConnected` | bool | Verification of gRPC link |
| `ServerIp` | string | Remote IP address |
| `Error` | error | Last error message |
