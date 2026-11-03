# High-Performance Reverse Proxy & Load Balancer in Go

A lightweight, concurrent HTTP reverse proxy and load balancer built from scratch in Go using standard library primitives (`net/http`, `httputil.ReverseProxy`, and `sync/atomic`).

Designed for high resilience, featuring **zero-downtime failover**, **transparent retries**, and **background health checks**.

---

## 🚀 Key Features

- **Round-Robin Load Balancing:** Evenly distributes incoming HTTP traffic across healthy backend pools.
- **Failover with Transparent Retries:** If a backend server drops connection or becomes unavailable, the proxy disables the node and instantly retries the request against another healthy backend without returning an error to the client.
- **Active Background Health Checks:** Periodically pings (`time.Ticker`) backend servers in an asynchronous Goroutine to automatically reactivate nodes once they recover.
- **Thread-Safe & Lock-Free State:** Employs `sync/atomic.Bool` for backend active states, eliminating lock contention on hot read paths.
- **Graceful Shutdown:** Respects OS interruption signals (`SIGINT`, `SIGTERM`) with a configurable graceful shutdown context to drain in-flight requests.
- **Zero Third-Party Dependencies:** 100% built with Go's standard library.

---

## 🏗️ Architecture

```
                                  +-----------------------+
                                  |   Client Requests     |
                                  +-----------+-----------+
                                              |
                                              v
                              +-------------------------------+
                              |    Reverse Proxy (:8000)      |
                              |  - Round-Robin Scheduler      |
                              |  - Active Health Checker      |
                              +---------------+---------------+
                                              |
                     +------------------------+------------------------+
                     |                        |                        |
                     v                        v                        v
           +-------------------+    +-------------------+    +-------------------+
           | Backend Server 1  |    | Backend Server 2  |    | Backend Server 3  |
           |     (:8001)       |    |     (:8002)       |    |     (:8003)       |
           +-------------------+    +-------------------+    +-------------------+
```

---

## 📦 Getting Started

### 1. Prerequisites
- [Go 1.22+](https://go.dev/dl/) installed.

### 2. Run Mock Backend Servers
To quickly test the load balancer, start the bundled mock servers (runs 3 backends on ports 8001, 8002, and 8003):
```bash
go run examples/mock_servers.go
```

### 3. Start the Reverse Proxy
In another terminal, start the reverse proxy:
```bash
go run main.go
```
The reverse proxy listens on `http://localhost:8000`.

### 4. Send Requests
Send requests to the proxy:
```bash
curl http://localhost:8000
```
You will see responses distributed sequentially:
```json
{"message": "Alive the server-1", "port": 8001}
{"message": "Alive the server-2", "port": 8002}
{"message": "Alive the server-3", "port": 8003}
```

---

## 🧪 Running the Automated Test Suite

The project includes an end-to-end unit and concurrency test suite testing:
- Round-robin distribution
- Automatic failover and client retry
- Health check recovery of revived nodes
- Graceful `503 Service Unavailable` handling
- Race-condition safety under concurrent load

Run all tests:
```bash
go test -v ./proxy/...
```

---

## 📂 Project Structure

```
reverse-proxy/
├── .gitignore             # Git ignore rules for Go and IDE files
├── go.mod                 # Go module definition
├── main.go                # Application entrypoint & HTTP server configuration
├── proxy/
│   ├── proxy.go           # Core Balancer, Proxy, and HealthCheck logic
│   └── proxy_test.go      # Comprehensive unit, failover, and concurrency tests
├── examples/
│   └── mock_servers.go    # Lightweight mock backend server cluster for testing
└── README.md              # Project documentation
```

---

## 📄 License
MIT
