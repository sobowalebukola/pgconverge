#!/bin/bash
set -e

NODE_NAME=$1
NODES_JSON=/scripts/nodes.json

# --- Extract Config ---
DB_NAME=$(grep -zoP "\"name\"\s*:\s*\"$NODE_NAME\"[\s\S]*?\"db\"\s*:\s*\"\K[^\"]+" "$NODES_JSON")
POSTGRES_USER=$(grep -zoP "\"name\"\s*:\s*\"$NODE_NAME\"[\s\S]*?\"user\"\s*:\s*\"\K[^\"]+" "$NODES_JSON")
POSTGRES_PASSWORD=$(grep -zoP "\"name\"\s*:\s*\"$NODE_NAME\"[\s\S]*?\"password\"\s*:\s*\"\K[^\"]+" "$NODES_JSON")

DB_NAME=${DB_NAME%$'\0'}
POSTGRES_USER=${POSTGRES_USER%$'\0'}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD%$'\0'}

echo "Node: $NODE_NAME"
echo "Database: $DB_NAME"
echo "User: $POSTGRES_USER"

MAX_RETRIES=20
RETRY_DELAY=5

# --- Start Postgres with logical replication ---
docker-entrypoint.sh postgres \
  -c wal_level=logical \
  -c max_replication_slots=10 \
  -c max_wal_senders=10 &

# --- Wait until Postgres is ready ---
until pg_isready -U "$POSTGRES_USER" -d "$DB_NAME"; do
  echo "Waiting for Postgres ($NODE_NAME)..."
  sleep 2
done

echo "Postgres ready on $NODE_NAME"

# --- Parse node names dynamically from nodes.json ---
nodes=$(grep -oP '"name"\s*:\s*"\K[^"]+' "$NODES_JSON")

echo ""
echo "📋 Node Information:"
echo "   Current node: $NODE_NAME"
echo "   All nodes: $nodes"
echo ""

# --- Apply schema ---
psql -U "$POSTGRES_USER" -d "$DB_NAME" -f /scripts/generated.sql

# --- Create publication for THIS node only ---
echo "Creating publication for $NODE_NAME..."
PUB_NAME="pub_$NODE_NAME"
psql -U "$POSTGRES_USER" -d "$DB_NAME" <<EOF
-- Drop and recreate to make idempotent
DROP PUBLICATION IF EXISTS $PUB_NAME;
CREATE PUBLICATION $PUB_NAME FOR ALL TABLES;
EOF
echo "✓ Created publication: $PUB_NAME"

# --- Wait a few seconds to ensure publication is ready ---
sleep 3

# --- Create subscriptions for all other nodes ---
for node in $nodes; do
  # Trim any whitespace from node name
  node=$(echo "$node" | xargs)
  
  if [ "$node" = "$NODE_NAME" ]; then
    echo "⏭️  Skipping self-subscription to $node"
    continue
  fi

  pub_name="pub_$node"
  sub_name="sub_${NODE_NAME}_from_${node}"

  echo ""
  echo "========================================="
  echo "Setting up: $NODE_NAME subscribes to $node"
  echo "Subscription: $sub_name -> Publication: $pub_name"
  echo "========================================="

  # 1. Wait for the remote publisher node to be ready
  retries=0
  while ! pg_isready -h "$node" -U "$POSTGRES_USER" -d "$DB_NAME" >/dev/null 2>&1; do
    retries=$((retries+1))
    if [ $retries -ge $MAX_RETRIES ]; then
      echo "✗ ERROR: Publisher node $node not reachable after $MAX_RETRIES retries"
      exit 1
    fi
    echo "Waiting for publisher node $node to be ready ($retries/$MAX_RETRIES)..."
    sleep $RETRY_DELAY
  done
  
  echo "✓ Publisher node $node is ready"

  # 2. Ensure Replication Slot exists on REMOTE node (Idempotent)
  # We connect to the REMOTE node (-h $node) and create the slot if missing.
  echo "Ensuring replication slot '$sub_name' exists on publisher '$node'..."
  export PGPASSWORD=$POSTGRES_PASSWORD
  psql -h "$node" -U "$POSTGRES_USER" -d "$DB_NAME" -c "
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = '$sub_name') THEN
        PERFORM pg_create_logical_replication_slot('$sub_name', 'pgoutput');
    END IF;
