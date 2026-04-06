# pgconverge

A Go CLI tool and library for setting up and managing **bidirectional PostgreSQL logical replication** across multiple nodes. Define your nodes and schema in JSON, and pgconverge generates the SQL, Docker infrastructure, and replication topology automatically.

## How It Works

pgconverge sets up a **full-mesh multi-master** replication topology. Every node publishes its changes to all other nodes, and subscribes to changes from all other nodes. This means a write on any node is replicated everywhere.

```
   node_a <-----> node_b
     ^  \         /  ^
     |   \       /   |
     |    v     v    |
     +--> node_c <---+
```

Under the hood, it uses PostgreSQL's built-in **logical replication** (publications and subscriptions). Conflicts are resolved using either a simple **last-write-wins** strategy (based on `updated_at` timestamps) or a **Hybrid Logical Clock (HLC)** for tables that opt into CRDT mode. Primary keys defined as `serial` are automatically converted to **UUIDs** to avoid cross-node collisions.

### What Gets Generated

From two JSON config files (`nodes.json` and `schema.json`), pgconverge generates:

- **`generated.sql`** -- DDL with tables, UUID primary keys, conflict resolution triggers, replica identity settings, and indexes
- **`docker-compose.yml`** -- Docker services for each node running PostgreSQL 16, with volumes, port mappings, and an entrypoint script that handles bootstrapping and replication setup

### Bootstrap Flow (Docker)

When Docker containers start, each node's entrypoint script:

1. **Clones data** from an existing node via `pg_basebackup` (if the data directory is empty and a donor is available), or initializes as a seed node
2. **Starts PostgreSQL** with `wal_level=logical` and dynamically calculated replication slot limits
3. **Applies the schema** (only if not cloned, since a clone already has it)
4. **Creates a publication** for all tables on the local node
5. **Creates subscriptions** to every other node, waiting for each to come online

## Installation

### CLI

```bash
go install github.com/sobowalebukola/pgconverge/cmd/pgconverge@latest
```

### Library

```bash
go get github.com/sobowalebukola/pgconverge
```

## Quick Start

### 1. Define Your Nodes

Create a `nodes.json` file describing your PostgreSQL instances:

```json
[
  {
    "name": "node_a",
    "host": "node_a",
    "db": "store",
    "user": "postgres",
    "password": "postgres"
  },
  {
    "name": "node_b",
    "host": "node_b",
    "db": "store",
    "user": "postgres",
    "password": "postgres"
  },
  {
    "name": "node_c",
    "host": "node_c",
    "db": "store",
    "user": "postgres",
    "password": "postgres"
  }
]
```

### 2. Define Your Schema

Create a `schema.json` file with your table definitions:

```json
{
  "users": {
    "name": "users",
    "columns": {
      "id": { "name": "id", "data_type": "serial" },
      "email": { "name": "email", "data_type": "varchar(150)" }
    },
    "constraints": {
      "primary": ["id"],
      "unique": [["email"]]
    },
    "indexes": [["email"]]
  },
  "orders": {
    "name": "orders",
    "columns": {
      "id": { "name": "id", "data_type": "serial" },
      "user_id": { "name": "user_id", "data_type": "int" },
      "amount": { "name": "amount", "data_type": "numeric(10,2)" },
      "created_at": { "name": "created_at", "data_type": "timestamp" }
    },
    "constraints": {
      "primary": ["id"],
      "foreign_keys": [
        {
          "columns": ["user_id"],
          "references": { "table": ["users"], "columns": ["id"] }
        }
      ]
    },
    "indexes": [["user_id"], ["created_at"]],
    "crdt": {
      "enabled": true
    }
  }
}
```

### 3. Generate Everything

```bash
pgconverge generate
```

This reads `schema.json` and `nodes.json` and produces `generated.sql` and `docker-compose.yml`.

### 4. Start the Cluster

```bash
docker compose up -d
```

Each node automatically bootstraps, clones data if a donor is available, applies the schema, and sets up replication.

### 5. Verify

```bash
pgconverge status
```

This shows connectivity, PostgreSQL version, WAL level, publications, subscriptions, and replication slots for every node. It also runs a health check that validates the full replication topology.

## CLI Reference

### `pgconverge generate`

Generates `generated.sql` from `schema.json` and `docker-compose.yml` from `nodes.json`.

```bash
pgconverge generate
pgconverge generate --nodes custom-nodes.json --schema custom-output.sql
```

### `pgconverge apply-schema`

