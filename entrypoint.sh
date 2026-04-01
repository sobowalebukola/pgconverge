#!/bin/bash
set -e

NODE_NAME=$1
NODES_JSON=/scripts/nodes.json

# --- Validate node name (prevent injection) ---
if ! [[ "$NODE_NAME" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]]; then
    echo "ERROR: Invalid node name '$NODE_NAME'. Must match [a-zA-Z_][a-zA-Z0-9_]*"
    exit 1
fi

# --- Extract Config (Once) ---
DB_NAME=$(grep -zoP "\"name\"\s*:\s*\"$NODE_NAME\"[\s\S]*?\"db\"\s*:\s*\"\K[^\"]+" "$NODES_JSON")
POSTGRES_USER=$(grep -zoP "\"name\"\s*:\s*\"$NODE_NAME\"[\s\S]*?\"user\"\s*:\s*\"\K[^\"]+" "$NODES_JSON")
POSTGRES_PASSWORD=$(grep -zoP "\"name\"\s*:\s*\"$NODE_NAME\"[\s\S]*?\"password\"\s*:\s*\"\K[^\"]+" "$NODES_JSON")

# Cleanup null bytes
DB_NAME=${DB_NAME%$'\0'}
POSTGRES_USER=${POSTGRES_USER%$'\0'}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD%$'\0'}

# Support env var password override (same convention as Go code)
ENV_KEY="PGCONVERGE_$(echo "$NODE_NAME" | tr '[:lower:]' '[:upper:]')_PASSWORD"
if [ -n "${!ENV_KEY}" ]; then
    POSTGRES_PASSWORD="${!ENV_KEY}"
fi

# --- Function to get host for a node ---
get_host_for_node() {
    local node=$1
    local host
    host=$(grep -zoP "\"name\"\s*:\s*\"$node\"[\s\S]*?\"host\"\s*:\s*\"\K[^\"]+" "$NODES_JSON")
    echo "${host%$'\0'}"
}

# Export password globally so all commands use it automatically
export PGPASSWORD="$POSTGRES_PASSWORD"

echo "Node: $NODE_NAME"
echo "Database: $DB_NAME"
echo "User: $POSTGRES_USER"

# --- Calculate dynamic replication limits based on cluster size ---
NODE_COUNT=$(grep -oP '"name"\s*:\s*"[^"]+"' "$NODES_JSON" | wc -l | tr -d ' ')
# Full mesh needs N*(N-1) slots, add buffer of 4
SLOT_COUNT=$(( NODE_COUNT * (NODE_COUNT - 1) + 4 ))
WAL_SENDERS=$(( SLOT_COUNT + 4 ))

echo "Cluster size: $NODE_COUNT nodes"
echo "Replication slots: $SLOT_COUNT, WAL senders: $WAL_SENDERS"

# =================================================================
# PHASE 1: BOOTSTRAP DATA (CLONE IF EMPTY)
# =================================================================

CLONED_SUCCESSFULLY=false

if [ -z "$(ls -A "$PGDATA" 2>/dev/null)" ]; then
    echo ">>> Data directory is empty. Looking for a donor node..."

    # Get list of all OTHER nodes
    other_nodes=$(grep -oP '"name"\s*:\s*"\K[^"]+' "$NODES_JSON" | grep -v "^${NODE_NAME}$")

    for donor in $other_nodes; do
        donor_host=$(get_host_for_node "$donor")
        echo "   Checking if '$donor' ($donor_host) is online..."

        # Check if donor is reachable
        if pg_isready -h "$donor_host" -U "$POSTGRES_USER" -d "$DB_NAME" -t 2 >/dev/null 2>&1; then
            echo " Found active donor: $donor ($donor_host). Starting Clone..."

            # --- EXECUTE CLONE ---
            # -X stream: Fetch WAL files for consistency
            # NO -R flag: We want a writeable DB
            pg_basebackup -h "$donor_host" -U "$POSTGRES_USER" -D "$PGDATA" -X stream -v

            # Fix permissions
            chown -R postgres:postgres "$PGDATA"
            chmod 700 "$PGDATA"

            CLONED_SUCCESSFULLY=true
            echo " Cloned successfully from $donor"
            break
        fi
    done

    if [ "$CLONED_SUCCESSFULLY" = false ]; then
        echo ">>> No active donors found. Initializing as SEED node."
    fi
else
    echo ">>> Data exists. Skipping clone."
fi

# =================================================================
# PHASE 2: START POSTGRES
# =================================================================

MAX_RETRIES=20
RETRY_DELAY=5

# Start Postgres in background with dynamic limits, WAL safety, and node identity
docker-entrypoint.sh postgres \
  -c wal_level=logical \
  -c "max_replication_slots=$SLOT_COUNT" \
  -c "max_wal_senders=$WAL_SENDERS" \
  -c max_slot_wal_keep_size=1GB \
  -c "pgconverge.node_name=$NODE_NAME" &

# Wait for readiness
until pg_isready -U "$POSTGRES_USER" -d "$DB_NAME"; do
  echo "Waiting for Postgres ($NODE_NAME)..."
  sleep 2
done

echo "Postgres ready on $NODE_NAME"

# --- Parse node names dynamically from nodes.json ---
nodes=$(grep -oP '"name"\s*:\s*"\K[^"]+' "$NODES_JSON")

