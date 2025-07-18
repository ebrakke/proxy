# TCP Proxy

A simple TCP proxy tool in Go that supports both forward and reverse proxy modes, with automatic configuration file support.

## Usage

### Auto Mode (Recommended)
Use a `.proxy.conf` file to automatically set up multiple proxies:

**Auto Forward Mode** (run on client):
```bash
proxy
```

**Auto Reverse Mode** (run on server):
```bash
proxy -r
```

### Manual Mode
**Forward Mode** - Forward localhost connections to remote servers:
```bash
proxy [remote]:[port] [localPort]
```

**Reverse Mode** - Expose localhost services on all network interfaces:
```bash
proxy -r [localPort] [externalPort]
```

## Configuration File

Create a `.proxy.conf` file in your project directory:

```
# Proxy Configuration
# Format: port:description (optional)
# Lines starting with # are comments

# Web development
3000:React dev server
8080:API server
8782:Backend service

# Database connections
5432:PostgreSQL
6379:Redis

# Other services
9000:Grafana
8000:Django
```

## Examples

### Using Config File (Recommended Workflow)

1. **On your server (work-mbp):**
   ```bash
   # Create config file in your project
   echo "8782:Backend API" > .proxy.conf
   
   # Start reverse proxy (exposes localhost:8782 on Tailscale)
   ./proxy -r
   ```

2. **On your client machine:**
   ```bash
   # Set remote host environment variable
   export PROXY_REMOTE_HOST=work-mbp.tailnet.ts.net
   
   # Start forward proxy (connects localhost:8782 to remote)
   ./proxy
   ```

3. **Now you can access your remote service locally:**
   ```bash
   curl localhost:8782  # Actually connects to work-mbp:8782
   ```

### Manual Mode Examples

**Forward Mode:**
```bash
./proxy myserver.tailnet.ts.net:8080 3000
```

**Reverse Mode:**
```bash
./proxy -r 8080 8080
```

## Features

- **Forward mode**: Connect localhost to remote services
- **Reverse mode**: Expose localhost services to the network
- Single Go file with no external dependencies
- Handles multiple concurrent connections
- Basic error handling and connection logging
- Tests connectivity before starting proxy
- Bidirectional data forwarding using goroutines

## How it works

### Forward Mode
1. Tests connectivity to the remote server
2. Starts a TCP listener on localhost:[localPort]
3. For each incoming connection, creates a connection to the remote server
4. Uses `io.Copy` in goroutines for bidirectional data flow

### Reverse Mode
1. Tests connectivity to the local service
2. Starts a TCP listener on 0.0.0.0:[externalPort]
3. For each incoming connection, creates a connection to localhost:[localPort]
4. Uses `io.Copy` in goroutines for bidirectional data flow
5. Logs connection events for debugging