Applies the generated SQL to nodes. Each application is wrapped in a transaction -- if it fails partway, the node is not left in a partial state.

```bash
# Apply to all nodes
pgconverge apply-schema

# Apply to a specific node
pgconverge apply-schema --node node_a

# With custom files
pgconverge apply-schema --nodes nodes.json --schema generated.sql
```

### `pgconverge setup-replication`

Creates publications and subscriptions for bidirectional replication. For N nodes, this creates N publications and N*(N-1) subscriptions (full mesh).

```bash
# All nodes
pgconverge setup-replication

# Add a single node to an existing cluster
pgconverge setup-replication --node node_d
```

### `pgconverge generate-hba`

Generates `pg_hba.conf` entries for each node, allowing all other nodes in the cluster to connect.

```bash
pgconverge generate-hba
pgconverge generate-hba --nodes prod-nodes.json
pgconverge generate-hba --auth-method md5   # for PostgreSQL < 14
```

Output:

```
# --- mumbai (34.100.5.12) — add to pg_hba.conf ---
# TYPE  DATABASE        USER            ADDRESS                 METHOD
host    store           postgres        18.200.3.44/32          scram-sha-256
host    store           postgres        35.177.8.90/32          scram-sha-256
```

Copy the relevant block to each server's `pg_hba.conf`, then reload: `SELECT pg_reload_conf();`

Docker containers don't need this -- the default `postgres:16` image already allows all connections.

| Flag | Default | Description |
|------|---------|-------------|
| `--auth-method` | `scram-sha-256` | Authentication method (`scram-sha-256`, `md5`) |

### `pgconverge status`

Displays the health and replication state of all configured nodes.

```bash
pgconverge status
pgconverge status --nodes nodes.json
```

### Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--nodes` | `-n` | `nodes.json` | Path to nodes configuration file |
| `--schema` | `-s` | `generated.sql` | Path to schema SQL file |

## Configuration

### nodes.json

Each entry defines a PostgreSQL node:

```json
{
  "name": "us_east",
  "host": "db1.example.com",
  "port": 5432,
  "db": "store",
  "user": "replicator",
  "password": "secret",
  "external": true
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | -- | Unique identifier. Must match `[a-zA-Z_][a-zA-Z0-9_]*` |
| `host` | Yes | -- | Hostname or IP address |
| `port` | No | `5432` | PostgreSQL port |
| `db` | Yes | -- | Database name |
| `user` | Yes | -- | PostgreSQL user |
| `password` | No | -- | Password (can be overridden via env var, see below) |
| `external` | No | `false` | If `true`, this node is not managed by Docker. It will be added as an `extra_hosts` entry for DNS resolution but won't get a Docker service |

### schema.json

A map of table name to table definition:

```json
{
  "table_name": {
    "name": "table_name",
    "columns": {
      "col_name": { "name": "col_name", "data_type": "varchar(100)", "default": "'unknown'" }
    },
    "constraints": {
      "primary": ["id"],
      "unique": [["email"], ["username", "tenant_id"]],
      "foreign_keys": [
        {
          "columns": ["user_id"],
          "references": { "table": ["users"], "columns": ["id"] }
        }
      ]
    },
    "indexes": [["col_a"], ["col_b", "col_c"]],
    "crdt": {
      "enabled": true,
      "columns": {
        "col_name": { "type": "lww_field" }
      }
    }
  }
}
```

**Automatic transformations applied during SQL generation:**

- `serial` primary keys are converted to `uuid` with `gen_random_uuid()` default (prevents cross-node ID collisions)
- An `updated_at TIMESTAMP DEFAULT now()` column is added to every table
- An `origin_node VARCHAR(50)` column is added to every table (tracks which node originated the row)
- `REPLICA IDENTITY FULL` is set on every table (required for logical replication)
- A conflict resolution trigger is created for each table (last-write-wins based on `updated_at`)

**Additional transformations for CRDT-enabled tables** (tables with `"crdt": {"enabled": true}`):

- `_hlc_ts BIGINT`, `_hlc_counter INTEGER`, and `_hlc_node VARCHAR(50)` columns are added for HLC tracking
- A shared `_pgconverge` schema is created with the HLC state table and `advance_hlc()` function
- Conflict resolution uses the HLC tuple `(ts, counter, node)` instead of `updated_at` for deterministic total ordering
- An HLC stamping trigger (`a_stamp_hlc`) sets the clock values on local writes; a conflict resolution trigger (`z_resolve_conflict`) compares HLC tuples on all writes including replicated ones. Triggers are alphabetically prefixed to ensure correct execution order

### Password Management

Passwords can be provided in three ways (in order of precedence):

1. **Environment variable**: `PGCONVERGE_<NODENAME>_PASSWORD` (node name uppercased)
2. **JSON config**: the `password` field in `nodes.json`

```bash
# Instead of putting passwords in nodes.json:
export PGCONVERGE_NODE_A_PASSWORD=secure_password
export PGCONVERGE_NODE_B_PASSWORD=another_password
pgconverge status
```

This avoids storing credentials in plaintext configuration files. The same convention is supported in the Docker entrypoint script.

## Library Usage

### Connection Management

```go
import (
    "context"
    "github.com/sobowalebukola/pgconverge/db"
    "github.com/sobowalebukola/pgconverge/schema"
)