echo ""
echo "Node Information:"
echo "   Current node: $NODE_NAME"
echo "   All nodes: $nodes"
echo ""

# =================================================================
# PHASE 3: LOGICAL REPLICATION SETUP
# =================================================================

# Set node identity at the database level so all sessions (including replication workers) inherit it
psql -U "$POSTGRES_USER" -d "$DB_NAME" -c "ALTER DATABASE \"$DB_NAME\" SET pgconverge.node_name = '$NODE_NAME';"

# Only run schema if we didn't clone (Cloning copies schema too)
if [ "$CLONED_SUCCESSFULLY" != "true" ]; then
    echo "Applying schema to seed node..."
    psql -U "$POSTGRES_USER" -d "$DB_NAME" -c "SET pgconverge.node_name = '$NODE_NAME'" -f /scripts/generated.sql || echo "Schema check skipped."
fi

# --- Create publication for THIS node ---
echo "Creating publication for $NODE_NAME..."
PUB_NAME="pub_${NODE_NAME}"
psql -U "$POSTGRES_USER" -d "$DB_NAME" -v pub_name="$PUB_NAME" <<'EOSQL'
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = :'pub_name') THEN
        EXECUTE format('CREATE PUBLICATION %I FOR TABLES IN SCHEMA public', :'pub_name');
    END IF;
END
$$;
EOSQL
echo "Created publication: $PUB_NAME"

# --- Wait a few seconds to ensure publication is ready ---
sleep 3

# --- Create subscriptions for all other nodes ---
for node in $nodes; do
  node=$(echo "$node" | xargs)

  # Validate each node name
  if ! [[ "$node" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]]; then
    echo "WARNING: Skipping node with invalid name: '$node'"
    continue
  fi

  if [ "$node" = "$NODE_NAME" ]; then
    echo "Skipping self-subscription to $node"
    continue
  fi

  node_host=$(get_host_for_node "$node")
  pub_name="pub_${node}"
  sub_name="sub_${NODE_NAME}_from_${node}"

  echo ""
  echo "========================================="
  echo "Setting up: $NODE_NAME subscribes to $node ($node_host)"
  echo "Subscription: $sub_name -> Publication: $pub_name"
  echo "========================================="

  # 1. Wait for Publisher
  retries=0
  while ! pg_isready -h "$node_host" -U "$POSTGRES_USER" -d "$DB_NAME" >/dev/null 2>&1; do
    retries=$((retries+1))
    if [ $retries -ge $MAX_RETRIES ]; then
      echo "ERROR: Publisher node $node ($node_host) not reachable after $MAX_RETRIES retries"
      continue 2
    fi
    echo "Waiting for publisher node $node ($node_host) to be ready ($retries/$MAX_RETRIES)..."
    sleep $RETRY_DELAY
  done

  echo "Publisher node $node ($node_host) is ready"

  # 2. Ensure Remote Slot Exists (Idempotent)
  echo "Ensuring replication slot '$sub_name' exists on publisher '$node' ($node_host)..."
  psql -h "$node_host" -U "$POSTGRES_USER" -d "$DB_NAME" -v slot_name="$sub_name" <<'EOSQL'
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = :'slot_name') THEN
        PERFORM pg_create_logical_replication_slot(:'slot_name', 'pgoutput');
    END IF;
END
$$;
EOSQL

  # 3. Create Local Subscription (Idempotent)
  echo "Configuring local subscription '$sub_name'..."
  CONN_STRING="host=$node_host dbname=$DB_NAME user=$POSTGRES_USER password=$POSTGRES_PASSWORD"
  psql -U "$POSTGRES_USER" -d "$DB_NAME" \
    -v sub_name="$sub_name" \
    -v conn_string="$CONN_STRING" \
    -v pub_name="$pub_name" <<'EOSQL'
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_subscription WHERE subname = :'sub_name') THEN
        EXECUTE format(
            'CREATE SUBSCRIPTION %I CONNECTION %L PUBLICATION %I WITH (create_slot = false, slot_name = %L, enabled = true, copy_data = false, origin = ''none'')',
            :'sub_name', :'conn_string', :'pub_name', :'sub_name'
        );
    ELSE
        RAISE NOTICE 'Subscription % already exists, skipping.', :'sub_name';
    END IF;
END
$$;
EOSQL

  # Verify subscription status
  sleep 1
  psql -U "$POSTGRES_USER" -d "$DB_NAME" -c \
    "SELECT subname, subenabled, subslotname FROM pg_subscription WHERE subname='$sub_name';"

done

echo ""
echo "========================================="
echo "All subscriptions configured on $NODE_NAME"
echo "========================================="

# Display final replication status
echo ""
echo "Subscriptions on $NODE_NAME (what we're receiving):"
psql -U "$POSTGRES_USER" -d "$DB_NAME" -c \
  "SELECT subname, subenabled, subslotname FROM pg_subscription ORDER BY subname;"

echo ""
echo "Replication slots on $NODE_NAME (other nodes subscribing to us):"
psql -U "$POSTGRES_USER" -d "$DB_NAME" -c \
  "SELECT slot_name, slot_type, active, temporary FROM pg_replication_slots ORDER BY slot_name;"

echo ""
echo "Publications on $NODE_NAME (what we're publishing):"
psql -U "$POSTGRES_USER" -d "$DB_NAME" -c \
  "SELECT pubname FROM pg_publication;"

wait
