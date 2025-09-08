# Distributed Key-Value Store

A distributed key-value store built with Go, using HashiCorp Raft for consensus and gRPC for communication. This implementation provides strong consistency guarantees through the Raft consensus algorithm while offering a simple key-value API.

## Features

- **Distributed Architecture**: Multi-node cluster support with automatic leader election
- **Strong Consistency**: Uses Raft consensus algorithm to ensure data consistency across nodes
- **gRPC API**: High-performance RPC interface for client operations
- **Automatic Failover**: Leader election and failover handling
- **Persistent Storage**: Data persistence using BoltDB
- **Dynamic Cluster Management**: Add new nodes to existing clusters

## Architecture

The system consists of:
- **gRPC Server**: Handles client requests (GET, SET, DELETE)
- **Raft Node**: Manages consensus, replication, and leader election
- **FSM Store**: In-memory key-value store that applies Raft log entries
- **Persistent Storage**: BoltDB for Raft logs and snapshots

## Prerequisites

- Go 1.24.6 or later
- Protocol Buffers compiler (`protoc`)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/AndreiAlbert/kv.git
cd kv-store
```

2. Install dependencies:
```bash
make deps
```

3. Build the project:
```bash
make build
```

## Usage

### Starting a Cluster

#### Bootstrap the first node:
```bash
./bin/server -id=node1 -grpcport=50051 -raftport=60051 -bootstrap
```

#### Add additional nodes:
```bash
# Node 2
./bin/server -id=node2 -grpcport=50052 -raftport=60052 -join=localhost:50051

# Node 3  
./bin/server -id=node3 -grpcport=50053 -raftport=60053 -join=localhost:50051
```

### Client Operations

#### Set a key-value pair:
```bash
./bin/client set mykey myvalue
./bin/client -server=localhost:50052 set anotherkey anothervalue
```

#### Get a value:
```bash
./bin/client get mykey
./bin/client -server=localhost:50053 get mykey
```

#### Delete a key:
```bash
./bin/client delete mykey
```

## Configuration Options

### Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-id` | `node1` | Unique identifier for the node |
| `-grpcport` | `50051` | Port for gRPC server |
| `-raftport` | `60051` | Port for Raft communication |
| `-bootstrap` | `false` | Bootstrap a new cluster (first node only) |
| `-join` | `""` | Address of existing node to join |

### Client Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | `localhost:50051` | Address of server to connect to |

## API Reference

### gRPC Service Definition

```protobuf
service KeyValueStore {
  rpc Set(SetRequest) returns (SetResponse) {}
  rpc Get(GetRequest) returns (GetResponse) {}
  rpc Delete(DeleteRequest) returns (DeleteResponse) {}
  rpc Join(JoinRequest) returns (JoinResponse) {}
}
```

### Operations

- **SET**: Store a key-value pair (writes go to leader, redirected if necessary)
- **GET**: Retrieve value for a key (can be served by any node)
- **DELETE**: Remove a key (writes go to leader, redirected if necessary)
- **JOIN**: Add a new node to the cluster (leader only)

## Development

### Project Structure

```
kv-store/
├── cmd/
│   └── server/           # Server entry point
├── client/               # Client implementation
├── internal/
│   ├── config/          # Configuration management
│   ├── node/            # Raft node and gRPC handlers
│   └── server/          # Server orchestration
├── proto/               # Protocol buffer definitions
├── store/               # FSM implementation
├── Makefile             # Build automation
└── go.mod              # Go module definition
```

### Building from Source

```bash
# Install protocol buffer tools
make deps

# Generate protocol buffer code
make proto

# Build server and client binaries
make build

# Clean build artifacts
make clean
```