nodes := []schema.Node{
    {
        Name:     "primary",
        Host:     "localhost",
        Port:     5433,
        User:     "postgres",
        Database: "mydb",
        Password: "postgres",
    },
}

manager := db.NewDBManager(nodes)
defer manager.Close()

ctx := context.Background()

// Check connectivity
err := manager.Ping(ctx, "primary")

// Get detailed status
statuses := manager.GetAllNodeStatuses(ctx)

// Check replication health
healthy, issues := manager.CheckReplicationHealth(ctx)
```

### Schema Application

```go
// Apply SQL string (wrapped in a transaction)
err := manager.ApplySchema(ctx, &nodes[0], "CREATE TABLE users (...)")

// Apply from file to all nodes
errors := manager.ApplySchemaFromFileToAll(ctx, "generated.sql")
```

### Replication Setup

```go
// Full mesh between all nodes
results := manager.SetupBidirectionalReplication(ctx)

// Add a single node to an existing cluster
results := manager.SetupReplicationForNode(ctx, "node_d")
```

### SQL and Docker Compose Generation

```go
import (
    "github.com/sobowalebukola/pgconverge/sqlgen"
    "github.com/sobowalebukola/pgconverge/compose"
)

// Generate SQL from file
sqlgen.Generate("schema.json", "output.sql")

// Generate programmatically
tables := map[string]schema.Table{...}
sql := sqlgen.GenerateSQL(tables)

// Generate Docker Compose from file
compose.Generate("nodes.json", "docker-compose.yml")

// Generate programmatically
composeMap := compose.GenerateComposeMap(nodes)
```

## Architecture

### Replication Topology

pgconverge creates a **full-mesh** topology. For N nodes:

- **N publications** (one per node, named `pub_<node_name>`)
- **N*(N-1) subscriptions** (each node subscribes to every other, named `sub_<subscriber>_from_<publisher>`)
- **N*(N-1) replication slots** (one per subscription, same name as the subscription)

Example for 3 nodes:

```
node_a publishes  -> node_b subscribes (sub_node_b_from_node_a)
node_a publishes  -> node_c subscribes (sub_node_c_from_node_a)
node_b publishes  -> node_a subscribes (sub_node_a_from_node_b)
node_b publishes  -> node_c subscribes (sub_node_c_from_node_b)
node_c publishes  -> node_a subscribes (sub_node_a_from_node_c)
node_c publishes  -> node_b subscribes (sub_node_b_from_node_c)
```

### Conflict Resolution

pgconverge supports two conflict resolution strategies, configurable per table:

#### Timestamp-based Last-Write-Wins (default)

For tables **without** CRDT enabled:

- Every table has an `updated_at` column set to `NOW()` on each write
- A `BEFORE UPDATE` trigger compares the incoming `updated_at` with the stored value
- If the incoming timestamp is newer, the update is applied; otherwise it is discarded

This means the node whose write happened later (by wall-clock time) wins. For this to work reliably, **node clocks should be synchronized** (e.g., via NTP).

#### Hybrid Logical Clock (HLC) -- CRDT mode

For tables with `"crdt": {"enabled": true}`:

HLC combines physical wall-clock time with a logical counter to provide **causally consistent, deterministic ordering** without requiring perfectly synchronized clocks. Each write is stamped with an HLC tuple `(timestamp, counter, node_name)`:

1. **Local writes**: the `a_stamp_hlc` trigger calls `_pgconverge.advance_hlc()` to advance the clock and stamps the row with the new HLC values. This trigger runs only for local writes (skipped by replication apply workers).
2. **Replicated writes**: arrive with the origin node's HLC values already set. The `z_resolve_conflict` trigger (set to `ENABLE ALWAYS`) fires on all writes including replicated ones, and advances the local HLC to track the incoming timestamp for causal ordering.
3. **Conflict resolution**: the HLC tuple `(ts, counter, node_name)` is compared lexicographically. The write with the higher tuple wins; the lower one is discarded.

Trigger names are alphabetically prefixed (`a_`, `z_`) because PostgreSQL fires `BEFORE` triggers in alphabetical order -- stamping must happen before conflict resolution.

This gives a **total ordering** across all nodes. Since the node name acts as a tiebreaker, concurrent writes with identical physical timestamps are resolved deterministically rather than arbitrarily.

**Node identity** is propagated via the `pgconverge.node_name` PostgreSQL GUC, set at the database level so all sessions (including replication apply workers) inherit it.

### Data Bootstrapping

New nodes joining a cluster get their initial data in one of two ways:

- **Docker mode (entrypoint.sh)**: uses `pg_basebackup` to clone from the first available donor node. Subscriptions are created with `copy_data = false` since the clone already has all data
- **CLI mode (setup-replication)**: subscriptions are created with `copy_data = true`, so PostgreSQL copies existing data from each publisher during initial sync

## Deploying to External Nodes (Bastion Host)

For production deployments with PostgreSQL instances running on separate servers (EC2, GCP, bare metal), you run pgconverge from a **bastion host** -- a small management machine that has network access to all database nodes.

```
  Bastion host (runs pgconverge)
      │
      ├── pgconverge apply-schema
      ├── pgconverge setup-replication
      ├── pgconverge status
      │
      ├──► mumbai     (34.100.5.12:5432)
      ├──► frankfurt  (18.200.3.44:5432)
      └──► london     (35.177.8.90:5432)
