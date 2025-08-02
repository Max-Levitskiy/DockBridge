# Building an SSH-based Docker “socket proxy” in Go

Below is a practical blueprint for a small Go program that:

1. Listens locally on a Unix socket (or TCP port) that **any Docker-compatible CLI or SDK can target**.  
2. Establishes an **SSH tunnel** to a remote host running Docker.  
3. **Relays raw HTTP traffic** between the local client and the remote Docker Engine API, so every Docker feature keeps working.  
4. Uses the **official Docker Go SDK** whenever it adds value (e.g., version negotiation, ping/health checks), but avoids re-implementing low-level HTTP proxying logic.

## Why a pure HTTP proxy is enough

The Docker CLI (and every SDK) ultimately speaks the **Docker Engine HTTP API**. When you run:

```bash
docker -H unix:///var/run/docker.sock ps
```

the CLI simply opens the Unix socket, sends an HTTP GET `/vX.Y/containers/json`, and reads the JSON response. Therefore, if we can forward arbitrary bytes **unchanged** between:

* a local pseudo-socket, and  
* a remote `tcp://127.0.0.1:2375` (or `unix:///var/run/docker.sock` reached via SSH port-forward),

then **all** Docker sub-commands—build, exec, events, attach, etc.—will just work.

## High-level architecture

```
                 (local machine)
+-------------------+           +-------------------------------+
| any docker client |  |     proxy (this project)     |
+-------------------+           |  - listens on /tmp/docker.sock|
                                |  - dials remote over SSH      |
                                +-------------------------------+
                                                     ||
                                                     ||  SSH TCP forward
                                                     \/
                                    (remote host)
                                +--------------------+
                                |  dockerd -H 2375   |
                                +--------------------+
```

## Step-by-step implementation outline

### 1. Dependencies

```bash
go get github.com/docker/docker/client   # official SDK
go get golang.org/x/crypto/ssh           # SSH transport
go get github.com/creack/pty             # optional, for UNIX domain support
```

### 2. Parse user flags

```go
type cfg struct {
    localSock string // e.g. /tmp/docker.sock
    sshUser   string
    sshHost   string
    sshKey    string // path to private key
    remoteSock string // usually /var/run/docker.sock or tcp://127.0.0.1:2375
}
```

### 3. Establish SSH connection

```go
func sshDial(c cfg) (net.Conn, error) {
    key, _ := os.ReadFile(c.sshKey)
    signer, _ := ssh.ParsePrivateKey(key)
    clientConf := &ssh.ClientConfig{
        User: c.sshUser,
        Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(), // or verify!
        Timeout: 10 * time.Second,
    }
    sshConn, err := ssh.Dial("tcp", c.sshHost+":22", clientConf)
    if err != nil { return nil, err }

    // Forward a TCP stream to remote dockerd
    return sshConn.Dial("unix", c.remoteSock) // or ("tcp", "127.0.0.1:2375")
}
```

### 4. Listen on a local Unix socket

```go
l, err := net.Listen("unix", c.localSock)
defer os.Remove(c.localSock)
```

### 5. Bidirectional copy (the core proxy loop)

```go
for {
    localConn, _ := l.Accept()        // connection from docker CLI
    go func() {
        remoteConn, err := sshDial(c) // fresh SSH stream per client
        if err != nil { log.Println(err); localConn.Close(); return }

        go io.Copy(remoteConn, localConn) // client → remote
        io.Copy(localConn, remoteConn)    // remote → client
    }()
}
```

`io.Copy` moves raw bytes; no Docker-specific code is needed for 99% of traffic.

### 6. Optional: health-check with the SDK

Before exposing the socket you may want to verify the remote daemon is reachable:

```go
apiCli, _ := client.NewClientWithOpts(
    client.WithHost("http://dummy"), // not used
    client.WithAPIVersionNegotiation(),
    client.WithDialContext(func(ctx context.Context, _, _ string) (net.Conn, error) {
        return sshDial(c)            // SDK tunnels via our SSH Dialer
    }),
)
if _, err := apiCli.Ping(ctx); err != nil {
    log.Fatalf("Remote Docker unreachable: %v", err)
}
```

The SDK handles **API version negotiation** automatically[1].

### 7. Systemd service (optional)

```
[Unit]
Description=Docker SSH proxy
After=network.target

[Service]
ExecStart=/usr/local/bin/docker-ssh-proxy \
    --local-sock=/tmp/docker.sock \
    --ssh-user=user \
    --ssh-host=server.example.com \
    --ssh-key=/home/user/.ssh/id_ed25519