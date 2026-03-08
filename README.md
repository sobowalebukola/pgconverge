# pgconverge

A Go library and CLI tool for setting up and managing bidirectional PostgreSQL logical replication across multiple nodes.

## Installation

### CLI Tool

```bash
go install github.com/sobowalebukola/pgconverge/cmd/pgconverge@latest
```

### Library

```bash
go get github.com/sobowalebukola/pgconverge
```

## CLI Usage

### Generate Configuration Files

Generate SQL schema and docker-compose.yml from your configuration:

```bash
pgconverge generate
```

This reads `schema.json` and `nodes.json` to produce:
- `generated.sql` - DDL with tables, triggers, and conflict resolution
- `docker-compose.yml` - Docker configuration for PostgreSQL nodes

### Check Node Status

```bash
pgconverge status --nodes nodes.json
```

Shows connectivity, WAL level, publications, subscriptions, and replication slots for all nodes.

### Apply Schema

Apply schema to all nodes:
```bash
pgconverge apply-schema
```

Apply to a specific node:
```bash
pgconverge apply-schema --node us_east
```

### Setup Replication

Set up bidirectional replication between all nodes:
```bash
pgconverge setup-replication
```

Set up for a specific node:
```bash
pgconverge setup-replication --node us_east
```

## Library Usage

### Database Connection Management

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
        Port:     5432,
        User:     "postgres",
        Password: "postgres",
        Database: "mydb",
    },
}

manager := db.NewDBManager(nodes)
defer manager.Close()

// Check status
ctx := context.Background()
statuses := manager.GetAllNodeStatuses(ctx)

// Apply schema
manager.ApplySchema(ctx, &nodes[0], "CREATE TABLE users (...)")

// Setup replication
results := manager.SetupBidirectionalReplication(ctx)
```

### Generate SQL Schema

```go
import "github.com/sobowalebukola/pgconverge/sqlgen"

// From file
sqlgen.Generate("schema.json", "output.sql")

// Programmatically
tables := map[string]schema.Table{...}
sql := sqlgen.GenerateSQL(tables)
```

### Generate Docker Compose

```go
import "github.com/sobowalebukola/pgconverge/compose"

// From file
compose.Generate("nodes.json", "docker-compose.yml")

// Programmatically
nodes := []schema.Node{...}
composeMap := compose.GenerateComposeMap(nodes)
```

## Configuration Files

### nodes.json

```json
[
  {
    "name": "us_east",
    "host": "db1.example.com",
    "port": 5432,
    "db": "store",
    "user": "postgres",
    "password": "secret",
    "external": true
  },
  {
    "name": "local_dev",
    "host": "node_a",
    "db": "store",
    "user": "postgres",
    "password": "postgres"
  }
]
```

| Field | Description |
|-------|-------------|
| `name` | Unique node identifier |
| `host` | Hostname or IP address |
| `port` | PostgreSQL port (default: 5432) |
| `db` | Database name |
| `user` | PostgreSQL user |
| `password` | PostgreSQL password |
| `external` | If true, node is not managed by Docker |

### schema.json

```json
{
  "users": {
    "name": "users",
    "columns": {
      "id": {"name": "id", "data_type": "serial"},
      "email": {"name": "email", "data_type": "VARCHAR(150)"}
    },
    "constraints": {
      "primary": ["id"],
      "unique": [["email"]],
      "foreign_keys": []
    },
    "indexes": [["email"]]
  }
}
```

## Requirements

External PostgreSQL hosts must have:
- `wal_level = logical` in postgresql.conf
- Network connectivity between all nodes
- Appropriate pg_hba.conf entries for replication

## License

MIT