```

### Step-by-step

**1. Prepare each PostgreSQL node**

Each node needs `wal_level = logical` in `postgresql.conf` and must allow connections from the other nodes in `pg_hba.conf`. Generate the entries:

```bash
pgconverge generate-hba -n nodes.json
```

Apply the output to each node's `pg_hba.conf`, then reload: `SELECT pg_reload_conf();`

**2. Set node identity on each node**

```bash
# Run on each node (or via psql from the bastion)
ALTER DATABASE store SET pgconverge.node_name = 'mumbai';
```

This ensures replication workers inherit the node name for conflict resolution tiebreaking.

**3. Apply schema and set up replication from the bastion**

```bash
pgconverge apply-schema -n nodes.json
pgconverge setup-replication -n nodes.json
pgconverge status -n nodes.json
```

The `host` in `nodes.json` must be an address that the bastion **and** the nodes can reach each other on. Use private VPC IPs if all nodes are in the same VPC, or public IPs if they're across regions.

### Important: inter-node connectivity

pgconverge connects to each node from the bastion to run SQL. But when it creates a subscription, the `CONNECTION` string is embedded in PostgreSQL -- the subscriber node itself connects to the publisher. This means the `host` value in `nodes.json` must be routable **between nodes**, not just from the bastion.

```
nodes.json host = 34.100.5.12
                │
                ├── Bastion → 34.100.5.12  ✓ (CLI connects)
                └── London  → 34.100.5.12  ✓ (subscription connects)
```

If your bastion uses a different address to reach the nodes than the nodes use to reach each other (e.g., bastion goes through a load balancer), the subscription will fail. Use direct IPs.

## Requirements

### For Docker-Managed Nodes

- Docker and Docker Compose
- The `entrypoint.sh` script in the project root (mounted automatically via the generated `docker-compose.yml`)

### For External Nodes

External PostgreSQL instances must have:

- **`wal_level = logical`** in `postgresql.conf`
- **Sufficient replication slots**: at least `N*(N-1)` where N is the total number of nodes
- **Network connectivity** between all nodes
- **pg_hba.conf** entries allowing replication connections from all other nodes

### For the CLI

- Go 1.21+ (for building from source)
- Network access to all configured PostgreSQL nodes

## Project Structure

```
pgconverge/
  cmd/pgconverge/     CLI entry point
  cli/                Cobra command definitions
  db/                 Connection pooling, schema application, replication, status
  schema/             Core types (Node, Table, Column, Constraints)
  sqlgen/             SQL DDL generation from JSON schema
  compose/            Docker Compose YAML generation
  util/               Helpers (string utils, port allocation)
  entrypoint.sh       Docker container bootstrap script
  nodes.json          Node configuration (user-provided)
  schema.json         Table definitions (user-provided)
```

## License

MIT