END
\$\$;"

  # 3. Create Subscription LOCALLY (Idempotent)
  # We use create_slot=false because we handled it in step 2.
  echo "Configuring local subscription '$sub_name'..."
  
  # We use a simple IF NOT EXISTS check via SELECT before creating
  psql -U "$POSTGRES_USER" -d "$DB_NAME" <<EOF
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_subscription WHERE subname = '$sub_name') THEN
        CREATE SUBSCRIPTION $sub_name 
        CONNECTION 'host=$node dbname=$DB_NAME user=$POSTGRES_USER password=$POSTGRES_PASSWORD' 
        PUBLICATION $pub_name
        WITH (create_slot = false, slot_name = '$sub_name', enabled = true, copy_data = false, origin = 'none');
    ELSE
        RAISE NOTICE 'Subscription % already exists, skipping.', '$sub_name';
    END IF;
END
\$\$;
EOF

  # Verify subscription status
  sleep 1
  psql -U "$POSTGRES_USER" -d "$DB_NAME" -c \
    "SELECT subname, subenabled, subslotname FROM pg_subscription WHERE subname='$sub_name';"
  
done

echo ""
echo "========================================="
echo "✓ All subscriptions configured on $NODE_NAME"
echo "========================================="

# Display final replication status
echo ""
echo "📊 Subscriptions on $NODE_NAME (what we're receiving):"
psql -U "$POSTGRES_USER" -d "$DB_NAME" -c \
  "SELECT subname, subenabled, subslotname FROM pg_subscription ORDER BY subname;"

echo ""
echo "📊 Replication slots on $NODE_NAME (other nodes subscribing to us):"
psql -U "$POSTGRES_USER" -d "$DB_NAME" -c \
  "SELECT slot_name, slot_type, active, temporary FROM pg_replication_slots ORDER BY slot_name;"

echo ""
echo "📊 Publications on $NODE_NAME (what we're publishing):"
psql -U "$POSTGRES_USER" -d "$DB_NAME" -c \
  "SELECT pubname FROM pg_publication;"

wait


# docker exec node_a psql -U postgres -d mydb -c "INSERT INTO users (email) VALUES ('test@example.com');"
# docker exec $node psql -U postgres -d mydb -c "SELECT * FROM users;"

# psql -U postgres -d store -c "INSERT INTO users (email) VALUES ('iku@gmail.com')"
# psql -U postgres -d store -c "SELECT * FROM users";
#psql -U postgres -d store -c "UPDATE users set email = 'killer@gmail.com' where email='iku@gmail.com';"

#psql -U postgres -d store -c "DELETE FROM users where email = 'iku@gmail.com';"

# psql -U postgres -d store -c "CREATE TABLE IF NOT EXISTS "users" (
#   "email" varchar(150),
#   "id" uuid DEFAULT gen_random_uuid(),
#   "updated_at" TIMESTAMP DEFAULT now(),
#   "origin_node" VARCHAR(50) NOT NULL DEFAULT,
#   PRIMARY KEY ("id"),
#   UNIQUE ("email")
# );"

#

#psql -U postgres -d store -c "INSERT INTO users (email, origin_node) VALUES ('iku@gmail.com', 'node_a')"
psql -U postgres -d store -c "UPDATE users SET email = 'killer@gmail.com', origin_node = 'node_c', updated_at = 'NOW() ' WHERE email = 'iku@gmail.com'"
psql -U postgres -d store -c "SELECT * FROM users where email = 'iku@gmail.com'";
psql -U postgres -d store -c "DELETE FROM users where email = 'iku@gmail.com';"