# DockBridge Product Overview

DockBridge is a sophisticated Go-based system that automatically provisions Hetzner Cloud servers for Docker containers with intelligent laptop lock detection and keep-alive mechanisms.

## Core Functionality

- **Docker Socket Proxying**: Intercepts local Docker commands and forwards them to remote Hetzner Cloud instances
- **Automatic Provisioning**: Creates Hetzner Cloud servers on-demand when Docker commands are executed
- **Smart Lifecycle Management**: Monitors laptop lock state and implements keep-alive mechanisms to manage server lifecycle
- **Cost Optimization**: Automatically destroys servers when not in use to minimize cloud costs

## Architecture

The system consists of two main components:
- **Client**: Runs locally, manages Docker socket proxying, screen lock detection, and keep-alive messaging
- **Server**: Runs on Hetzner Cloud instances, receives Docker commands via HTTP and manages server lifecycle

## Key Benefits

- Seamless Docker experience with cloud-based execution
- Automatic cost optimization through intelligent server management
- Cross-platform support (Linux, macOS, Windows)
- Persistent volume support for data retention